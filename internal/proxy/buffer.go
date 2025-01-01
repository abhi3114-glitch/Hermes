package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// Buffer wraps request body with buffering capabilities
type Buffer struct {
	maxSize int64
}

// NewBuffer creates a new request buffer
func NewBuffer(maxSize int64) *Buffer {
	return &Buffer{maxSize: maxSize}
}

// BufferRequest reads and buffers the request body
func (b *Buffer) BufferRequest(r *http.Request) (*bytes.Buffer, error) {
	if r.Body == nil {
		return nil, nil
	}

	// Limit the reader to prevent OOM
	limitedReader := io.LimitReader(r.Body, b.maxSize+1)

	buf := &bytes.Buffer{}
	n, err := io.Copy(buf, limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to buffer request body: %w", err)
	}

	if n > b.maxSize {
		return nil, fmt.Errorf("request body too large: %d bytes (max: %d)", n, b.maxSize)
	}

	return buf, nil
}

// WrapBody wraps a buffer as a ReadCloser for re-reading
func WrapBody(buf *bytes.Buffer) io.ReadCloser {
	if buf == nil {
		return nil
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes()))
}
