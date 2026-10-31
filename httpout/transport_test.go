package httpout

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestLogger(buf *bytes.Buffer, level logger.Level) *slog.Logger {
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
			t.Fatalf("invalid json line: %v line=%s", err, line)
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

func TestCorrelationPropagation(t *testing.T) {
	var buf bytes.Buffer
	var capturedHeader string

	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			capturedHeader = req.Header.Get("X-Correlation-ID")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)

	tr := New(
		rt, Config{
			Logger: newTestLogger(&buf, logger.LevelInfo),
		},
	)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	ctx := logger.WithCorrelationID(context.Background(), "corr-xyz")
	req = req.WithContext(ctx)

	_, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedHeader != "corr-xyz" {
		t.Fatalf("expected propagated correlation header, got=%s", capturedHeader)
	}
	if got := req.Header.Get("X-Correlation-ID"); got != "" {
		t.Fatalf("caller request mutated with correlation header %q", got)
	}
}

func TestDurationAndSummaryLogging(t *testing.T) {
	var buf bytes.Buffer
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			time.Sleep(10 * time.Millisecond)
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)
	tr := New(
		rt, Config{
			Logger: newTestLogger(&buf, logger.LevelInfo),
		},
	)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/orders?q=1", nil)
	_, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := parseLogLines(t, buf.String())
	summary := findByMsg(logs, "outbound http request")
	if summary == nil {
		t.Fatalf("missing summary log")
	}
	if int(summary["http_status_code"].(float64)) != http.StatusAccepted {
		t.Fatalf("unexpected status: %v", summary["http_status_code"])
	}
	if summary["duration_ms"].(float64) <= 0 {
		t.Fatalf("duration_ms should be > 0")
	}
}

func TestRedactionAndTruncation(t *testing.T) {
	var buf bytes.Buffer
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			_, _ = io.ReadAll(req.Body)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"token":"1234567890","ok":true}`)),
			}, nil
		},
	)
	tr := New(
		rt, Config{
			Logger:              newTestLogger(&buf, logger.LevelDebug),
			CaptureRequestBody:  true,
			CaptureResponseBody: true,
			LogRequestBody:      true,
			LogResponseBody:     true,
			MaxBodyCaptureBytes: 10,
			ShouldLogDetail: func(*http.Request, *http.Response, error, time.Duration) bool {
				return true
			},
		},
	)

	req, _ := http.NewRequest(
		http.MethodPost, "http://example.com/login", strings.NewReader(`{"username":"john","password":"secret"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	logs := parseLogLines(t, buf.String())
	reqBody := findByMsg(logs, "outbound request body")
	if reqBody == nil {
		t.Fatalf("missing request body log")
	}
	reqLogged := reqBody["http_request_body"].(string)
	if strings.Contains(reqLogged, "secret") {
		t.Fatalf("expected request body to be redacted")
	}

	respBody := findByMsg(logs, "outbound response body")
	if respBody == nil {
		t.Fatalf("missing response body log")
	}
	if respBody["http_response_body_truncated"] != true {
		t.Fatalf("expected truncated response body")
	}
}

func TestGracefulFallbackOnBodyReadError(t *testing.T) {
	var buf bytes.Buffer
	brokenReader := io.NopCloser(errorReader{err: errors.New("read failed")})

	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       brokenReader,
			}, nil
		},
	)

	tr := New(
		rt, Config{
			Logger:              newTestLogger(&buf, logger.LevelDebug),
			CaptureResponseBody: true,
			LogResponseBody:     true,
			ShouldLogDetail: func(*http.Request, *http.Response, error, time.Duration) bool {
				panic("detail hook panic")
			},
		},
	)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/resource", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("request flow should not break")
	}

	logs := parseLogLines(t, buf.String())
	if findByMsg(logs, "outbound http request") == nil {
		t.Fatalf("summary log should remain available")
	}
}

func TestResponseCapturePreservesDataReturnedWithError(t *testing.T) {
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(&partialErrorReader{data: []byte(`{"ok":true}`)}),
		}, nil
	})
	tr := New(rt, Config{
		CaptureResponseBody: true,
		LogResponseBody:     true,
		ShouldLogDetail:     func(*http.Request, *http.Response, error, time.Duration) bool { return true },
	})
	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)

	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if got := string(body); got != `{"ok":true}` {
		t.Fatalf("caller response body = %q, want complete original body", got)
	}
}

func TestRequestCapturePreservesDataReturnedWithError(t *testing.T) {
	received := ""
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		received = string(body)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
	})
	tr := New(rt, Config{CaptureRequestBody: true})
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", nil)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(&partialErrorReader{data: []byte(`{"name":"john"}`)})

	_, _ = tr.RoundTrip(req)

	if received != `{"name":"john"}` {
		t.Fatalf("transport body = %q, want complete original body", received)
	}
}

func TestRequestRedactionFailureDoesNotLogOriginalContent(t *testing.T) {
	var buf bytes.Buffer
	rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: http.NoBody}, nil
	})
	tr := New(rt, Config{
		Logger:             newTestLogger(&buf, logger.LevelDebug),
		CaptureRequestBody: true,
		LogRequestBody:     true,
		ShouldLogDetail:    func(*http.Request, *http.Response, error, time.Duration) bool { return true },
	})
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", strings.NewReader(`{"password":"secret-value"`))
	req.Header.Set("Content-Type", "application/json")

	_, _ = tr.RoundTrip(req)

	if strings.Contains(buf.String(), "secret-value") {
		t.Fatalf("redaction failure leaked original secret: %s", buf.String())
	}
}

func TestOutboundErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		},
	)
	tr := New(
		rt, Config{
			Logger: newTestLogger(&buf, logger.LevelInfo),
			Layer:  logger.LayerContract,
		},
	)

	req, _ := http.NewRequest(http.MethodGet, "http://example.com/downstream", nil)
	_, err := tr.RoundTrip(req)
	if err == nil {
		t.Fatalf("expected outbound error")
	}

	logs := parseLogLines(t, buf.String())
	summary := findByMsg(logs, "outbound http request")
	if summary == nil {
		t.Fatalf("missing summary log")
	}
	if summary["error_kind"] != "context_deadline_exceeded" {
		t.Fatalf("unexpected error_kind: %v", summary["error_kind"])
	}
}

func TestWrapClientDoesNotMutateCaller(t *testing.T) {
	base := roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, nil })
	client := &http.Client{Transport: base, Timeout: time.Second}
	wrapped := WrapClient(client, Config{})
	if wrapped == client {
		t.Fatal("WrapClient must return a shallow copy")
	}
	if _, ok := client.Transport.(roundTripFunc); !ok {
		t.Fatal("caller transport was mutated")
	}
	if wrapped.Timeout != client.Timeout {
		t.Fatalf("timeout = %v, want %v", wrapped.Timeout, client.Timeout)
	}
}

type errorReader struct {
	err error
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

func (r errorReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
