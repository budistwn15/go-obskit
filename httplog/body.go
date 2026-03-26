package httplog

import (
	"strings"

	"github.com/budistwn15/go-obskit/redact"
)

var defaultRedactRules = redact.DefaultRules()

type BodyCapture struct {
	Value     string
	Truncated bool
	Skipped   bool
	Reason    string
}

func CaptureBody(contentType string, body []byte, maxBytes int, jsonDenylist []string) BodyCapture {
	if len(body) == 0 {
		return BodyCapture{}
	}
	if !IsSafeBodyContentType(contentType) {
		return BodyCapture{Skipped: true, Reason: "content_type_not_captured"}
	}
	
	if maxBytes <= 0 {
		maxBytes = 4 * 1024
	}
	
	if isJSONContentType(contentType) {
		rules := defaultRedactRules
		if len(jsonDenylist) > 0 {
			rules.JSONKeys = toSet(jsonDenylist)
		}
		out, truncated := redact.RedactJSONBytes(body, maxBytes, rules)
		return BodyCapture{
			Value:     string(out),
			Truncated: truncated,
		}
	}
	
	out, truncated := redact.TruncateBytes(body, maxBytes)
	return BodyCapture{
		Value:     string(out),
		Truncated: truncated,
	}
}

func IsSafeBodyContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		return false
	}
	if strings.HasPrefix(ct, "multipart/") {
		return false
	}
	if strings.HasPrefix(ct, "image/") || strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		return false
	}
	if strings.Contains(ct, "application/octet-stream") ||
		strings.Contains(ct, "application/pdf") ||
		strings.Contains(ct, "application/zip") ||
		strings.Contains(ct, "application/gzip") {
		return false
	}
	if strings.HasPrefix(ct, "text/") {
		return true
	}
	if strings.Contains(ct, "application/json") ||
		strings.Contains(ct, "+json") ||
		strings.Contains(ct, "application/xml") ||
		strings.Contains(ct, "application/x-www-form-urlencoded") {
		return true
	}
	return false
}

func isJSONContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "+json")
}
