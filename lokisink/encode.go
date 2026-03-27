package lokisink

import (
	"encoding/json"
	"log/slog"
	"time"
)

func recordToDocument(rec slog.Record, static map[string]any) map[string]any {
	doc := make(map[string]any, 20)
	doc["@timestamp"] = rec.Time.UTC().Format(time.RFC3339Nano)
	doc["message"] = rec.Message
	doc["level"] = rec.Level.String()
	for k, v := range static {
		doc[k] = v
	}
	rec.Attrs(func(a slog.Attr) bool {
		applyAttr(doc, a)
		return true
	})
	return doc
}

func applyAttr(m map[string]any, attr slog.Attr) {
	if attr.Key == "" && attr.Value.Kind() != slog.KindGroup {
		return
	}
	v := attr.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		grp := v.Group()
		if attr.Key == "" {
			for _, ga := range grp {
				applyAttr(m, ga)
			}
			return
		}
		nested := make(map[string]any, len(grp))
		for _, ga := range grp {
			applyAttr(nested, ga)
		}
		m[attr.Key] = nested
		return
	}
	m[attr.Key] = v.Any()
}

func recordLine(rec slog.Record, static map[string]any) (string, error) {
	doc := recordToDocument(rec, static)
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
