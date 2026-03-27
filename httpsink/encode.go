package httpsink

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

func marshalBatch(format PayloadFormat, batch []slog.Record, static map[string]any) ([]byte, error) {
	switch format {
	case FormatJSONArray:
		docs := make([]map[string]any, 0, len(batch))
		for _, rec := range batch {
			docs = append(docs, recordToDocument(rec, static))
		}
		return json.Marshal(docs)
	default:
		// NDJSON
		buf := make([]byte, 0, len(batch)*256)
		for _, rec := range batch {
			doc := recordToDocument(rec, static)
			raw, err := json.Marshal(doc)
			if err != nil {
				return nil, err
			}
			buf = append(buf, raw...)
			buf = append(buf, '\n')
		}
		return buf, nil
	}
}
