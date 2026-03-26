package httplog

import (
	"net/http"
	"strings"

	"github.com/budistwn15/go-obskit/redact"
)

var defaultSensitiveHeaderSet = redact.DefaultRules().HeaderKeys

func FilterHeaders(headers map[string][]string, allowlist []string, denylist []string) map[string]any {
	if len(headers) == 0 {
		return map[string]any{}
	}
	
	var allow map[string]struct{}
	if len(allowlist) > 0 {
		allow = toSet(allowlist)
	}
	var deny map[string]struct{}
	if len(denylist) > 0 {
		deny = toSet(denylist)
	}
	hasAllowlist := len(allow) > 0
	
	out := make(map[string]any)
	for k, vals := range headers {
		ck := http.CanonicalHeaderKey(k)
		lk := strings.ToLower(strings.TrimSpace(ck))
		if hasAllowlist {
			if _, ok := allow[lk]; !ok {
				continue
			}
		}
		
		if _, ok := defaultSensitiveHeaderSet[lk]; ok {
			out[ck] = redact.RedactedValue
			continue
		}
		if _, ok := deny[lk]; ok {
			out[ck] = redact.RedactedValue
			continue
		}
		out[ck] = strings.Join(vals, ",")
	}
	return out
}

func FilterHTTPHeaders(headers http.Header, allowlist []string, denylist []string) map[string]any {
	if headers == nil {
		return map[string]any{}
	}
	return FilterHeaders(headers, allowlist, denylist)
}
