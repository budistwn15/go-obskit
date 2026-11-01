package obskit_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/v2/httpsink"
	"github.com/budistwn15/go-obskit/v2/logger"
)

func TestSoakStdout(t *testing.T) {
	duration := soakDuration(t)
	log := logger.New(logger.Config{Output: io.Discard})
	runSoak(t, duration, 2, func(i int) {
		log.LogAttrs(context.Background(), slog.LevelInfo, "soak", slog.Int("index", i))
	})
}

func TestSoakDirectHTTP(t *testing.T) {
	duration := soakDuration(t)
	runtime.GC()
	baselineGoroutines := runtime.NumGoroutine()
	var requests atomic.Uint64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		switch requests.Add(1) % 4 {
		case 2:
			time.Sleep(5 * time.Millisecond)
		case 3:
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	sink := httpsink.NewMiddleware(httpsink.Config{
		Enabled:               true,
		Endpoint:              server.URL,
		QueueSize:             1024,
		BatchSize:             100,
		FlushInterval:         10 * time.Millisecond,
		ConnectionLogToStdout: false,
	})
	log := logger.New(logger.Config{Output: io.Discard, Middlewares: []logger.HandlerMiddleware{sink.LoggerMiddleware()}})
	runSoak(t, duration, 8, func(i int) {
		log.LogAttrs(context.Background(), slog.LevelInfo, "soak", slog.Int("index", i))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sink.Close(ctx); err != nil {
		t.Fatalf("close sink: %v", err)
	}
	server.Close()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	var afterClose runtime.MemStats
	runtime.ReadMemStats(&afterClose)
	if got := runtime.NumGoroutine(); got > baselineGoroutines+2 {
		t.Fatalf("goroutines after close=%d, baseline=%d", got, baselineGoroutines)
	}
	t.Logf("sink stats: %+v", sink.Stats())
	t.Logf("post-close goroutines=%d heap=%d", runtime.NumGoroutine(), afterClose.HeapAlloc)
}

func soakDuration(t *testing.T) time.Duration {
	t.Helper()
	raw := os.Getenv("OBSKIT_SOAK_DURATION")
	if raw == "" {
		t.Skip("set OBSKIT_SOAK_DURATION, for example 60s")
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		t.Fatalf("invalid OBSKIT_SOAK_DURATION %q", raw)
	}
	return duration
}

func runSoak(t *testing.T, duration time.Duration, maxGoroutineGrowth int, logRecord func(int)) {
	t.Helper()
	runtime.GC()
	beforeGoroutines := runtime.NumGoroutine()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	deadline := time.Now().Add(duration)
	count := 0
	for time.Now().Before(deadline) {
		logRecord(count)
		count++
	}

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if got := runtime.NumGoroutine(); got > beforeGoroutines+maxGoroutineGrowth {
		t.Fatalf("goroutines grew during soak: %d -> %d", beforeGoroutines, got)
	}
	const maxLiveHeapGrowth = 16 << 20
	if after.HeapAlloc > before.HeapAlloc+maxLiveHeapGrowth {
		t.Fatalf("live heap grew beyond budget: %d -> %d", before.HeapAlloc, after.HeapAlloc)
	}
	t.Logf(
		"records=%d goroutines=%d->%d heap=%d->%d",
		count, beforeGoroutines, runtime.NumGoroutine(), before.HeapAlloc, after.HeapAlloc,
	)
}
