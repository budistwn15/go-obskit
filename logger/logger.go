package logger

import (
	"context"
	"log/slog"
	"os"
)

func New(cfg Config) *slog.Logger {
	cfg = normalizeConfig(cfg)

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

	host := ""
	if h, err := os.Hostname(); err == nil {
		host = h
	}
	instanceID := cfg.InstanceID
	if instanceID == "" {
		instanceID = os.Getenv("HOSTNAME")
	}

	l := slog.New(newContextHandler(newSafeHandler(baseHandler))).With(
		slog.String(FieldServiceName, cfg.ServiceName),
		slog.String(FieldServiceVersion, cfg.ServiceVersion),
		slog.String(FieldEnvironment, cfg.Environment),
		slog.String(FieldHost, host),
		slog.String(FieldInstanceID, instanceID),
	)

	return l
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

// WithCommon is an alias of With for readable shared-field enrichment.
func WithCommon(l *slog.Logger, attrs ...slog.Attr) *slog.Logger {
	return With(l, attrs...)
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
			record.AddAttrs(slog.Int64(FieldDurationMS, meta.DurationMS))
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

func (h *safeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	defer func() { _ = recover() }()
	return &safeHandler{next: h.next.WithAttrs(attrs)}
}

func (h *safeHandler) WithGroup(name string) slog.Handler {
	defer func() { _ = recover() }()
	return &safeHandler{next: h.next.WithGroup(name)}
}
