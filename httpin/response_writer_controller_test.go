package httpin

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type deadlineWriter struct {
	*httptest.ResponseRecorder
	writeDeadline time.Time
}

func (w *deadlineWriter) SetWriteDeadline(deadline time.Time) error {
	w.writeDeadline = deadline
	return nil
}

func TestResponseWriterUnwrapSupportsResponseController(t *testing.T) {
	base := &deadlineWriter{ResponseRecorder: httptest.NewRecorder()}
	w := newResponseCaptureWriter(base, false, 0)
	deadline := time.Now().Add(time.Second)
	if err := http.NewResponseController(w).SetWriteDeadline(deadline); err != nil {
		t.Fatalf("set write deadline: %v", err)
	}
	if !base.writeDeadline.Equal(deadline) {
		t.Fatalf("write deadline = %v, want %v", base.writeDeadline, deadline)
	}
}
