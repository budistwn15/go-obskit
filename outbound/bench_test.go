package outbound

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
)

func BenchmarkTransportRoundTripMinimal(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"ok":true}`))),
			}, nil
		},
	)
	tr := NewTransport(rt, log, DefaultOptions())
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := tr.RoundTrip(req.Clone(req.Context()))
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func BenchmarkTransportRoundTripWithBodyCapture(b *testing.B) {
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
	
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"boom","token":"abc"}`)),
			}, nil
		},
	)
	tr := NewTransport(rt, log, opts)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest(
			http.MethodPost, "http://example.com/login", strings.NewReader(`{"password":"secret"}`),
		)
		req.Header.Set("Content-Type", "application/json")
		resp, err := tr.RoundTrip(req)
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}

func BenchmarkTransportRoundTripSampledSuccess(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 10
	
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)
	tr := NewTransport(rt, log, opts)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := tr.RoundTrip(req.Clone(req.Context()))
		if err != nil {
			b.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}
}
