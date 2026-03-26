package redact

import (
	"net/http"
	"strings"
)

func RedactHeaders(input http.Header, rules Rules) http.Header {
	if len(input) == 0 {
		return http.Header{}
	}
	out := make(http.Header, len(input))
	for k, vals := range input {
		if isSensitive(rules.HeaderKeys, k) {
			out[k] = []string{RedactedValue}
			continue
		}
		copyVals := make([]string, len(vals))
		copy(copyVals, vals)
		out[k] = copyVals
	}
	return out
}

func TruncateBytes(input []byte, maxBytes int) (out []byte, truncated bool) {
	if maxBytes <= 0 {
		return []byte{}, len(input) > 0
	}
	if len(input) <= maxBytes {
		cp := make([]byte, len(input))
		copy(cp, input)
		return cp, false
	}
	out = make([]byte, maxBytes)
	copy(out, input[:maxBytes])
	return out, true
}

func HeaderValue(input http.Header, key string) string {
	if input == nil {
		return ""
	}
	return strings.TrimSpace(input.Get(key))
}
