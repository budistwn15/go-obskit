// Package obskit provides the small common API for structured stdout logging
// and optional direct delivery.
package obskit

import (
	"context"
	"io"
	"log/slog"

	"github.com/budistwn15/go-obskit/v2/logger"
)

// Config controls the common stdout logger. Output defaults to os.Stdout.
type Config struct {
	Service     string
	Version     string
	Environment string
	Level       logger.Level
	Output      io.Writer
	AddSource   bool
}

// TraceContext contains vendor-neutral IDs used to correlate logs with APM data.
type TraceContext struct {
	TraceID       string
	SpanID        string
	TransactionID string
}

// Delivery is an optional direct log destination with explicit shutdown.
type Delivery interface {
	LoggerMiddleware() logger.HandlerMiddleware
	Close(context.Context) error
}

// Runtime owns a logger configured with optional direct delivery.
type Runtime struct {
	log      *slog.Logger
	delivery Delivery
}

// New returns a JSON stdout logger without background goroutines or network I/O.
func New(cfg Config) *slog.Logger {
	return newLogger(cfg, nil)
}

// Open returns a logger runtime that owns delivery shutdown.
func Open(cfg Config, delivery Delivery) *Runtime {
	return &Runtime{log: newLogger(cfg, delivery), delivery: delivery}
}

// WithTrace adds vendor-neutral APM correlation IDs to a context.
func WithTrace(ctx context.Context, trace TraceContext) context.Context {
	meta, _ := logger.Meta(ctx)
	meta.TraceID = trace.TraceID
	meta.SpanID = trace.SpanID
	meta.TransactionID = trace.TransactionID
	return logger.WithMeta(ctx, meta)
}

// Logger returns the runtime logger.
func (r *Runtime) Logger() *slog.Logger {
	if r == nil || r.log == nil {
		return slog.Default()
	}
	return r.log
}

// Close flushes and stops direct delivery. Stdout-only runtimes return nil.
func (r *Runtime) Close(ctx context.Context) error {
	if r == nil || r.delivery == nil {
		return nil
	}
	return r.delivery.Close(ctx)
}

func newLogger(cfg Config, delivery Delivery) *slog.Logger {
	var middlewares []logger.HandlerMiddleware
	if delivery != nil {
		middlewares = []logger.HandlerMiddleware{delivery.LoggerMiddleware()}
	}
	return logger.New(logger.Config{
		ServiceName:    cfg.Service,
		ServiceVersion: cfg.Version,
		Environment:    cfg.Environment,
		SchemaVersion:  "2",
		Level:          cfg.Level,
		Format:         logger.FormatJSON,
		Output:         cfg.Output,
		AddSource:      cfg.AddSource,
		Middlewares:    middlewares,
	})
}
