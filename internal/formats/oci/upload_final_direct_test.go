package oci_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type finalStageStore struct {
	*appendCountingStore
	pending                      bool
	abortErr, putErr, publishErr error
}

func (s *finalStageStore) AppendBlob(ctx context.Context, key string, r io.Reader) (int64, error) {
	s.pending = true
	return s.appendCountingStore.AppendBlob(ctx, key, r)
}
func (s *finalStageStore) AbortAppend(ctx context.Context, key string) error {
	if s.abortErr != nil {
		return s.abortErr
	}
	s.pending = false
	return s.appendCountingStore.AbortAppend(ctx, key)
}
func (s *finalStageStore) Put(ctx context.Context, key string, r io.Reader, n int64) error {
	if strings.HasPrefix(key, "_uploads/") {
		if s.putErr != nil {
			return s.putErr
		}
	} else if s.publishErr != nil {
		return s.publishErr
	}
	return s.appendCountingStore.Put(ctx, key, r, n)
}
func (s *finalStageStore) FinalizeAppend(ctx context.Context, key string) error {
	if s.pending {
		return errors.New("stale append state would overwrite direct staging")
	}
	return s.appendCountingStore.FinalizeAppend(ctx, key)
}

func TestMonolithicUploadBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bytes    int
		declared int64
		status   int
	}{
		{"below", 1023, 1023, 201}, {"at", 1024, 1024, 201}, {"above", 1025, 1025, 413},
		{"unknown-below", 1023, -1, 201}, {"unknown-at", 1024, -1, 201}, {"unknown-above", 1025, -1, 413},
		{"short", 1023, 1024, 500}, {"long", 1024, 1023, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &finalStageStore{appendCountingStore: newAppendCountingStore()}
			r := setupWithStore(store, 1024)
			loc := startUpload(t, r)
			body := bytes.Repeat([]byte{17}, tc.bytes)
			digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
			req := httptest.NewRequest(http.MethodPut, loc+"?digest="+digest, bytes.NewReader(body))
			req.ContentLength = tc.declared
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == 201 {
				read := httptest.NewRecorder()
				r.ServeHTTP(read, httptest.NewRequest(http.MethodGet, w.Header().Get("Location"), nil))
				require.Equal(t, 200, read.Code)
				require.Equal(t, body, read.Body.Bytes())
				require.Zero(t, store.appends)
				require.Zero(t, store.finalizes)
			} else {
				n, err := store.Size(context.Background(), "_uploads/docker/"+loc[strings.LastIndex(loc, "/")+1:])
				require.NoError(t, err)
				require.Zero(t, n)
			}
		})
	}
}

func TestMonolithicUploadClearsEmptyPatchAndRetainsRetry(t *testing.T) {
	store := &finalStageStore{appendCountingStore: newAppendCountingStore()}
	r := setupWithStore(store, 1024)
	loc := startUpload(t, r)
	patch := httptest.NewRecorder()
	r.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, loc, http.NoBody))
	require.Equal(t, 202, patch.Code)
	require.False(t, store.pending)
	// Simulate empty multipart state left by an earlier registry version.
	_, err := store.AppendBlob(context.Background(), "_uploads/docker/"+loc[strings.LastIndex(loc, "/")+1:], http.NoBody)
	require.NoError(t, err)
	require.True(t, store.pending)
	body := []byte("config-json")
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256(body))
	wrong := httptest.NewRecorder()
	r.ServeHTTP(wrong, httptest.NewRequest(http.MethodPut, loc+"?digest=sha256:"+strings.Repeat("0", 64), bytes.NewReader(body)))
	require.Equal(t, 400, wrong.Code)
	require.False(t, store.pending)
	store.publishErr = errors.New("write failure")
	failed := httptest.NewRecorder()
	r.ServeHTTP(failed, httptest.NewRequest(http.MethodPut, loc+"?digest="+digest, http.NoBody))
	require.Equal(t, 500, failed.Code)
	store.publishErr = nil
	good := httptest.NewRecorder()
	r.ServeHTTP(good, httptest.NewRequest(http.MethodPut, loc+"?digest="+digest, http.NoBody))
	require.Equal(t, 201, good.Code, good.Body.String())
}

func TestMonolithicUploadAbortAndPutFailures(t *testing.T) {
	for _, phase := range []string{"abort", "put", "read"} {
		t.Run(phase, func(t *testing.T) {
			store := &finalStageStore{appendCountingStore: newAppendCountingStore()}
			r := setupWithStore(store, 1024)
			loc := startUpload(t, r)
			var body io.Reader = bytes.NewReader([]byte("body"))
			fault := errors.New("storage unavailable")
			switch phase {
			case "abort":
				store.abortErr = fault
			case "put":
				store.putErr = fault
			case "read":
				body = failingFinalReader{}
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, loc+"?digest="+fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("body"))), body)
			req.ContentLength = 4
			r.ServeHTTP(w, req)
			require.Equal(t, 500, w.Code)
			require.Equal(t, 1, store.puts, "only the empty session was stored")
		})
	}
}

type failingFinalReader struct{}

func (failingFinalReader) Read([]byte) (int, error) { return 0, context.Canceled }

func TestMonolithicUploadKeepsNonemptyStaging(t *testing.T) {
	store := newAppendCountingStore()
	r := setupWithStore(store, 1024)
	loc := startUpload(t, r)
	patch := httptest.NewRecorder()
	r.ServeHTTP(patch, httptest.NewRequest(http.MethodPatch, loc, strings.NewReader("first")))
	require.Equal(t, 202, patch.Code)
	w := httptest.NewRecorder()
	digest := fmt.Sprintf("sha256:%x", sha256.Sum256([]byte("firstlast")))
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, loc+"?digest="+digest, strings.NewReader("last")))
	require.Equal(t, 201, w.Code, w.Body.String())
	require.Equal(t, 1, store.appends)
	require.Equal(t, 1, store.finalizes)
}
