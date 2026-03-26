package nethttp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
)

func BenchmarkMiddlewareMinimal(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	opts := DefaultOptions()
	h := Middleware(log, opts)(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			},
		),
	)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req.Clone(req.Context()))
	}
}

func BenchmarkMiddlewareWithBodyCapture(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	opts := DefaultOptions()
	opts.CaptureRequestBody = true
	opts.CaptureResponseBody = true
	opts.LogErrorBodies = true
	opts.MaxBodyBytes = 128
	
	h := Middleware(log, opts)(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"boom","token":"abc"}`))
			},
		),
	)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(
			http.MethodPost, "http://example.com/login", strings.NewReader(`{"password":"secret"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
	}
}

func BenchmarkMiddlewareSampledSuccess(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 10
	h := Middleware(log, opts)(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			},
		),
	)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req.Clone(req.Context()))
	}
}
