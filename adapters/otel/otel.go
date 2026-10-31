// Package otel adds active OpenTelemetry trace correlation to obskit log contexts.
package otel

import (
	"context"

	obskit "github.com/budistwn15/go-obskit"
	"go.opentelemetry.io/otel/trace"
)

// WithTrace copies the active OpenTelemetry trace and span IDs into obskit metadata.
func WithTrace(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	span := trace.SpanContextFromContext(ctx)
	if !span.IsValid() {
		return ctx
	}
	return obskit.WithTrace(ctx, obskit.TraceContext{
		TraceID: span.TraceID().String(),
		SpanID:  span.SpanID().String(),
	})
}
