package outbound

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/budistwn15/go-obskit/logger"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func parseLogs(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid log line: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestCorrelationPropagationAndDuration(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	var gotCorr string
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			gotCorr = req.Header.Get("X-Correlation-ID")
			time.Sleep(5 * time.Millisecond)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)
	tr := NewTransport(rt, log, DefaultOptions())
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	req = req.WithContext(correlation.WithID(context.Background(), "corr-1"))
	
	_, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotCorr != "corr-1" {
		t.Fatalf("correlation header not propagated")
	}
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if last["duration_ms"] == nil {
		t.Fatalf("duration_ms must be logged")
	}
}

func TestErrorClassification(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		},
	)
	tr := NewTransport(rt, log, DefaultOptions())
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/timeout", nil)
	_, _ = tr.RoundTrip(req)
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if last["error.kind"] != "context_deadline_exceeded" {
		t.Fatalf("unexpected error kind: %v", last["error.kind"])
	}
}

func TestRequestBodyRestoreAndRedactionTruncation(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	opts := DefaultOptions()
	opts.CaptureRequestBody = true
	opts.MaxBodyBytes = 16
	opts.LogRequestStart = true
	opts.LogRequestComplete = true
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			if !strings.Contains(string(b), "password") {
				t.Fatalf("request body should be preserved for transport")
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)
	tr := NewTransport(rt, log, opts)
	req, _ := http.NewRequest(
		http.MethodPost, "http://example.com/login", strings.NewReader(`{"password":"secret-value"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	_, _ = tr.RoundTrip(req)
	logs := parseLogs(t, buf.String())
	found := false
	for _, l := range logs {
		if v, ok := l["http.request.body"].(string); ok {
			found = true
			if strings.Contains(v, "secret-value") {
				t.Fatalf("expected redacted request body")
			}
		}
	}
	if !found {
		t.Fatalf("expected request body log when enabled")
	}
}

func TestResponseBodyRestore(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	opts := DefaultOptions()
	opts.CaptureResponseBody = true
	opts.LogErrorBodies = true
	opts.MaxBodyBytes = 20
	rt := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"token":"super-secret-token","error":"boom"}`)),
			}, nil
		},
	)
	tr := NewTransport(rt, log, opts)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/fail", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != `{"token":"super-secret-token","error":"boom"}` {
		t.Fatalf("response body should be restorable")
	}
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if body, ok := last["http.response.body"].(string); !ok || body == "" {
		t.Fatalf("expected response body log")
	} else if strings.Contains(body, "super-secret-token") {
		t.Fatalf("expected redacted response body")
	}
}

func TestClassifyGenericTransportError(t *testing.T) {
	if got := classifyError(errors.New("x")); got != "transport_error" {
		t.Fatalf("unexpected classify: %s", got)
	}
}

func TestDefaultLowNoise(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	tr := NewTransport(
		roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			},
		), log, DefaultOptions(),
	)
	
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	_, _ = tr.RoundTrip(req)
	
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if _, ok := last["http.request.body"]; ok {
		t.Fatalf("request body should not be logged by default")
	}
	if _, ok := last["http.response.body"]; ok {
		t.Fatalf("response body should not be logged by default")
	}
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("panic writer") }

func TestPanicSafeLoggingDoesNotBreakFlow(t *testing.T) {
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: panicWriter{}})
	tr := NewTransport(
		roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			},
		), log, DefaultOptions(),
	)
	
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	resp, err := tr.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("flow must continue despite logging panic, resp=%v err=%v", resp, err)
	}
}

func TestDisableAllRequestLogs(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	opts := DefaultOptions()
	opts.LogRequestStart = false
	opts.LogRequestComplete = false
	opts.LogRequestError = false
	
	tr := NewTransport(
		roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			},
		), log, opts,
	)
	
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	_, _ = tr.RoundTrip(req)
	
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected no logs when all log flags are disabled")
	}
}

func TestSelectiveLoggingHooks(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	opts := DefaultOptions()
	opts.LogRequestStart = true
	opts.ShouldLogStart = func(meta httplog.DecisionMeta) bool {
		return false
	}
	opts.ShouldLogComplete = func(meta httplog.DecisionMeta) bool {
		return meta.Response.StatusCode >= 500
	}
	opts.ShouldLogError = func(meta httplog.DecisionMeta) bool {
		return true
	}
	
	tr := NewTransport(
		roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			},
		), log, opts,
	)
	
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	_, _ = tr.RoundTrip(req)
	
	logs := parseLogs(t, buf.String())
	if len(logs) != 0 {
		t.Fatalf("expected no logs because hook filtered start+complete and no error")
	}
}

func TestSlowFieldOnComplete(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	opts := DefaultOptions()
	opts.SlowThreshold = 1 * time.Millisecond
	
	tr := NewTransport(
		roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				time.Sleep(3 * time.Millisecond)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			},
		), log, opts,
	)
	
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/slow", nil)
	_, _ = tr.RoundTrip(req)
	
	logs := parseLogs(t, buf.String())
	last := logs[len(logs)-1]
	if slow, ok := last["slow"].(bool); !ok || !slow {
		t.Fatalf("expected slow=true, got=%v", last["slow"])
	}
	if _, ok := last["slow_threshold_ms"]; !ok {
		t.Fatalf("expected slow_threshold_ms in event")
	}
	if _, ok := last["threshold_ms"]; !ok {
		t.Fatalf("expected threshold_ms in event")
	}
}

func TestSuccessSamplingKeepsErrorsAndSlow(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 1000
	
	successRT := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)
	errRT := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			return nil, context.DeadlineExceeded
		},
	)
	slowRT := roundTripFunc(
		func(req *http.Request) (*http.Response, error) {
			time.Sleep(3 * time.Millisecond)
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			}, nil
		},
	)
	
	trSuccess := NewTransport(successRT, log, opts)
	req1, _ := http.NewRequest(http.MethodGet, "http://example.com/success", nil)
	_, _ = trSuccess.RoundTrip(req1)
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("sampled success log should be skipped")
	}
	
	trErr := NewTransport(errRT, log, opts)
	req2, _ := http.NewRequest(http.MethodGet, "http://example.com/error", nil)
	_, _ = trErr.RoundTrip(req2)
	logs := parseLogs(t, buf.String())
	if len(logs) == 0 || logs[len(logs)-1]["event"] != "http.outbound.error" {
		t.Fatalf("error logs must not be sampled out")
	}
	
	buf.Reset()
	opts.SlowThreshold = 1 * time.Millisecond
	trSlow := NewTransport(slowRT, log, opts)
	req3, _ := http.NewRequest(http.MethodGet, "http://example.com/slow", nil)
	_, _ = trSlow.RoundTrip(req3)
	logs = parseLogs(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("slow complete logs must not be sampled out")
	}
	last := logs[len(logs)-1]
	if slow, ok := last["slow"].(bool); !ok || !slow {
		t.Fatalf("expected slow=true on retained slow log")
	}
}
