package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"testing"
	"time"
)

func BenchmarkLoggerInfo(b *testing.B) {
	l := New(Config{
		ServiceName: "bench",
		Environment: "production",
		Level:       LevelInfo,
		Output:      io.Discard,
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("ping")
	}
}

func BenchmarkLoggerOutput(b *testing.B) {
	b.Run("discard", func(b *testing.B) {
		benchmarkLoggerOutput(b, io.Discard)
	})
	b.Run("dev-null-file", func(b *testing.B) {
		file, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			b.Fatal(err)
		}
		defer file.Close()
		benchmarkLoggerOutput(b, file)
	})
	b.Run("slow-writer", func(b *testing.B) {
		benchmarkLoggerOutput(b, slowWriter{delay: 100 * time.Microsecond})
	})
}

func benchmarkLoggerOutput(b *testing.B, output io.Writer) {
	l := New(Config{ServiceName: "bench", Environment: "production", Output: output})
	b.ResetTimer()
	for b.Loop() {
		l.Info("ping")
	}
}

type slowWriter struct {
	delay time.Duration
}

func (w slowWriter) Write(p []byte) (int, error) {
	time.Sleep(w.delay)
	return len(p), nil
}

func BenchmarkLoggerInfoContextMeta(b *testing.B) {
	l := New(Config{
		ServiceName: "bench",
		Environment: "production",
		Level:       LevelInfo,
		Output:      io.Discard,
	})
	ctx := WithMeta(context.Background(), ContextMeta{
		CorrelationID: "corr-1",
		RequestID:     "req-1",
		Layer:         "handler",
		Component:     "orders",
		Operation:     "create",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.InfoContext(ctx, "done", slog.Int("n", i))
	}
}

func BenchmarkLoggerWithFields(b *testing.B) {
	l := New(Config{
		ServiceName: "bench",
		Environment: "production",
		Level:       LevelInfo,
		Output:      io.Discard,
	})
	l = With(l, slog.String("app.region", "ap-southeast-1"), slog.String("app.module", "billing"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("enriched")
	}
}

func BenchmarkLoggerInfoAttrs(b *testing.B) {
	for _, count := range []int{1, 2, 10} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			l := New(Config{ServiceName: "bench", Environment: "production", Output: io.Discard})
			attrs := make([]slog.Attr, count)
			for i := range attrs {
				attrs[i] = slog.Int("value", i)
			}
			b.ResetTimer()
			for b.Loop() {
				l.LogAttrs(context.Background(), slog.LevelInfo, "attrs", attrs...)
			}
		})
	}
}

func BenchmarkLoggerDisabled(b *testing.B) {
	l := New(Config{ServiceName: "bench", Environment: "production", Level: LevelError, Output: io.Discard})
	for b.Loop() {
		l.Info("disabled")
	}
}

func BenchmarkLoggerError(b *testing.B) {
	l := New(Config{ServiceName: "bench", Environment: "production", Output: io.Discard})
	for b.Loop() {
		l.Error("failed", slog.String("error.type", "test"))
	}
}

func BenchmarkLoggerInfoTwoContextAttrs(b *testing.B) {
	benchmarkLoggerInfoContextAttrs(b, 2)
}

func BenchmarkLoggerInfoOneContextAttr(b *testing.B) {
	benchmarkLoggerInfoContextAttrs(b, 1)
}

func BenchmarkLoggerInfoTenContextAttrs(b *testing.B) {
	benchmarkLoggerInfoContextAttrs(b, 10)
}

func benchmarkLoggerInfoContextAttrs(b *testing.B, count int) {
	l := New(Config{
		ServiceName: "bench",
		Environment: "production",
		Level:       LevelInfo,
		Output:      io.Discard,
	})
	attrs := make([]slog.Attr, count)
	for i := range attrs {
		attrs[i] = slog.Int("context_attr", i)
	}
	ctx := WithContextAttrs(context.Background(), attrs...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.InfoContext(ctx, "ctx attrs", slog.Int("n", i))
	}
}
