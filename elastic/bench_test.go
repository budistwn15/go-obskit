package elastic

import (
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/v2/logger"
)

func BenchmarkLoggerWithElasticDisabled(b *testing.B) {
	mw := NewMiddleware(DefaultConfig())
	log := logger.New(logger.Config{
		ServiceName: "svc",
		Environment: "production",
		Output:      io.Discard,
		Middlewares: []logger.HandlerMiddleware{mw.LoggerMiddleware()},
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info("bench", "i", i)
	}
}

func BenchmarkRecordToDocumentPlain(b *testing.B) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "bench", 0)
	rec.AddAttrs(slog.String("service", "api"), slog.Int("status", 200))
	for b.Loop() {
		_ = recordToDocument(rec, nil)
	}
}

func BenchmarkRecordToDocumentJSON4KB(b *testing.B) {
	benchmarkRecordToDocumentJSON(b, 4*1024)
}

func BenchmarkRecordToDocumentJSON64KB(b *testing.B) {
	benchmarkRecordToDocumentJSON(b, 64*1024)
}

func benchmarkRecordToDocumentJSON(b *testing.B, size int) {
	payload := `{"data":"` + strings.Repeat("x", size-len(`{"data":""}`)) + `"}`
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "bench", 0)
	rec.AddAttrs(slog.String("http.request.body", payload))
	b.ResetTimer()
	for b.Loop() {
		_ = recordToDocument(rec, nil)
	}
}
