package nethttp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

func parseLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid line: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestMiddleware_CorrelationPropagationAndBodyRestore(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(
		logger.Config{
			ServiceName: "svc",
			Environment: "production",
			Level:       logger.LevelInfo,
			Output:      &buf,
		},
	)
	opts := DefaultOptions()
	opts.CaptureRequestBody = true
	opts.LogRequestComplete = true
	opts.CorrelationHeader = "X-Correlation-ID"

	mw := Middleware(log, opts)

	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"name":"john"}` {
				t.Fatalf("body should be restored for downstream")
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("ok"))
		},
	)

	req := httptest.NewRequest(http.MethodPost, "http://example.com/orders?id=1", strings.NewReader(`{"name":"john"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Correlation-ID", "corr-1")
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)

	if rec.Header().Get("X-Correlation-ID") != "corr-1" {
		t.Fatalf("missing propagated correlation header")
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status not captured")
	}

	logs := parseLines(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("expected log events")
	}
	last := logs[len(logs)-1]
	if last["duration_ms"] == nil {
		t.Fatalf("duration_ms must exist")
	}
	if int(last["http.status_code"].(float64)) != http.StatusCreated {
		t.Fatalf("status code must be logged")
	}
}

func TestMiddleware_PreservesBodyWhenCaptureReadReturnsDataAndError(t *testing.T) {
	opts := DefaultOptions()
	opts.CaptureRequestBody = true
	received := ""
	h := Middleware(nil, opts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = string(body)
	}))
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(&partialErrorReader{data: []byte(`{"name":"john"}`)})

	h.ServeHTTP(httptest.NewRecorder(), req)

	if received != `{"name":"john"}` {
		t.Fatalf("downstream body = %q, want complete original body", received)
	}
}

func TestMiddleware_RecordsFirstResponseStatus(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	h := Middleware(log, DefaultOptions())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	logs := parseLines(t, buf.String())
	if got := int(logs[len(logs)-1]["http.status_code"].(float64)); got != http.StatusCreated {
		t.Fatalf("logged status = %d, want first status %d", got, http.StatusCreated)
	}
}

type partialErrorReader struct {
	data []byte
	done bool
}

func (r *partialErrorReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, io.EOF
	}
	r.done = true
	return copy(p, r.data), io.ErrUnexpectedEOF
}

func TestMiddleware_SuccessSamplingKeepsSlow(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(
		logger.Config{
			ServiceName: "svc",
			Environment: "production",
			Level:       logger.LevelInfo,
			Output:      &buf,
		},
	)
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 1000
	opts.SlowRequestThreshold = 1 * time.Millisecond

	mw := Middleware(log, opts)
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		},
	)
	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	rec := httptest.NewRecorder()
	mw(next).ServeHTTP(rec, req)

	logs := parseLines(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("slow events should not be sampled out")
	}
	last := logs[len(logs)-1]
	if slow, ok := last["slow"].(bool); !ok || !slow {
		t.Fatalf("expected slow=true")
	}
	if _, ok := last["threshold_ms"]; !ok {
		t.Fatalf("expected threshold_ms")
	}
}

func TestMiddleware_ForensicCapturesBodiesAndHeadersOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(
		logger.Config{
			ServiceName: "svc",
			Environment: "production",
			Level:       logger.LevelInfo,
			Output:      &buf,
		},
	)
	opts := ForensicOptions()
	opts.SuccessSampleEvery = 1

	mw := Middleware(log, opts)
	next := http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Upstream", "auth-service")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true,"token":"abc123"}`))
		},
	)
	req := httptest.NewRequest(
		http.MethodPost, "http://example.com/api/v1/auth/login?from=web",
		strings.NewReader(`{"email":"a@b.c","password":"secret"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer mytoken")
	req.Header.Set("X-Correlation-ID", "corr-1")
	rec := httptest.NewRecorder()

	mw(next).ServeHTTP(rec, req)
	logs := parseLines(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("expected logs")
	}
	last := logs[len(logs)-1]
	if _, ok := last["http.request.body"]; !ok {
		t.Fatalf("expected request body")
	}
	if _, ok := last["http.response.body"]; !ok {
		t.Fatalf("expected response body on success")
	}
	reqHeaders, ok := last["http.request.headers"].(map[string]any)
	if !ok || reqHeaders["Authorization"] != "***redacted***" {
		t.Fatalf("expected request headers with redaction")
	}
	if _, ok := last["http.response.headers"]; !ok {
		t.Fatalf("expected response headers")
	}
}
