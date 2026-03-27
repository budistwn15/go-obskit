package redact

import "encoding/json"

func RedactJSONBytes(input []byte, maxBytes int, rules Rules) (output []byte, truncated bool) {
	if len(input) == 0 {
		return []byte{}, false
	}

	var payload any
	if err := json.Unmarshal(input, &payload); err != nil {
		return safeFallback(maxBytes), len(safeFallback(maxBytes)) >= maxBytes && maxBytes > 0
	}

	redacted := redactAny(payload, rules)
	raw, err := json.Marshal(redacted)
	if err != nil {
		return safeFallback(maxBytes), len(safeFallback(maxBytes)) >= maxBytes && maxBytes > 0
	}
	return TruncateBytes(raw, maxBytes)
}

func redactAny(value any, rules Rules) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, nested := range v {
			if isSensitive(rules.JSONKeys, key) {
				out[key] = RedactedValue
				continue
			}
			out[key] = redactAny(nested, rules)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = redactAny(v[i], rules)
		}
		return out
	case string:
		return redactStringValue(v, rules)
	default:
		return value
	}
}

func redactStringValue(v string, rules Rules) string {
	if !rules.EnabledPatternRedaction || len(rules.ValuePatterns) == 0 || v == "" {
		return v
	}
	redacted := v
	for _, p := range rules.ValuePatterns {
		if p.Expr == nil {
			continue
		}
		redacted = p.Expr.ReplaceAllString(redacted, RedactedValue)
	}
	return redacted
}

func safeFallback(maxBytes int) []byte {
	fallback := []byte(`{"_redaction":"failed","_value":"***redacted***"}`)
	out, _ := TruncateBytes(fallback, maxBytes)
	return out
}
