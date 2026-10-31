package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	obskit "github.com/budistwn15/go-obskit"
	"github.com/budistwn15/go-obskit/logger"
	"go.opentelemetry.io/otel/trace"
)

func TestWithTraceCopiesActiveSpanIDs(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	}))

	var output bytes.Buffer
	obskit.New(obskit.Config{Output: &output}).InfoContext(WithTrace(ctx), "correlated")
	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event[logger.FieldTraceID] != traceID.String() || event[logger.FieldSpanID] != spanID.String() {
		t.Fatalf("missing trace correlation: %v", event)
	}
}

func TestWithTraceLeavesContextWithoutActiveSpanUnchanged(t *testing.T) {
	ctx := context.Background()
	if got := WithTrace(ctx); got != ctx {
		t.Fatal("context without active span must remain unchanged")
	}
}

func TestWithTraceNilContextReturnsUsableContext(t *testing.T) {
	if got := WithTrace(nil); got == nil {
		t.Fatal("nil context must not propagate")
	}
}

func BenchmarkWithTraceNoActiveSpan(b *testing.B) {
	ctx := context.Background()
	for b.Loop() {
		_ = WithTrace(ctx)
	}
}
