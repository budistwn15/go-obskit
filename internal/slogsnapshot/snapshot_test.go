package slogsnapshot

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

func TestRecordSnapshotsMutableValue(t *testing.T) {
	value := map[string]any{"state": "before"}
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "test", 0)
	rec.AddAttrs(slog.Any("payload", value))
	frozen, _ := Record(rec)
	value["state"] = "after"
	frozen.Attrs(func(attr slog.Attr) bool {
		raw, ok := attr.Value.Any().(json.RawMessage)
		if !ok || string(raw) != `{"state":"before"}` {
			t.Fatalf("snapshot = %T %s", attr.Value.Any(), raw)
		}
		return false
	})
}

func TestRecordSimpleValueDoesNotAllocate(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "test", 0)
	rec.AddAttrs(slog.Int("status", 200))
	if got := testing.AllocsPerRun(1000, func() { _, _ = Record(rec) }); got != 0 {
		t.Fatalf("allocations = %.2f", got)
	}
}
