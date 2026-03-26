package logger

import (
	"context"
	"io"
	"log/slog"
	"testing"
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

func BenchmarkLoggerWithCommon(b *testing.B) {
	l := New(Config{
		ServiceName: "bench",
		Environment: "production",
		Level:       LevelInfo,
		Output:      io.Discard,
	})
	l = WithCommon(l, slog.String("app.region", "ap-southeast-1"), slog.String("app.module", "billing"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Info("enriched")
	}
}

func BenchmarkLoggerInfoContextAttrs(b *testing.B) {
	l := New(Config{
		ServiceName: "bench",
		Environment: "production",
		Level:       LevelInfo,
		Output:      io.Discard,
	})
	ctx := WithContextAttrs(
		context.Background(),
		slog.String("tenant_id", "t-1"),
		slog.String("region", "ap-southeast-1"),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.InfoContext(ctx, "ctx attrs", slog.Int("n", i))
	}
}
