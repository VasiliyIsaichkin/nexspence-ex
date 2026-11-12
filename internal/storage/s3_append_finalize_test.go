package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestS3SmallFinalizeIsRetryableAfterCleanupFailure(t *testing.T) {
	want := []byte("small registry config")
	state := s3AppendState{UploadID: "diagnostic-upload", Pending: want}
	meta, _ := json.Marshal(state)
	var stored []byte
	deletes, partCalls := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, appendMetaSuffix):
			if deletes > 1 {
				w.WriteHeader(404)
				_, _ = w.Write([]byte("<Error><Code>NoSuchKey</Code></Error>"))
				return
			}
			_, _ = w.Write(meta)
		case r.URL.Query().Get("uploadId") != "":
			if r.Method == http.MethodDelete {
				w.WriteHeader(404)
				_, _ = w.Write([]byte("<Error><Code>NoSuchUpload</Code></Error>"))
				return
			}
			partCalls++
			w.WriteHeader(400)
		case r.Method == http.MethodPut:
			stored, _ = io.ReadAll(r.Body)
			w.Header().Set("ETag", "\"test\"")
		case r.Method == http.MethodDelete:
			deletes++
			if deletes == 1 {
				w.WriteHeader(503)
				return
			}
			w.WriteHeader(204)
		default:
			w.WriteHeader(500)
		}
	}))
	defer srv.Close()
	client := s3.NewFromConfig(aws.Config{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("test", "test", ""), Retryer: func() aws.Retryer { return aws.NopRetryer{} }, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired}, func(o *s3.Options) { o.BaseEndpoint = aws.String(srv.URL); o.UsePathStyle = true })
	store := &S3BlobStore{client: client, bucket: "test"}
	require.Error(t, store.FinalizeAppend(context.Background(), "upload-key"))
	require.True(t, bytes.Equal(want, stored), "pending bytes must be durable before cleanup")
	require.NoError(t, store.FinalizeAppend(context.Background(), "upload-key"))
	require.NoError(t, store.FinalizeAppend(context.Background(), "upload-key"))
	require.Equal(t, 0, partCalls, "small blobs do not need multipart completion")
	require.Equal(t, 2, deletes)
}
