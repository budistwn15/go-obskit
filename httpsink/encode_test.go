package httpsink

import (
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestMarshalBatchPreservesRecordFields(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "hello", 0)
	rec.AddAttrs(
		slog.Int("count", 2),
		slog.Bool("ok", true),
		slog.Group("http", slog.String("method", "GET")),
		slog.Any("labels", map[string]string{"team": "payments"}),
	)
	for _, format := range []PayloadFormat{FormatNDJSON, FormatJSONArray} {
		t.Run(string(format), func(t *testing.T) {
			raw, err := marshalBatch(format, []slog.Record{rec}, map[string]any{"service.name": "billing"})
			if err != nil {
				t.Fatal(err)
			}
			payload := strings.TrimSpace(string(raw))
			if format == FormatJSONArray {
				payload = strings.TrimSuffix(strings.TrimPrefix(payload, "["), "]")
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if got["message"] != "hello" || got["service.name"] != "billing" || got["count"] != float64(2) {
				t.Fatalf("missing fields: %v", got)
			}
			group, ok := got["http"].(map[string]any)
			if !ok || group["method"] != "GET" {
				t.Fatalf("missing group: %v", got["http"])
			}
		})
	}
}

func TestMarshalBatchAllocationBudget(t *testing.T) {
	records := make([]slog.Record, 100)
	for i := range records {
		records[i] = slog.NewRecord(time.Unix(0, int64(i)), slog.LevelInfo, "budget", 0)
		records[i].AddAttrs(slog.Int("index", i))
	}
	if got := testing.AllocsPerRun(100, func() {
		_, _ = marshalBatch(FormatNDJSON, records, nil)
	}); got > 120 {
		t.Fatalf("batch allocations = %.0f, budget 120", got)
	}
}
