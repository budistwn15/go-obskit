package elastic

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestRecordToDocument_ParsesBodyJSON(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "http", 0)
	rec.AddAttrs(
		slog.String("http.request.body", `{"email":"a@b.c","password":"***redacted***"}`),
		slog.String("http.response.body", `{"ok":true,"token":"***redacted***"}`),
	)
	doc := recordToDocument(rec, nil)

	if _, ok := doc["http.request.body_json"]; !ok {
		t.Fatalf("expected http.request.body_json")
	}
	if _, ok := doc["http.response.body_json"]; !ok {
		t.Fatalf("expected http.response.body_json")
	}
	if _, ok := doc["http.request.body"]; !ok {
		t.Fatalf("expected original http.request.body to remain")
	}
	if doc["http.request.body.email"] != "a@b.c" {
		t.Fatalf("expected flattened email, got=%v", doc["http.request.body.email"])
	}
}

func TestRecordToDocument_ExtractsDBTable(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "db", 0)
	rec.AddAttrs(slog.String("db.statement", "SELECT * FROM users WHERE id=1"))
	doc := recordToDocument(rec, nil)
	if doc["db.table"] != "users" {
		t.Fatalf("expected users table, got=%v", doc["db.table"])
	}
}

func TestRecordToDocument_ParsesGenericJSONStringFields(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "evt", 0)
	rec.AddAttrs(
		slog.String("custom.payload", `{"tenant_id":"t-1","batch_id":"b-1"}`),
		slog.String("custom.invalid", `{"tenant_id"`),
	)
	doc := recordToDocument(rec, nil)

	if _, ok := doc["custom.payload_json"]; !ok {
		t.Fatalf("expected custom.payload_json")
	}
	if _, ok := doc["custom.payload"]; !ok {
		t.Fatalf("expected original custom.payload to remain")
	}
	if _, ok := doc["custom.invalid_json"]; ok {
		t.Fatalf("invalid json should not produce *_json field")
	}
	if doc["custom.payload.tenant_id"] != "t-1" {
		t.Fatalf("expected flattened custom.payload.tenant_id")
	}
	if doc["custom.payload.batch_id"] != "b-1" {
		t.Fatalf("expected flattened custom.payload.batch_id")
	}
}

func TestRecordToDocument_DoesNotOverrideExistingJSONField(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "evt", 0)
	rec.AddAttrs(
		slog.String("custom.payload", `{"x":1}`),
		slog.String("custom.payload_json", `{"already":"present"}`),
	)
	doc := recordToDocument(rec, nil)
	if doc["custom.payload_json"] != `{"already":"present"}` {
		t.Fatalf("existing *_json field should not be overwritten")
	}
}

func TestRecordToDocument_SkipsVeryLargeJSONExpand(t *testing.T) {
	largeJSON := `{"x":"` + strings.Repeat("a", maxAutoJSONExpandBytes+1) + `"}`
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "evt", 0)
	rec.AddAttrs(slog.String("huge.body", largeJSON))

	doc := recordToDocument(rec, nil)
	if _, ok := doc["huge.body_json"]; ok {
		t.Fatalf("very large json should be skipped from auto expansion")
	}
}

func TestRecordToDocument_FlattensNestedObjects(t *testing.T) {
	rec := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "evt", 0)
	rec.AddAttrs(slog.String("http.response.body", `{"status":"ok","data":{"email":"a@b.c","name":"Budi"}}`))
	doc := recordToDocument(rec, nil)

	if doc["http.response.body.status"] != "ok" {
		t.Fatalf("expected flattened status")
	}
	if doc["http.response.body.data.email"] != "a@b.c" {
		t.Fatalf("expected flattened nested email")
	}
	if doc["http.response.body.data.name"] != "Budi" {
		t.Fatalf("expected flattened nested name")
	}
}
