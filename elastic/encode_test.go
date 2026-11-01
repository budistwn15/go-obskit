package elastic

import (
	"log/slog"
	"testing"
	"time"
)

func TestRecordToDocument_ExtractsDBTable(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "db", 0)
	rec.AddAttrs(slog.String("db.statement", "SELECT * FROM users WHERE id=1"))
	doc := recordToDocument(rec, nil)
	if doc["db.collection.name"] != "users" {
		t.Fatalf("expected users table, got=%v", doc["db.collection.name"])
	}
}

func TestRecordToDocument_DoesNotExpandJSONStrings(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "evt", 0)
	rec.AddAttrs(slog.String("payload", `{"token":"redacted"}`))
	doc := recordToDocument(rec, nil)
	if _, ok := doc["payload_json"]; ok {
		t.Fatal("JSON strings must remain opaque")
	}
}
