package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

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

func TestDirectElasticFieldsMapping(t *testing.T) {
	var hitAuth string
	var hitBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"es"}`))
			return
		}
		hitAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		hitBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"errors":false}`))
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ElasticURL = ts.URL
	cfg.ElasticIndex = "direct-index"
	cfg.ElasticUsername = "user"
	cfg.ElasticPassword = "pass"
	cfg.BatchSize = 1
	cfg.FlushInterval = 10 * time.Millisecond
	cfg.MaxRetries = 0
	cfg.IndexTimestampSuffix = true
	cfg.IndexTimestampLayout = "20060102"
	cfg.BootstrapOnStart = false

	m := NewMiddleware(cfg)
	defer func() { _ = m.Close(context.Background()) }()

	var buf bytes.Buffer
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      &buf,
		Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
	})
	log.Info("direct fields")
	time.Sleep(80 * time.Millisecond)

	if m.Stats().Sent == 0 {
		t.Fatalf("expected sent log via direct elastic fields")
	}
	if !strings.HasPrefix(hitAuth, "Basic ") {
		t.Fatalf("expected basic auth header")
	}
	if !strings.Contains(hitBody, "direct-index-") {
		t.Fatalf("expected timestamp suffixed index in bulk action, body=%s", hitBody)
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
	cfg.BootstrapOnStart = false

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
	cfg.BootstrapOnStart = false

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
		slog.String("service_name", "billing"),
		slog.Group("http", slog.String("method", "GET"), slog.Int("status_code", 200)),
	)
	doc := recordToDocument(rec, map[string]any{"environment": "prod"})
	if doc["service_name"] != "billing" {
		t.Fatalf("missing service_name")
	}
	httpObj, ok := doc["http"].(map[string]any)
	if !ok || httpObj["method"] != "GET" {
		t.Fatalf("missing grouped field")
	}
	if doc["environment"] != "prod" {
		t.Fatalf("missing static field")
	}

	_, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("document should be json serializable: %v", err)
	}
}

func TestMonitorConnectionStatus(t *testing.T) {
	var onMonitorCalls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"es"}`))
			return
		}
		if r.URL.Path == "/_bulk" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"errors":false}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ts.URL
	cfg.Index = "app"
	cfg.BatchSize = 1
	cfg.FlushInterval = 10 * time.Millisecond
	cfg.EnableMonitor = true
	cfg.MonitorInterval = 20 * time.Millisecond
	cfg.BootstrapOnStart = false
	cfg.OnMonitor = func(st ConnectionStatus) {
		onMonitorCalls.Add(1)
	}
	statusOut := &lockedBuffer{}
	cfg.ConnectionLogOutput = statusOut

	m := NewMiddleware(cfg)
	defer func() { _ = m.Close(context.Background()) }()

	time.Sleep(100 * time.Millisecond)
	st := m.MonitorStatus()
	if !st.Up {
		t.Fatalf("expected monitor up=true, got=%+v", st)
	}
	if st.LastCheckedAt.IsZero() {
		t.Fatalf("expected last checked timestamp")
	}
	if onMonitorCalls.Load() == 0 {
		t.Fatalf("expected monitor callback at least once")
	}

	manual := m.HealthCheck(context.Background())
	if !manual.Up {
		t.Fatalf("manual health check should be up")
	}
	logText := statusOut.String()
	if !strings.Contains(logText, "elastic sink enabled") {
		t.Fatalf("expected sink enabled status log")
	}
	if !strings.Contains(logText, "elastic connection check") {
		t.Fatalf("expected connection check status log")
	}
}
