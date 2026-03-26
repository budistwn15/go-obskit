package logger

import (
	"context"
	"log/slog"

	"github.com/budistwn15/go-obskit/correlation"
)

type contextKey string

const (
	loggerKey contextKey = "go-obskit/logger"
	metaKey   contextKey = "go-obskit/context-meta"
	attrsKey  contextKey = "go-obskit/context-attrs"
)

func WithLogger(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey, l)
}

// WithContext is an alias of WithLogger for ergonomics in application code.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return WithLogger(ctx, l)
}

func FromContext(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if ctx != nil {
		if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
			return l
		}
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}

func WithMeta(ctx context.Context, meta ContextMeta) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, metaKey, meta)
}

func Meta(ctx context.Context) (ContextMeta, bool) {
	if ctx == nil {
		return ContextMeta{}, false
	}
	meta, ok := ctx.Value(metaKey).(ContextMeta)
	return meta, ok
}

// WithContextAttrs stores structured attrs in context for log enrichment.
// Existing attrs are replaced.
func WithContextAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(attrs) == 0 {
		return ctx
	}
	cp := make([]slog.Attr, len(attrs))
	copy(cp, attrs)
	return context.WithValue(ctx, attrsKey, cp)
}

// AppendContextAttrs appends attrs to existing context attrs.
func AppendContextAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(attrs) == 0 {
		return ctx
	}
	existing, _ := contextAttrsNoCopy(ctx)
	out := make([]slog.Attr, 0, len(existing)+len(attrs))
	out = append(out, existing...)
	out = append(out, attrs...)
	return context.WithValue(ctx, attrsKey, out)
}

// ContextAttrs retrieves context attrs, if any.
func ContextAttrs(ctx context.Context) ([]slog.Attr, bool) {
	attrs, ok := contextAttrsNoCopy(ctx)
	if !ok {
		return nil, false
	}
	cp := make([]slog.Attr, len(attrs))
	copy(cp, attrs)
	return cp, true
}

func contextAttrsNoCopy(ctx context.Context) ([]slog.Attr, bool) {
	if ctx == nil {
		return nil, false
	}
	attrs, ok := ctx.Value(attrsKey).([]slog.Attr)
	if !ok || len(attrs) == 0 {
		return nil, false
	}
	return attrs, true
}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	ctx = correlation.WithID(ctx, id)
	meta, _ := Meta(ctx)
	meta.CorrelationID = id
	return WithMeta(ctx, meta)
}

func CorrelationID(ctx context.Context) string {
	if id := correlation.ID(ctx); id != "" {
		return id
	}
	meta, ok := Meta(ctx)
	if !ok {
		return ""
	}
	return meta.CorrelationID
}
