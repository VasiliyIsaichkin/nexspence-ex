package oci

import (
	"context"
	"fmt"
	"io"
)

// stageFinal uses the ordinary streaming Put for an empty session.
// Bytes still go to staging; digest and write policy are checked before publish.
func (s uploadStore) stageFinal(ctx context.Context, id string, r io.Reader, declared int64) (direct, ok bool, err error) {
	stored, ok := s.size(ctx, id)
	if !ok {
		return false, false, nil
	}
	if stored != 0 {
		_, ok, err = s.append(ctx, id, r, declared)
		return false, ok, err
	}
	_, err = s.stageEmpty(ctx, id, r, declared)
	return err == nil, true, err
}

// stageEmpty commits the first request body atomically. A failed request has
// no acknowledged bytes, and the client can retry it from offset zero.
func (s uploadStore) stageEmpty(ctx context.Context, id string, r io.Reader, declared int64) (int64, error) {
	if s.deps.MaxUploadBytes > 0 && declared > s.deps.MaxUploadBytes {
		return 0, errUploadTooLarge
	}
	// Even an empty PATCH can leave multipart state that would later overwrite
	// the direct Put. Never write until that state has been successfully removed.
	if as, appendable := s.appendable(); appendable {
		if err := as.AbortAppend(ctx, s.key(id)); err != nil {
			return 0, err
		}
	}
	checked := &checkedUploadReader{reader: r, limit: s.deps.MaxUploadBytes, declared: declared}
	if err := s.deps.BlobStore.Put(ctx, s.key(id), checked, declared); err != nil {
		return 0, err
	}
	return checked.received, nil
}

// An overflow or truncated body is an error to the storage writer, so it does
// not commit a silently shortened object. Memory use is independent of size.
type checkedUploadReader struct {
	reader                    io.Reader
	limit, declared, received int64
}

func (r *checkedUploadReader) Read(p []byte) (int, error) {
	remaining := int64(len(p))
	if r.limit > 0 && r.limit-r.received < remaining {
		remaining = r.limit - r.received + 1
	}
	if r.declared >= 0 && r.declared-r.received < remaining {
		remaining = r.declared - r.received + 1
	}
	n, err := r.reader.Read(p[:remaining])
	r.received += int64(n)
	if r.limit > 0 && r.received > r.limit {
		return 0, errUploadTooLarge
	}
	if r.declared >= 0 && r.received > r.declared {
		return 0, fmt.Errorf("upload body exceeds declared size")
	}
	if err == io.EOF && r.declared >= 0 && r.received != r.declared {
		return n, io.ErrUnexpectedEOF
	}
	return n, err
}
