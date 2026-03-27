package lokisink

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

func TestSendLokiPushPayload(t *testing.T) {
	var hitBody []byte
	var hitPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitPath = r.URL.Path
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		hitBody = append([]byte(nil), b...)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	mw := NewMiddleware(Config{
		Enabled:               true,
		Endpoint:              ts.URL,
		BatchSize:             1,
		FlushInterval:         10 * time.Millisecond,
		ConnectionLogToStdout: false,
		Labels:                map[string]string{"app": "test"},
	})
	defer func() { _ = mw.Close(context.Background()) }()

	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      io.Discard,
		Middlewares: []logger.HandlerMiddleware{mw.LoggerMiddleware()},
	})
	log.Info("loki event")

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(hitBody) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(hitBody) == 0 {
		t.Fatalf("expected loki payload")
	}
	if hitPath != "/loki/api/v1/push" {
		t.Fatalf("unexpected path: %s", hitPath)
	}
	var payload map[string]any
	if err := json.Unmarshal(hitBody, &payload); err != nil {
		t.Fatalf("invalid payload json: %v", err)
	}
	streams, ok := payload["streams"].([]any)
	if !ok || len(streams) == 0 {
		t.Fatalf("expected streams in payload")
	}
}
