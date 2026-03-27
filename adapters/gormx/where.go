package gormx

import (
	"strings"

	"github.com/budistwn15/go-obskit/redact"
)

type whereCondition struct {
	Column string `json:"column"`
	Op     string `json:"op"`
	Value  string `json:"value"`
}

func extractWhere(statement string, maxConditions int, redactSensitive bool) (columns []string, values map[string]string, conditions []whereCondition) {
	if maxConditions <= 0 {
		maxConditions = 16
	}
	where := findWhereClause(statement)
	if where == "" {
		return nil, nil, nil
	}
	parts := splitConditions(where)
	if len(parts) == 0 {
		return nil, nil, nil
	}
	seenCols := make(map[string]struct{}, len(parts))
	values = make(map[string]string, len(parts))
	conditions = make([]whereCondition, 0, len(parts))

	for _, part := range parts {
		if len(conditions) >= maxConditions {
			break
		}
		col, op, val, ok := parseCondition(part)
		if !ok {
			continue
		}
		if redactSensitive && isSensitiveWhereColumn(col) {
			val = redact.RedactedValue
		}
		if _, ok := seenCols[col]; !ok {
			seenCols[col] = struct{}{}
			columns = append(columns, col)
		}
		if _, exists := values[col]; !exists {
			values[col] = val
		}
		conditions = append(conditions, whereCondition{Column: col, Op: op, Value: val})
	}
	if len(columns) == 0 {
		return nil, nil, nil
	}
	return columns, values, conditions
}

func findWhereClause(statement string) string {
	s := strings.TrimSpace(statement)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	idx := strings.Index(lower, " where ")
	if idx < 0 {
		return ""
	}
	where := s[idx+7:]
	lowerWhere := strings.ToLower(where)
	for _, term := range []string{" group by ", " order by ", " limit ", " returning ", " for update ", ";"} {
		if cut := strings.Index(lowerWhere, term); cut >= 0 {
			where = where[:cut]
			break
		}
	}
	return strings.TrimSpace(where)
}

func splitConditions(where string) []string {
	if where == "" {
		return nil
	}
	s := strings.TrimSpace(where)
	var out []string
	start := 0
	depth := 0
	inSingle := false
	inDouble := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '(':
			if !inSingle && !inDouble {
				depth++
			}
		case ')':
			if !inSingle && !inDouble && depth > 0 {
				depth--
			}
		}
		if inSingle || inDouble || depth > 0 {
			continue
		}
		if i+5 <= len(s) && strings.EqualFold(s[i:i+5], " and ") {
			part := strings.TrimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			i += 4
			start = i + 1
			continue
		}
		if i+4 <= len(s) && strings.EqualFold(s[i:i+4], " or ") {
			part := strings.TrimSpace(s[start:i])
			if part != "" {
				out = append(out, part)
			}
			i += 3
			start = i + 1
			continue
		}
	}
	last := strings.TrimSpace(s[start:])
	if last != "" {
		out = append(out, last)
	}
	return out
}

func parseCondition(part string) (col, op, val string, ok bool) {
	p := strings.TrimSpace(part)
	if p == "" {
		return "", "", "", false
	}
	lower := strings.ToLower(p)
	ops := []string{" is not ", " not in ", " ilike ", " like ", " is ", " in ", ">=", "<=", "<>", "!=", "=", ">", "<"}
	for _, token := range ops {
		idx := strings.Index(lower, token)
		if idx < 0 {
			continue
		}
		col = cleanWhereColumn(p[:idx])
		op = strings.TrimSpace(strings.ToUpper(token))
		val = normalizeWhereValue(p[idx+len(token):])
		if col == "" || val == "" {
			return "", "", "", false
		}
		return col, op, val, true
	}
	return "", "", "", false
}

func cleanWhereColumn(in string) string {
	s := strings.TrimSpace(in)
	s = strings.Trim(s, "`\"")
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		s = parts[len(parts)-1]
		s = strings.Trim(s, "`\"")
	}
	return strings.TrimSpace(s)
}

func normalizeWhereValue(in string) string {
	v := strings.TrimSpace(in)
	v = strings.TrimSuffix(v, ";")
	v = strings.TrimSpace(v)
	if len(v) >= 2 {
		if (v[0] == '\'' && v[len(v)-1] == '\'') || (v[0] == '"' && v[len(v)-1] == '"') {
			v = v[1 : len(v)-1]
		}
	}
	if len(v) > 128 {
		v = v[:128]
	}
	return v
}

func isSensitiveWhereColumn(col string) bool {
	c := strings.ToLower(strings.TrimSpace(col))
	if c == "" {
		return false
	}
	switch c {
	case "email", "phone", "nik", "password", "token", "secret", "access_token", "refresh_token":
		return true
	default:
		return false
	}
}
