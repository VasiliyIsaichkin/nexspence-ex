package oci_test

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type countedUploadStore struct {
	*synthStore
	gets, finalizes int
}

func TestFinalizeUploadUnknownLengthBody(t *testing.T) {
	store := newSynthStore()
	r := setupWithStore(store, 2048)
	loc := startUpload(t, r)
	req := httptest.NewRequest(http.MethodPut, loc+"?digest="+synthDigest(2048), synthReader(2048))
	req.ContentLength = -1
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestFinalizeUploadEmptyUnknownBodyAtLimit(t *testing.T) {
	store := newSynthStore()
	r := setupWithStore(store, 2048)
	loc := startUpload(t, r)
	patch := httptest.NewRequest(http.MethodPatch, loc, synthReader(2048))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, patch)
	require.Equal(t, http.StatusAccepted, w.Code)
	req := httptest.NewRequest(http.MethodPut, loc+"?digest="+synthDigest(2048), http.NoBody)
	req.ContentLength = -1
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func (s *countedUploadStore) Get(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	s.gets++
	return s.synthStore.Get(ctx, key)
}
func (s *countedUploadStore) FinalizeAppend(ctx context.Context, key string) error {
	s.finalizes++
	return s.synthStore.FinalizeAppend(ctx, key)
}

func TestFinalizeUploadRemoteReads(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int64
		gets int
	}{{"config", 2048, 1}, {"buffer-boundary", 1 << 20, 1}, {"stream", (1 << 20) + 1, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			store := &countedUploadStore{synthStore: newSynthStore()}
			w := pushSynthLayer(t, setupWithStore(store, 0), tc.size, synthDigest(tc.size))
			require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
			require.Equal(t, 1, store.finalizes, "one upload must be finalized only once")
			require.Equal(t, tc.gets, store.gets, "small blobs must not make a second remote read")
		})
	}
}
