package sinkenv

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
)

func TestFromEnv_StdoutProvider(t *testing.T) {
	t.Setenv("OBSKIT_SINK_PROVIDER", "stdout")
	rt := FromEnv()
	if rt.Provider != "stdout" {
		t.Fatalf("expected stdout provider, got=%s", rt.Provider)
	}
	if len(rt.Middlewares) != 0 {
		t.Fatalf("stdout provider should not inject middlewares")
	}
}

func TestFromEnv_HTTPSinkProvider(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	t.Setenv("OBSKIT_SINK_PROVIDER", "http")
	t.Setenv("OBSKIT_HTTP_SINK_ENABLED", "true")
	t.Setenv("OBSKIT_HTTP_SINK_URL", ts.URL)
	t.Setenv("OBSKIT_HTTP_SINK_BATCH_SIZE", "1")

	rt := FromEnv()
	if rt.Provider != "http" {
		t.Fatalf("expected http provider, got=%s", rt.Provider)
	}
	l := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      io.Discard,
		Middlewares: rt.Middlewares,
	})
	l.Info("hello")
	_ = rt.Close(context.Background())
	if hits == 0 {
		t.Fatalf("expected http sink to receive request")
	}
}

func TestFromEnv_LokiProvider(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	t.Setenv("OBSKIT_SINK_PROVIDER", "loki")
	t.Setenv("OBSKIT_LOKI_ENABLED", "true")
	t.Setenv("OBSKIT_LOKI_URL", ts.URL)
	t.Setenv("OBSKIT_LOKI_BATCH_SIZE", "1")

	rt := FromEnv()
	if rt.Provider != "loki" {
		t.Fatalf("expected loki provider, got=%s", rt.Provider)
	}
	l := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      io.Discard,
		Middlewares: rt.Middlewares,
	})
	l.Info("hello")
	_ = rt.Close(context.Background())
	if hits == 0 {
		t.Fatalf("expected loki sink to receive request")
	}
}

func TestParseHeaders(t *testing.T) {
	h := parseHeaders("X-Api-Key: abc, Authorization: Bearer token, invalid")
	if h["X-Api-Key"] != "abc" || h["Authorization"] != "Bearer token" {
		t.Fatalf("unexpected parsed headers: %+v", h)
	}
}

func TestFromEnv_DefaultWithoutProvider(t *testing.T) {
	_ = os.Unsetenv("OBSKIT_SINK_PROVIDER")
	rt := FromEnv()
	if rt.Provider != "stdout" {
		t.Fatalf("expected default provider stdout")
	}
}
