package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendSeedCopiesLargeStagingInBoundedRanges(t *testing.T) {
	var ranges []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("X-Amz-Copy-Source-Range"))
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<CopyPartResult><ETag>"part-%d"</ETag></CopyPartResult>`, len(ranges))
	}))
	defer server.Close()
	store, err := NewS3BlobStore(context.Background(), S3Options{
		Bucket: "test", Region: "us-east-1", Endpoint: server.URL,
		AccessKeyID: "test", SecretAccessKey: "test", ForcePathStyle: true,
	})
	require.NoError(t, err)
	state := &s3AppendState{UploadID: "upload"}
	const size = int64(6<<30) + 7
	require.NoError(t, store.seedFromCopy(context.Background(), "staging", state, size))
	require.Equal(t, []string{
		"bytes=0-1073741823", "bytes=1073741824-2147483647",
		"bytes=2147483648-3221225471", "bytes=3221225472-4294967295",
		"bytes=4294967296-5368709119", "bytes=5368709120-6442450950",
	}, ranges, "a resumed first PATCH may exceed S3's single-copy size")
	require.Equal(t, size, state.Uploaded)
	require.Len(t, state.Parts, 6)
	require.Equal(t, int32(6), state.Parts[5].Number)
	require.Equal(t, int64(1<<30)+7, state.Parts[5].Size,
		"the last seed part becomes non-final when another chunk is appended")
}
