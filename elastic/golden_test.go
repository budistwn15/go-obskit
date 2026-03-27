package elastic

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecordToDocument_GoldenSnapshot(t *testing.T) {
	rec := slog.NewRecord(time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC), slog.LevelInfo, "golden event", 0)
	rec.AddAttrs(
		slog.String("http.request.body", `{"status":"ok","data":{"email":"a@b.c"}}`),
		slog.Int("http.status_code", 200),
		slog.String("db.statement", "SELECT * FROM users WHERE id=1"),
		slog.String("custom.payload", `{"tenant_id":"t-1"}`),
	)
	doc := recordToDocument(rec, map[string]any{"environment": "test"})

	got, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantPath := filepath.Join("testdata", "golden_document.json")
	want, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	if strings.TrimSpace(string(got)) != strings.TrimSpace(string(want)) {
		t.Fatalf("golden mismatch.\n--- got ---\n%s\n--- want ---\n%s", string(got), string(want))
	}
}
