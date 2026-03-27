package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestNew_DefaultTextForLocal(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		ServiceName: "api",
		Environment: "local",
		Output:      &buf,
	})
	l.Info("hello")
	got := buf.String()
	if !strings.Contains(got, "level=INFO") {
		t.Fatalf("expected text logs, got=%s", got)
	}
}

func TestNew_DefaultJSONForProd(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		ServiceName: "api",
		Environment: "production",
		Output:      &buf,
	})
	l.Info("hello")
	line := strings.TrimSpace(buf.String())
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m[FieldServiceName] != "api" {
		t.Fatalf("expected service_name=api got=%v", m[FieldServiceName])
	}
	if m[FieldSchemaVersion] != DefaultSchemaVersion {
		t.Fatalf("expected schema.version=%s got=%v", DefaultSchemaVersion, m[FieldSchemaVersion])
	}
}

func TestNew_CustomSchemaVersion(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		ServiceName:   "api",
		Environment:   "production",
		SchemaVersion: "2",
		Output:        &buf,
	})
	l.Info("hello")
	line := strings.TrimSpace(buf.String())
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m[FieldSchemaVersion] != "2" {
		t.Fatalf("expected custom schema.version, got=%v", m[FieldSchemaVersion])
	}
}

func TestLevelParsing(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Environment: "local",
		Level:       LevelWarn,
		Output:      &buf,
	})
	l.Info("skip")
	l.Warn("show")
	out := buf.String()
	if strings.Contains(out, "skip") {
		t.Fatalf("info should be filtered: %s", out)
	}
	if !strings.Contains(out, "show") {
		t.Fatalf("warn should be logged: %s", out)
	}
}

func TestContextMetaInjection(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Environment: "production",
		Output:      &buf,
	})

	ctx := WithMeta(context.Background(), ContextMeta{
		CorrelationID: "corr-1",
		RequestID:     "req-1",
		TraceID:       "trace-1",
		SpanID:        "span-1",
		Layer:         "handler",
		Component:     "orders",
		Operation:     "create",
		DurationMS:    10,
	})
	l.InfoContext(ctx, "done")

	line := strings.TrimSpace(buf.String())
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m[FieldCorrelationID] != "corr-1" || m[FieldLayer] != "handler" {
		t.Fatalf("expected context meta fields, got=%v", m)
	}
}

type panicWriter struct{}

func (panicWriter) Write([]byte) (int, error) { panic("writer panic") }

func TestSafeHandlerNoPanic(t *testing.T) {
	l := New(Config{
		Environment: "local",
		Output:      panicWriter{},
		Level:       LevelDebug,
	})
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("logger must not panic: %v", r)
		}
	}()
	l.Log(context.Background(), slog.LevelInfo, "no panic")
}

func TestConcurrentUse(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Environment: "production",
		Output:      &buf,
		Level:       LevelInfo,
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ctx := WithMeta(context.Background(), ContextMeta{
				RequestID: "r",
				Layer:     "handler",
			})
			l.InfoContext(ctx, "log", slog.Int("i", i))
		}(i)
	}
	wg.Wait()
}

func TestCustomFieldEnrichment(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Environment: "production",
		Output:      &buf,
		Level:       LevelInfo,
	})
	l2 := With(l, slog.String("app.team", "platform"), slog.String("app.service", "billing"))
	l2.Info("enriched")

	line := strings.TrimSpace(buf.String())
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m["app.team"] != "platform" || m["app.service"] != "billing" {
		t.Fatalf("custom fields should be present, got=%v", m)
	}
}

func TestContextAttrsInjection(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Environment: "production",
		Output:      &buf,
	})
	ctx := WithContextAttrs(
		context.Background(),
		slog.String("tenant_id", "t-42"),
		slog.String("feature_flag", "new_checkout"),
	)
	l.InfoContext(ctx, "ctx attrs")

	line := strings.TrimSpace(buf.String())
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m["tenant_id"] != "t-42" || m["feature_flag"] != "new_checkout" {
		t.Fatalf("expected context attrs in log, got=%v", m)
	}
}

func TestAppendContextAttrs(t *testing.T) {
	var buf bytes.Buffer
	l := New(Config{
		Environment: "production",
		Output:      &buf,
	})
	ctx := WithContextAttrs(context.Background(), slog.String("tenant_id", "t-42"))
	ctx = AppendContextAttrs(ctx, slog.String("user_id", "u-99"))
	l.InfoContext(ctx, "ctx attrs append")

	line := strings.TrimSpace(buf.String())
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if m["tenant_id"] != "t-42" || m["user_id"] != "u-99" {
		t.Fatalf("expected appended attrs in log, got=%v", m)
	}
}

func TestContextAttrsReturnsCopy(t *testing.T) {
	ctx := WithContextAttrs(context.Background(), slog.String("tenant_id", "t-42"))
	attrs, ok := ContextAttrs(ctx)
	if !ok || len(attrs) != 1 {
		t.Fatalf("expected context attrs")
	}
	attrs[0] = slog.String("tenant_id", "mutated")

	attrs2, ok := ContextAttrs(ctx)
	if !ok || len(attrs2) != 1 {
		t.Fatalf("expected context attrs on second read")
	}
	if got := attrs2[0].Value.String(); got != "t-42" {
		t.Fatalf("context attrs must be immutable by caller, got=%s", got)
	}
}
