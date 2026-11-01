package lokisink

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/v2/logger"
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

func TestCloseCancelsRetryAndDropsFurtherRecords(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestStarted <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	mw := NewMiddleware(Config{
		Enabled:               true,
		Endpoint:              ts.URL,
		BatchSize:             1,
		MaxRetries:            3,
		RetryBackoff:          10 * time.Second,
		MaxBackoff:            10 * time.Second,
		ConnectionLogToStdout: false,
	})
	mw.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "before close", 0))
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := mw.Close(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("close: %v", err)
	}
	<-mw.done
	before := mw.Stats()
	mw.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "after close", 0))
	after := mw.Stats()
	if after.Enqueued != before.Enqueued || after.Dropped != before.Dropped+1 {
		t.Fatalf("post-close record must be dropped, before=%+v after=%+v", before, after)
	}
	if after.Failed != 1 || after.Retried == 0 {
		t.Fatalf("failed records and retry attempts must be exact, stats=%+v", after)
	}
}

func TestConfigOwnership(t *testing.T) {
	client := &http.Client{}
	labels := map[string]string{"owner": "caller"}
	fields := map[string]any{"owner": "caller"}
	mw := NewMiddleware(Config{
		Enabled:      true,
		Endpoint:     "http://example.invalid",
		HTTPClient:   client,
		Labels:       labels,
		StaticFields: fields,
	})
	labels["owner"] = "mutated"
	fields["owner"] = "mutated"
	if client.Timeout != 0 {
		t.Fatalf("caller client mutated: timeout=%v", client.Timeout)
	}
	if mw.cfg.Labels["owner"] != "caller" || mw.cfg.StaticFields["owner"] != "caller" {
		t.Fatalf("config maps were not copied: labels=%v fields=%v", mw.cfg.Labels, mw.cfg.StaticFields)
	}
	if err := mw.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestSendLokiPushPayload(t *testing.T) {
	type request struct {
		body []byte
		path string
	}
	received := make(chan request, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		received <- request{body: b, path: r.URL.Path}
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

	var hit request
	select {
	case hit = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("expected loki payload")
	}
	if hit.path != "/loki/api/v1/push" {
		t.Fatalf("unexpected path: %s", hit.path)
	}
	var payload map[string]any
	if err := json.Unmarshal(hit.body, &payload); err != nil {
		t.Fatalf("invalid payload json: %v", err)
	}
	streams, ok := payload["streams"].([]any)
	if !ok || len(streams) == 0 {
		t.Fatalf("expected streams in payload")
	}
}
