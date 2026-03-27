package elastic

import (
	"encoding/json"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

const maxAutoJSONExpandBytes = 64 * 1024
const maxAutoJSONFlattenFields = 256

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
	enrichDocument(doc)

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

func enrichDocument(doc map[string]any) {
	expandJSONStringFields(doc)
	if _, exists := doc["db.table"]; exists {
		return
	}
	stmt, _ := doc["db.statement"].(string)
	if table := extractTableFromSQL(stmt); table != "" {
		doc["db.table"] = table
	}
}

func expandJSONStringFields(doc map[string]any) {
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	for _, srcKey := range keys {
		if strings.HasSuffix(srcKey, "_json") {
			continue
		}
		dstKey := srcKey + "_json"
		if _, exists := doc[dstKey]; exists {
			continue
		}
		raw, ok := doc[srcKey].(string)
		if !ok {
			continue
		}
		body := strings.TrimSpace(raw)
		if body == "" || len(body) > maxAutoJSONExpandBytes {
			continue
		}
		if !(strings.HasPrefix(body, "{") || strings.HasPrefix(body, "[")) {
			continue
		}
		var out any
		if err := json.Unmarshal([]byte(body), &out); err != nil {
			continue
		}
		doc[dstKey] = out
		remaining := maxAutoJSONFlattenFields
		flattenJSONValue(doc, srcKey, out, &remaining)
	}
}

func flattenJSONValue(doc map[string]any, path string, val any, remaining *int) {
	if remaining == nil || *remaining <= 0 || path == "" {
		return
	}
	switch x := val.(type) {
	case map[string]any:
		for k, v := range x {
			if *remaining <= 0 {
				return
			}
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			flattenJSONValue(doc, path+"."+k, v, remaining)
		}
	case []any:
		for i, v := range x {
			if *remaining <= 0 {
				return
			}
			flattenJSONValue(doc, path+"."+strconv.Itoa(i), v, remaining)
		}
	default:
		if _, exists := doc[path]; exists {
			return
		}
		doc[path] = x
		*remaining = *remaining - 1
	}
}

func extractTableFromSQL(statement string) string {
	s := strings.TrimSpace(statement)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return ""
	}
	upper := strings.ToUpper(parts[0])
	switch upper {
	case "SELECT", "DELETE":
		return nextAfterKeyword(parts, "FROM")
	case "UPDATE":
		return cleanIdent(parts[1])
	case "INSERT":
		if t := nextAfterKeyword(parts, "INTO"); t != "" {
			return t
		}
	}
	return ""
}

func nextAfterKeyword(parts []string, kw string) string {
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], kw) {
			return cleanIdent(parts[i+1])
		}
	}
	return ""
}

func cleanIdent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"")
	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSuffix(s, ",")
	s = strings.TrimPrefix(s, "(")
	return s
}
