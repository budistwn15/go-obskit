package logger

import (
	"context"
	"log/slog"
	"os"
	"time"
)

func New(cfg Config) *slog.Logger {
	cfg = normalizeConfig(cfg)

	// shortcut: slog's built-in keys preserve zero-allocation stdout; rename only with a benchmarked zero-allocation encoder.
	opts := &slog.HandlerOptions{
		Level:     ParseLevel(cfg.Level),
		AddSource: cfg.AddSource,
	}

	var baseHandler slog.Handler
	if cfg.Format == FormatText {
		baseHandler = slog.NewTextHandler(cfg.Output, opts)
	} else {
		baseHandler = slog.NewJSONHandler(cfg.Output, opts)
	}
	baseHandler = applyMiddlewares(baseHandler, cfg.Middlewares)

	host := ""
	if h, err := os.Hostname(); err == nil {
		host = h
	}
	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID = os.Getenv("HOSTNAME")
	}

	l := slog.New(newContextHandler(newSafeHandler(baseHandler)))
	attrs := make([]any, 0, 6)
	attrs = appendStringAttr(attrs, FieldServiceName, cfg.ServiceName)
	attrs = appendStringAttr(attrs, FieldServiceVersion, cfg.ServiceVersion)
	attrs = appendStringAttr(attrs, FieldEnvironment, cfg.Environment)
	attrs = appendStringAttr(attrs, FieldSchemaVersion, cfg.SchemaVersion)
	attrs = appendStringAttr(attrs, FieldHost, host)
	attrs = appendStringAttr(attrs, FieldInstanceID, instanceID)
	if len(attrs) > 0 {
		l = l.With(attrs...)
	}
	return l
}

func appendStringAttr(attrs []any, key, value string) []any {
	if value == "" {
		return attrs
	}
	return append(attrs, slog.String(key, value))
}

func applyMiddlewares(base slog.Handler, mws []HandlerMiddleware) slog.Handler {
	if base == nil || len(mws) == 0 {
		return base
	}
	h := base
	for _, mw := range mws {
		if mw == nil {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			next := mw(h)
			if next != nil {
				h = next
			}
		}()
	}
	return h
}

func With(l *slog.Logger, attrs ...slog.Attr) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	return l.With(args...)
}

type contextHandler struct {
	next slog.Handler
}

func newContextHandler(next slog.Handler) slog.Handler {
	return &contextHandler{next: next}
}

func (h *contextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *contextHandler) Handle(ctx context.Context, record slog.Record) error {
	meta, ok := Meta(ctx)
	if ok {
		if meta.CorrelationID != "" {
			record.AddAttrs(slog.String(FieldCorrelationID, meta.CorrelationID))
		}
		if meta.RequestID != "" {
			record.AddAttrs(slog.String(FieldRequestID, meta.RequestID))
		}
		if meta.TraceID != "" {
			record.AddAttrs(slog.String(FieldTraceID, meta.TraceID))
		}
		if meta.SpanID != "" {
			record.AddAttrs(slog.String(FieldSpanID, meta.SpanID))
		}
		if meta.TransactionID != "" {
			record.AddAttrs(slog.String(FieldTransactionID, meta.TransactionID))
		}
		if meta.Layer != "" {
			record.AddAttrs(slog.String(FieldLayer, meta.Layer))
		}
		if meta.Component != "" {
			record.AddAttrs(slog.String(FieldComponent, meta.Component))
		}
		if meta.Operation != "" {
			record.AddAttrs(slog.String(FieldOperation, meta.Operation))
		}
		if meta.DurationMS > 0 {
			record.AddAttrs(slog.Int64(FieldDurationMS, meta.DurationMS*int64(time.Millisecond)))
		}
	}
	if correlationID := CorrelationID(ctx); correlationID != "" && (meta.CorrelationID == "") {
		record.AddAttrs(slog.String(FieldCorrelationID, correlationID))
	}
	if attrs, ok := contextAttrsNoCopy(ctx); ok && len(attrs) > 0 {
		record.AddAttrs(attrs...)
	}
	return h.next.Handle(ctx, record)
}

func (h *contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &contextHandler{next: h.next.WithAttrs(attrs)}
}

func (h *contextHandler) WithGroup(name string) slog.Handler {
	return &contextHandler{next: h.next.WithGroup(name)}
}

type safeHandler struct {
	next slog.Handler
}

func newSafeHandler(next slog.Handler) slog.Handler {
	return &safeHandler{next: next}
}

func (h *safeHandler) Enabled(ctx context.Context, level slog.Level) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return h.next.Enabled(ctx, level)
}

func (h *safeHandler) Handle(ctx context.Context, record slog.Record) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()
	return h.next.Handle(ctx, record)
}

func (h *safeHandler) WithAttrs(attrs []slog.Attr) (out slog.Handler) {
	out = h
	if h == nil || h.next == nil {
		return out
	}
	defer func() { _ = recover() }()
	if next := h.next.WithAttrs(attrs); next != nil {
		out = &safeHandler{next: next}
	}
	return out
}

func (h *safeHandler) WithGroup(name string) (out slog.Handler) {
	out = h
	if h == nil || h.next == nil {
		return out
	}
	defer func() { _ = recover() }()
	if next := h.next.WithGroup(name); next != nil {
		out = &safeHandler{next: next}
	}
	return out
}
