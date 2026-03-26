package nethttp

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
)

type responseWriter struct {
	http.ResponseWriter
	status        int
	sizeBytes     int64
	captureBody   bool
	maxBodyBytes  int
	body          bytes.Buffer
	bodyTruncated bool
}

func newResponseWriter(w http.ResponseWriter, captureBody bool, maxBodyBytes int) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		status:         http.StatusOK,
		captureBody:    captureBody,
		maxBodyBytes:   maxBodyBytes,
	}
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.captureBody {
		w.capture(p)
	}
	n, err := w.ResponseWriter.Write(p)
	w.sizeBytes += int64(n)
	return n, err
}

func (w *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	return io.Copy(w, r)
}

func (w *responseWriter) capture(p []byte) {
	if w.maxBodyBytes <= 0 || w.bodyTruncated {
		return
	}
	remaining := w.maxBodyBytes - w.body.Len()
	if remaining <= 0 {
		w.bodyTruncated = true
		return
	}
	if len(p) > remaining {
		_, _ = w.body.Write(p[:remaining])
		w.bodyTruncated = true
		return
	}
	_, _ = w.body.Write(p)
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return h.Hijack()
}

func (w *responseWriter) Push(target string, opts *http.PushOptions) error {
	p, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return p.Push(target, opts)
}
