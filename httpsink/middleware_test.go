package httpsink

import (
	"bytes"
	"context"
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

func TestCloseCancelsInFlightRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	var calls atomic.Int32
	client := &http.Client{Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(requestStarted)
			<-r.Context().Done()
			return nil, r.Context().Err()
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})}

	mw := NewMiddleware(Config{
		Enabled:               true,
		Endpoint:              "http://example.invalid",
		BatchSize:             1,
		Timeout:               time.Minute,
		HTTPClient:            client,
		ConnectionLogToStdout: false,
	})
	mw.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "in flight", 0))
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := mw.Close(ctx); err != nil {
		t.Fatalf("close did not cancel request: %v", err)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestConfigOwnership(t *testing.T) {
	client := &http.Client{}
	headers := map[string]string{"X-Owner": "caller"}
	fields := map[string]any{"owner": "caller"}
	mw := NewMiddleware(Config{
		Enabled:      true,
		Endpoint:     "http://example.invalid",
		HTTPClient:   client,
		Headers:      headers,
		StaticFields: fields,
	})
	headers["X-Owner"] = "mutated"
	fields["owner"] = "mutated"
	if client.Timeout != 0 {
		t.Fatalf("caller client mutated: timeout=%v", client.Timeout)
	}
	if mw.cfg.Headers["X-Owner"] != "caller" || mw.cfg.StaticFields["owner"] != "caller" {
		t.Fatalf("config maps were not copied: headers=%v fields=%v", mw.cfg.Headers, mw.cfg.StaticFields)
	}
	if err := mw.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestPermanentClientErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	mw := NewMiddleware(Config{
		Enabled:               true,
		Endpoint:              ts.URL,
		BatchSize:             1,
		MaxRetries:            3,
		RetryBackoff:          time.Millisecond,
		ConnectionLogToStdout: false,
	})
	mw.enqueue(slog.NewRecord(time.Now(), slog.LevelInfo, "bad", 0))
	if err := mw.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("permanent 4xx calls = %d, want 1", got)
	}
	if got := mw.Stats().Retried; got != 0 {
		t.Fatalf("permanent 4xx retries = %d, want 0", got)
	}
}

func TestEnqueueAllocationBudget(t *testing.T) {
	mw := benchmarkMiddleware(1)
	record := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "budget", 0)
	if got := testing.AllocsPerRun(1000, func() {
		mw.enqueue(record)
		<-mw.queue
	}); got != 0 {
		t.Fatalf("enqueue allocations = %.2f, want 0", got)
	}
}

func TestEnqueueSnapshotsMutableAttrs(t *testing.T) {
	mw := benchmarkMiddleware(1)
	payload := map[string]any{"state": "before"}
	record := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "snapshot", 0)
	record.AddAttrs(slog.Any("payload", payload))
	mw.enqueue(record)
	payload["state"] = "after"

	frozen := <-mw.queue
	raw, err := marshalBatch(FormatNDJSON, []slog.Record{frozen}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "after") || !strings.Contains(string(raw), "before") {
		t.Fatalf("mutable attr was not snapshotted: %s", raw)
	}
}

func TestEnqueueDropsOversizedRecord(t *testing.T) {
	mw := benchmarkMiddleware(1)
	mw.cfg.MaxRecordBytes = 128
	mw.enqueue(slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, strings.Repeat("x", 128), 0))
	stats := mw.Stats()
	if stats.Oversized != 1 || stats.Dropped != 1 || len(mw.queue) != 0 {
		t.Fatalf("oversized record stats = %+v queue=%d", stats, len(mw.queue))
	}
}

func TestShipRejectsOversizedBatch(t *testing.T) {
	mw := &Middleware{cfg: Config{Format: FormatNDJSON, MaxBatchBytes: 32}}
	record := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "too large", 0)
	if err := mw.ship(context.Background(), []slog.Record{record}); err == nil {
		t.Fatal("expected oversized batch error")
	}
}
