package oci_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// One PATCH is how Skopeo sends a layer. It must not turn into a sequence of
// durable append checkpoints when there are no previously acknowledged bytes.
func TestFirstPatchStagesOnceAndResumesAcrossHandlers(t *testing.T) {
	store := newAppendCountingStore()
	r := setupWithStore(store, 1024)
	loc := startUpload(t, r)
	first := httptest.NewRecorder()
	r.ServeHTTP(first, httptest.NewRequest(http.MethodPatch, loc, strings.NewReader("first")))
	require.Equal(t, 202, first.Code)
	require.Equal(t, "0-4", first.Header().Get("Range"))
	require.Zero(t, store.appends)
	require.Equal(t, 2, store.puts, "empty session and one completed first chunk")

	r = setupWithStore(store, 1024)
	second := httptest.NewRecorder()
	r.ServeHTTP(second, httptest.NewRequest(http.MethodPatch, loc, strings.NewReader("last")))
	require.Equal(t, 202, second.Code)
	require.Equal(t, "0-8", second.Header().Get("Range"))
	require.Equal(t, 1, store.appends)
	done := httptest.NewRecorder()
	r.ServeHTTP(done, httptest.NewRequest(http.MethodPut, loc+"?digest="+digest("firstlast"), nil))
	require.Equal(t, 201, done.Code, done.Body.String())
	read := httptest.NewRecorder()
	r.ServeHTTP(read, httptest.NewRequest(http.MethodGet, done.Header().Get("Location"), nil))
	require.Equal(t, []byte("firstlast"), read.Body.Bytes())
}

func TestFirstPatchRejectsIncompleteOrOversizedBodyWithoutAcknowledgingIt(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int
		declared int64
		status   int
	}{
		{"short", 31, 32, 500},
		{"long", 32, 31, 500},
		{"limit-known", 1025, 1025, 413},
		{"limit-unknown", 1025, -1, 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newAppendCountingStore()
			r := setupWithStore(store, 1024)
			loc := startUpload(t, r)
			req := httptest.NewRequest(http.MethodPatch, loc, bytes.NewReader(bytes.Repeat([]byte{1}, tc.size)))
			req.ContentLength = tc.declared
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			n, err := store.Size(context.Background(), "_uploads/docker/"+loc[strings.LastIndex(loc, "/")+1:])
			require.NoError(t, err)
			require.Zero(t, n)
		})
	}
}

func TestFirstPatchStorageFailureKeepsSessionRetryable(t *testing.T) {
	store := &finalStageStore{appendCountingStore: newAppendCountingStore()}
	r := setupWithStore(store, 1024)
	loc := startUpload(t, r)
	store.putErr = errors.New("temporary storage failure")
	failed := httptest.NewRecorder()
	r.ServeHTTP(failed, httptest.NewRequest(http.MethodPatch, loc, strings.NewReader("body")))
	require.Equal(t, 500, failed.Code)
	store.putErr = nil
	retry := httptest.NewRecorder()
	r.ServeHTTP(retry, httptest.NewRequest(http.MethodPatch, loc, strings.NewReader("body")))
	require.Equal(t, 202, retry.Code)
	done := httptest.NewRecorder()
	r.ServeHTTP(done, httptest.NewRequest(http.MethodPut, loc+"?digest="+digest("body"), nil))
	require.Equal(t, 201, done.Code, done.Body.String())
}
