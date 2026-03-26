package httpin

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

func newTestSlog(buf *bytes.Buffer, level logger.Level) *slog.Logger {
	return logger.New(
		logger.Config{
			ServiceName: "test",
			Environment: "production",
			Level:       level,
			Output:      buf,
		},
	)
}

func parseLogLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var item map[string]any
		if err := json.Unmarshal([]byte(line), &item); err != nil {
			t.Fatalf("invalid json log line: %v line=%s", err, line)
		}
		out = append(out, item)
	}
	return out
}

func findByMsg(logs []map[string]any, msg string) map[string]any {
	for _, l := range logs {
		if l["msg"] == msg {
			return l
		}
	}
	return nil
}

func TestCorrelationPropagationFromIncomingHeader(t *testing.T) {
	var buf bytes.Buffer
	mw := New(
		Config{
			Logger: newTestSlog(&buf, logger.LevelInfo),
		},
	)
	
	handler := mw.Wrap(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if got := logger.CorrelationID(r.Context()); got != "corr-abc" {
					t.Fatalf("expected correlation id in context, got=%s", got)
				}
				w.WriteHeader(http.StatusNoContent)
			},
		),
	)
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("X-Correlation-ID", "corr-abc")
	rr := httptest.NewRecorder()
	
	handler.ServeHTTP(rr, req)
	
	if got := rr.Header().Get("X-Correlation-ID"); got != "corr-abc" {
		t.Fatalf("expected response correlation header corr-abc, got=%s", got)
	}
}

func TestCorrelationGeneratedWhenMissing(t *testing.T) {
	var buf bytes.Buffer
	mw := New(
		Config{
			Logger: newTestSlog(&buf, logger.LevelInfo),
			CorrelationIDFactory: func() string {
				return "generated-corr-id"
			},
		},
	)
	
	handler := mw.Wrap(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				if got := logger.CorrelationID(r.Context()); got != "generated-corr-id" {
					t.Fatalf("expected generated correlation id, got=%s", got)
				}
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	if got := rr.Header().Get("X-Correlation-ID"); got != "generated-corr-id" {
		t.Fatalf("expected generated correlation id in response header, got=%s", got)
	}
}

func TestDurationAndResponseCapture(t *testing.T) {
	var buf bytes.Buffer
	mw := New(
		Config{
			Logger:             newTestSlog(&buf, logger.LevelInfo),
			LogResponseHeaders: true,
		},
	)
	
	handler := mw.Wrap(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(10 * time.Millisecond)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"ok":true}`))
			},
		),
	)
	
	req := httptest.NewRequest(http.MethodPost, "http://example.com/orders?status=1", nil)
	req.Header.Set("User-Agent", "test-agent")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	logs := parseLogLines(t, buf.String())
	summary := findByMsg(logs, "incoming http request")
	if summary == nil {
		t.Fatalf("expected summary log")
	}
	if int(summary["http_status_code"].(float64)) != http.StatusCreated {
		t.Fatalf("expected status 201, got=%v", summary["http_status_code"])
	}
	if int(summary["response_size_bytes"].(float64)) != len(`{"ok":true}`) {
		t.Fatalf("unexpected response size: %v", summary["response_size_bytes"])
	}
	if summary["duration_ms"].(float64) <= 0 {
		t.Fatalf("expected duration_ms > 0, got=%v", summary["duration_ms"])
	}
}

func TestResponseBodyTruncation(t *testing.T) {
	var buf bytes.Buffer
	mw := New(
		Config{
			Logger:              newTestSlog(&buf, logger.LevelDebug),
			CaptureResponseBody: true,
			LogResponseBody:     true,
			MaxBodyCaptureBytes: 5,
			ShouldLogDetail: func(*http.Request, int, time.Duration) bool {
				return true
			},
		},
	)
	
	handler := mw.Wrap(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"value":"123456789"}`))
			},
		),
	)
	
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	handler.ServeHTTP(rr, req)
	
	logs := parseLogLines(t, buf.String())
	bodyLog := findByMsg(logs, "incoming response body")
	if bodyLog == nil {
		t.Fatalf("expected response body log")
	}
	if bodyLog["http_response_body_truncated"] != true {
		t.Fatalf("expected truncated=true, got=%v", bodyLog["http_response_body_truncated"])
	}
}

func TestRequestBodyRedaction(t *testing.T) {
	var buf bytes.Buffer
	mw := New(
		Config{
			Logger:             newTestSlog(&buf, logger.LevelDebug),
			CaptureRequestBody: true,
			LogRequestBody:     true,
			ShouldLogDetail: func(*http.Request, int, time.Duration) bool {
				return true
			},
		},
	)
	
	handler := mw.Wrap(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				b, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body failed: %v", err)
				}
				if !strings.Contains(string(b), `"password":"secret"`) {
					t.Fatalf("request body should still reach handler intact")
				}
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"john","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	
	logs := parseLogLines(t, buf.String())
	bodyLog := findByMsg(logs, "incoming request body")
	if bodyLog == nil {
		t.Fatalf("expected request body log")
	}
	body := bodyLog["http_request_body"].(string)
	if strings.Contains(body, "secret") {
		t.Fatalf("expected password redacted, got=%s", body)
	}
	if !strings.Contains(body, logger.DefaultMask) {
		t.Fatalf("expected redacted mask in request body, got=%s", body)
	}
}

func TestGracefulDegradationWhenDetailHookPanics(t *testing.T) {
	var buf bytes.Buffer
	mw := New(
		Config{
			Logger: newTestSlog(&buf, logger.LevelDebug),
			ShouldLogDetail: func(*http.Request, int, time.Duration) bool {
				panic("boom")
			},
		},
	)
	
	handler := mw.Wrap(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
			},
		),
	)
	
	req := httptest.NewRequest(http.MethodGet, "/panic-safe", nil)
	rr := httptest.NewRecorder()
	
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("request flow must not break, got status=%d", rr.Code)
	}
	
	logs := parseLogLines(t, buf.String())
	if findByMsg(logs, "incoming http request") == nil {
		t.Fatalf("summary log should still be produced")
	}
}
