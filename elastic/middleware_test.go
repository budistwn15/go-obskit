package elastic

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/v2/logger"
)

func TestDisabledMiddleware_StdoutStillWorks(t *testing.T) {
	var buf bytes.Buffer
	m := NewMiddleware(DefaultConfig())
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &buf,
		Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
	})

	log.Info("hello")
	if buf.Len() == 0 {
		t.Fatalf("expected stdout/json log even when elastic disabled")
	}
}

func TestEnabledWithoutEndpoint_NoPanicAndNoCrash(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ""
	cfg.Index = "xeanees-logs"
	cfg.ConnectionLogOutput = &bytes.Buffer{}

	m := NewMiddleware(cfg)
	if m == nil {
		t.Fatalf("middleware should not be nil")
	}

	var out bytes.Buffer
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &out,
		Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
	})
	log.Info("safe even without endpoint")
	if out.Len() == 0 {
		t.Fatalf("stdout log should still work")
	}
}

func TestCanonicalElasticFields(t *testing.T) {
	type request struct {
		auth string
		body string
	}
	received := make(chan request, 1)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"es"}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		received <- request{auth: r.Header.Get("Authorization"), body: string(b)}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ts.URL
	cfg.Index = "direct-index"
	cfg.Username = "user"
	cfg.Password = "pass"
	cfg.BatchSize = 1
	cfg.FlushInterval = 10 * time.Millisecond
	cfg.MaxRetries = 0
	cfg.IndexTimestampSuffix = true
	cfg.IndexTimestampLayout = "20060102"
	cfg.ConnectionLogToStdout = false

	m := NewMiddleware(cfg)

	var buf bytes.Buffer
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &buf,
		Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
	})
	log.Info("direct fields")
	var hit request
	select {
	case hit = <-received:
	case <-time.After(2 * time.Second):
		t.Fatal("expected elastic request")
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatal(err)
	}

	if m.Stats().Sent == 0 {
		t.Fatalf("expected sent log via direct elastic fields")
	}
	if !strings.HasPrefix(hit.auth, "Basic ") {
		t.Fatalf("expected basic auth header")
	}
	if !strings.Contains(hit.body, "direct-index-") {
		t.Fatalf("expected timestamp suffixed index in bulk action, body=%s", hit.body)
	}
}

func TestRetryAndSendSuccess(t *testing.T) {
	var calls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"es"}`))
			return
		}
		if r.URL.Path != "/_bulk" {
			t.Fatalf("expected /_bulk or /")
		}
		_, _ = io.ReadAll(r.Body)
		_ = r.Body.Close()
		n := calls.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"busy"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ts.URL
	cfg.Index = "app"
	cfg.BatchSize = 1
	cfg.FlushInterval = 10 * time.Millisecond
	cfg.Timeout = 1 * time.Second
	cfg.MaxRetries = 3
	cfg.RetryBackoff = 10 * time.Millisecond
	cfg.MaxBackoff = 50 * time.Millisecond

	m := NewMiddleware(cfg)
	defer func() { _ = m.Close(context.Background()) }()

	var buf bytes.Buffer
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &buf,
		Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
	})
	log.Info("test retry")
	time.Sleep(200 * time.Millisecond)

	st := m.Stats()
	if st.Sent == 0 {
		t.Fatalf("expected sent > 0, stats=%+v", st)
	}
	if st.Retried == 0 {
		t.Fatalf("expected retried > 0, stats=%+v", st)
	}
	if st.Failed != 0 {
		t.Fatalf("retry success must not count as failed, stats=%+v", st)
	}
}

func TestBulkItemFailureIsNotCountedAsSent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":true,"items":[{"index":{"status":400}}]}`))
	}))
	defer ts.Close()

	m := NewMiddleware(Config{
		Enabled: true, Endpoint: ts.URL, Index: "logs", BatchSize: 1, MaxRetries: 3,
		RetryBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})
	m.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "partial failure", 0))
	if err := m.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats := m.Stats()
	if stats.Sent != 0 || stats.Failed != 1 || stats.Retried != 0 {
		t.Fatalf("partial failure stats = %+v", stats)
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

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ts.URL
	cfg.Index = "app"
	cfg.BatchSize = 1
	cfg.MaxRetries = 3
	cfg.RetryBackoff = 10 * time.Second
	cfg.MaxBackoff = 10 * time.Second
	m := NewMiddleware(cfg)
	m.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "before close", 0))
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := m.Close(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("close: %v", err)
	}
	<-m.done
	before := m.Stats()
	m.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "after close", 0))
	after := m.Stats()
	if after.Enqueued != before.Enqueued || after.Dropped != before.Dropped+1 {
		t.Fatalf("post-close record must be dropped, before=%+v after=%+v", before, after)
	}
}

func TestConfigOwnership(t *testing.T) {
	client := &http.Client{}
	fields := map[string]any{"owner": "caller"}
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = "http://example.invalid"
	cfg.Index = "app"
	cfg.HTTPClient = client
	cfg.StaticFields = fields
	m := NewMiddleware(cfg)
	fields["owner"] = "mutated"
	if client.Timeout != 0 {
		t.Fatalf("caller client mutated: timeout=%v", client.Timeout)
	}
	if m.cfg.StaticFields["owner"] != "caller" {
		t.Fatalf("static fields were not copied: %v", m.cfg.StaticFields)
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestQueueFullDropsNotBlocking(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = "http://127.0.0.1:1"
	cfg.Index = "app"
	cfg.BatchSize = 10
	cfg.QueueSize = 1
	cfg.Timeout = 10 * time.Millisecond
	cfg.MaxRetries = 0
	cfg.FlushInterval = 1 * time.Second

	m := NewMiddleware(cfg)
	defer func() { _ = m.Close(context.Background()) }()

	var buf bytes.Buffer
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &buf,
		Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
	})

	start := time.Now()
	for i := 0; i < 2000; i++ {
		log.Info("spam", slog.Int("n", i))
	}
	elapsed := time.Since(start)
	if elapsed > 2*time.Second {
		t.Fatalf("logging path too slow, elapsed=%v", elapsed)
	}
	if m.Stats().Dropped == 0 {
		t.Fatalf("expected dropped records when queue full")
	}
}

func TestDocumentContainsAttrs(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "msg", 0)
	rec.AddAttrs(
		slog.String("service.name", "billing"),
		slog.Group("http", slog.String("method", "GET"), slog.Int("status_code", 200)),
	)
	doc := recordToDocument(rec, map[string]any{"service.environment": "prod"})
	if doc["service.name"] != "billing" {
		t.Fatalf("missing service_name")
	}
	httpObj, ok := doc["http"].(map[string]any)
	if !ok || httpObj["method"] != "GET" {
		t.Fatalf("missing grouped field")
	}
	if doc["service.environment"] != "prod" {
		t.Fatalf("missing static field")
	}

	_, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("document should be json serializable: %v", err)
	}
}
