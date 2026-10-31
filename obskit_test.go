package obskit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	obskit "github.com/budistwn15/go-obskit"
	"github.com/budistwn15/go-obskit/elastic"
	"github.com/budistwn15/go-obskit/httpsink"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/budistwn15/go-obskit/lokisink"
)

var (
	_ obskit.Delivery = (*elastic.Middleware)(nil)
	_ obskit.Delivery = (*httpsink.Middleware)(nil)
	_ obskit.Delivery = (*lokisink.Middleware)(nil)
)

func TestNewWritesJSONStdoutSchema(t *testing.T) {
	var output bytes.Buffer
	log := obskit.New(obskit.Config{
		Service:     "billing-api",
		Environment: "production",
		Output:      &output,
	})
	log.Info("ready")

	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode JSON log: %v", err)
	}
	if event[logger.FieldServiceName] != "billing-api" || event["msg"] != "ready" {
		t.Fatalf("unexpected event: %v", event)
	}
	if event[logger.FieldSchemaVersion] != "2" {
		t.Fatalf("schema version = %v, want 2", event[logger.FieldSchemaVersion])
	}
}

func BenchmarkFacadeInfo(b *testing.B) {
	log := obskit.New(obskit.Config{Output: io.Discard})
	b.ResetTimer()
	for b.Loop() {
		log.Info("ready")
	}
}

func TestOpenOwnsDeliveryClose(t *testing.T) {
	delivery := &fakeDelivery{}
	runtime := obskit.Open(obskit.Config{Output: &bytes.Buffer{}}, delivery)
	runtime.Logger().Info("ready")
	if delivery.records != 1 {
		t.Fatalf("delivered records = %d, want 1", delivery.records)
	}
	if err := runtime.Close(context.Background()); err != nil {
		t.Fatalf("close runtime: %v", err)
	}
	if !delivery.closed {
		t.Fatal("delivery was not closed")
	}
}

func TestWithTraceAddsAPMCorrelation(t *testing.T) {
	var output bytes.Buffer
	log := obskit.New(obskit.Config{Output: &output})
	ctx := obskit.WithTrace(context.Background(), obskit.TraceContext{
		TraceID:       "trace-1",
		SpanID:        "span-1",
		TransactionID: "transaction-1",
	})
	log.InfoContext(ctx, "correlated")

	var event map[string]any
	if err := json.Unmarshal(output.Bytes(), &event); err != nil {
		t.Fatalf("decode JSON log: %v", err)
	}
	if event[logger.FieldTraceID] != "trace-1" || event[logger.FieldSpanID] != "span-1" ||
		event[logger.FieldTransactionID] != "transaction-1" {
		t.Fatalf("missing APM correlation: %v", event)
	}
}

type fakeDelivery struct {
	records int
	closed  bool
}

func (d *fakeDelivery) LoggerMiddleware() logger.HandlerMiddleware {
	return func(next slog.Handler) slog.Handler {
		return deliveryHandler{next: next, delivery: d}
	}
}

func (d *fakeDelivery) Close(context.Context) error {
	d.closed = true
	return nil
}

type deliveryHandler struct {
	next     slog.Handler
	delivery *fakeDelivery
}

func (h deliveryHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h deliveryHandler) Handle(ctx context.Context, record slog.Record) error {
	h.delivery.records++
	return h.next.Handle(ctx, record)
}

func (h deliveryHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return deliveryHandler{next: h.next.WithAttrs(attrs), delivery: h.delivery}
}

func (h deliveryHandler) WithGroup(name string) slog.Handler {
	return deliveryHandler{next: h.next.WithGroup(name), delivery: h.delivery}
}
