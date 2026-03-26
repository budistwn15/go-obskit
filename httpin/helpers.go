package httpin

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/budistwn15/go-obskit/logger"
)

func newCorrelationID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "corr-fallback"
	}
	return hex.EncodeToString(b[:])
}

func requestScheme(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xf := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); xf != "" {
		return xf
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestRoute(r *http.Request, routeExtractor func(*http.Request) string) string {
	if routeExtractor != nil {
		route := strings.TrimSpace(routeExtractor(r))
		if route != "" {
			return route
		}
	}
	route, _ := RouteFromContext(r.Context())
	return strings.TrimSpace(route)
}

func clientIP(r *http.Request, customExtractor func(*http.Request) string) string {
	if customExtractor != nil {
		if ip := strings.TrimSpace(customExtractor(r)); ip != "" {
			return ip
		}
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

func filterHeaders(h http.Header, allow map[string]struct{}, redactor *logger.Redactor) map[string]any {
	if len(h) == 0 || len(allow) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any)
	for k, vals := range h {
		ck := http.CanonicalHeaderKey(k)
		if _, ok := allow[ck]; !ok {
			continue
		}
		value := strings.Join(vals, ",")
		if redactor != nil && redactor.IsSensitive(ck) {
			value = redactor.Mask()
		}
		out[ck] = value
	}
	return out
}

func sanitizeQuery(q map[string][]string, redactor *logger.Redactor) map[string]any {
	if len(q) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(q))
	for key, vals := range q {
		if redactor != nil && redactor.IsSensitive(key) {
			out[key] = redactor.Mask()
			continue
		}
		if len(vals) == 1 {
			out[key] = vals[0]
			continue
		}
		copyVals := make([]string, len(vals))
		copy(copyVals, vals)
		out[key] = copyVals
	}
	return out
}

func shouldCaptureBody(contentType string) bool {
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

type bodyCapture struct {
	body      string
	truncated bool
}

func captureRequestBody(r *http.Request, maxBytes int) (bodyCapture, error) {
	if r == nil || r.Body == nil {
		return bodyCapture{}, nil
	}
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	
	readLimit := int64(maxBytes + 1)
	buf, err := io.ReadAll(io.LimitReader(r.Body, readLimit))
	if err != nil {
		return bodyCapture{}, err
	}
	
	truncated := len(buf) > maxBytes
	captured := buf
	if truncated {
		captured = captured[:maxBytes]
	}
	
	r.Body = &replayBody{
		Reader: io.MultiReader(bytes.NewReader(buf), r.Body),
		Closer: r.Body,
	}
	
	return bodyCapture{
		body:      string(captured),
		truncated: truncated,
	}, nil
}

type replayBody struct {
	io.Reader
	io.Closer
}

func redactJSONBody(body string, redactor *logger.Redactor) (string, error) {
	if redactor == nil || strings.TrimSpace(body) == "" {
		return body, nil
	}
	
	var payload any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return body, err
	}
	redacted := redactAny(payload, redactor)
	out, err := json.Marshal(redacted)
	if err != nil {
		return body, err
	}
	return string(out), nil
}

func redactAny(v any, redactor *logger.Redactor) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, subv := range val {
			if redactor.IsSensitive(k) {
				out[k] = redactor.Mask()
				continue
			}
			out[k] = redactAny(subv, redactor)
		}
		return out
	case []any:
		out := make([]any, 0, len(val))
		for _, item := range val {
			out = append(out, redactAny(item, redactor))
		}
		return out
	default:
		return v
	}
}

func extractTraceSpanID(r *http.Request) (string, string) {
	traceID := strings.TrimSpace(r.Header.Get("X-Trace-ID"))
	spanID := strings.TrimSpace(r.Header.Get("X-Span-ID"))
	
	if traceID != "" || spanID != "" {
		return traceID, spanID
	}
	
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return "", ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) < 4 {
		return "", ""
	}
	return parts[1], parts[2]
}
