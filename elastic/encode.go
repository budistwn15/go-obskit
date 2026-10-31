package elastic

import (
	"log/slog"
	"strings"
	"time"
)

func recordToDocument(rec slog.Record, static map[string]any) map[string]any {
	doc := make(map[string]any, 20)
	doc["@timestamp"] = rec.Time.UTC().Format(time.RFC3339Nano)
	doc["message"] = rec.Message
	doc["log.level"] = rec.Level.String()

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
	if _, exists := doc["db.table"]; exists {
		return
	}
	stmt, _ := doc["db.statement"].(string)
	if table := extractTableFromSQL(stmt); table != "" {
		doc["db.table"] = table
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
