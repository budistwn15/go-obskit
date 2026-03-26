package httpin

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
)

type responseCaptureWriter struct {
	http.ResponseWriter
	status    int
	sizeBytes int64
	capture   bool
	maxBytes  int
	body      bytes.Buffer
	truncated bool
}

func newResponseCaptureWriter(w http.ResponseWriter, capture bool, maxBytes int) *responseCaptureWriter {
	return &responseCaptureWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		capture:        capture,
		maxBytes:       maxBytes,
	}
}

func (w *responseCaptureWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseCaptureWriter) Write(p []byte) (int, error) {
	if w.capture {
		w.captureBytes(p)
	}
	n, err := w.ResponseWriter.Write(p)
	w.sizeBytes += int64(n)
	return n, err
}

func (w *responseCaptureWriter) ReadFrom(r io.Reader) (int64, error) {
	n, err := io.Copy(w, r)
	return n, err
}

func (w *responseCaptureWriter) bodyString() string {
	return w.body.String()
}

func (w *responseCaptureWriter) captureBytes(p []byte) {
	if w.maxBytes <= 0 || w.truncated {
		return
	}
	remaining := w.maxBytes - w.body.Len()
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(p) > remaining {
		_, _ = w.body.Write(p[:remaining])
		w.truncated = true
		return
	}
	_, _ = w.body.Write(p)
}

func (w *responseCaptureWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseCaptureWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *responseCaptureWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
