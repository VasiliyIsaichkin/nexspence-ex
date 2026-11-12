package oci

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

const smallUploadBuffer = 1 << 20

// verified finalizes staging once and checks the claimed digest before the
// ordinary StoreArtifact path is allowed to publish it. Config blobs reuse a
// bounded buffer; large layers still stream, without another size/finalize pass.
func (s uploadStore) verified(ctx context.Context, id, digest string, direct bool) (body io.ReadCloser, size int64, ok bool, message string, err error) {
	var verify io.ReadCloser
	if direct {
		// stageFinal already checked the session and published its staging key.
		verify, size, err = s.deps.BlobStore.Get(ctx, s.key(id))
		ok = true
	} else {
		verify, size, ok, err = s.open(ctx, id)
	}
	if err != nil || !ok {
		return nil, size, ok, "", err
	}
	if size >= 0 && size <= smallUploadBuffer {
		data, readErr := io.ReadAll(io.LimitReader(verify, smallUploadBuffer+1))
		_ = verify.Close()
		if readErr != nil {
			return nil, size, true, "", readErr
		}
		if int64(len(data)) != size {
			return nil, size, true, "", fmt.Errorf("staged upload size changed")
		}
		message = digestMismatch(digest, data)
		if message != "" {
			return nil, size, true, message, nil
		}
		return io.NopCloser(bytes.NewReader(data)), size, true, "", nil
	}
	message, err = digestMismatchStream(digest, verify)
	_ = verify.Close()
	if err != nil || message != "" {
		return nil, size, true, message, err
	}
	// The session is already complete. Repeating open here performs metadata
	// GETs, HEADs and multipart finalization again on a remote blob store.
	body, actualSize, err := s.deps.BlobStore.Get(ctx, s.key(id))
	if err != nil {
		return nil, size, true, "", err
	}
	if actualSize != size {
		_ = body.Close()
		return nil, size, true, "", fmt.Errorf("staged upload size changed")
	}
	return body, size, true, "", nil
}
