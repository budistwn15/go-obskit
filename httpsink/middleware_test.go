package httpsink

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

func TestDisabledSinkDoesNotBreakLogging(t *testing.T) {
	var out bytes.Buffer
	mw := NewMiddleware(Config{Enabled: true})
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &out,
		Middlewares: []logger.HandlerMiddleware{mw.LoggerMiddleware()},
	})
	log.Info("hello")
	if !strings.Contains(out.String(), "hello") {
		t.Fatalf("stdout log should remain visible when sink config incomplete")
	}
}

func TestSendNDJSON(t *testing.T) {
	var got string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		got = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	mw := NewMiddleware(Config{
		Enabled:               true,
		Endpoint:              ts.URL,
		BatchSize:             1,
		FlushInterval:         10 * time.Millisecond,
		ConnectionLogToStdout: false,
	})
	defer func() { _ = mw.Close(context.Background()) }()

	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      io.Discard,
		Middlewares: []logger.HandlerMiddleware{mw.LoggerMiddleware()},
	})
	log.LogAttrs(context.Background(), slog.LevelInfo, "ndjson event", slog.String("event", "test"))

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(got, "ndjson event") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("sink payload not received, got=%q", got)
}
