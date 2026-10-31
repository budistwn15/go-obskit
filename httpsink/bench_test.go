package httpsink

import (
	"log/slog"
	"testing"
	"time"
)

func BenchmarkEnqueueAvailable(b *testing.B) {
	m := benchmarkMiddleware(1)
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "bench", 0)
	for b.Loop() {
		m.enqueue(rec)
		<-m.queue
	}
}

func BenchmarkEnqueueFull(b *testing.B) {
	m := benchmarkMiddleware(1)
	m.queue <- slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "full", 0)
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "bench", 0)
	for b.Loop() {
		m.enqueue(rec)
	}
}

func BenchmarkMarshalBatchNDJSON(b *testing.B) {
	records := make([]slog.Record, 100)
	for i := range records {
		records[i] = slog.NewRecord(time.Unix(0, int64(i)), slog.LevelInfo, "bench", 0)
		records[i].AddAttrs(slog.Int("index", i))
	}
	for b.Loop() {
		if _, err := marshalBatch(FormatNDJSON, records, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkMiddleware(queueSize int) *Middleware {
	return &Middleware{
		cfg:   Config{Enabled: true, Endpoint: "http://example.invalid"},
		queue: make(chan slog.Record, queueSize),
	}
}
