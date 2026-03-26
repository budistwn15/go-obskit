package httpout

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

type bodyCapture struct {
	body string
	truncated bool
}

type replayBody struct {
	io.Reader
	io.Closer
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

func isJSONContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "+json")
}

func captureRequestBody(req *http.Request, maxBytes int) (bodyCapture, error) {
	if req == nil || req.Body == nil {
		return bodyCapture{}, nil
	}
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	readLimit := int64(maxBytes + 1)
	buf, err := io.ReadAll(io.LimitReader(req.Body, readLimit))
	if err != nil {
		return bodyCapture{}, err
	}
	truncated := len(buf) > maxBytes
	captured := buf
	if truncated {
		captured = captured[:maxBytes]
	}
	req.Body = &replayBody{
		Reader: io.MultiReader(bytes.NewReader(buf), req.Body),
		Closer: req.Body,
	}
	return bodyCapture{body: string(captured), truncated: truncated}, nil
}

func captureResponseBody(resp *http.Response, maxBytes int) (bodyCapture, error) {
	if resp == nil || resp.Body == nil {
		return bodyCapture{}, nil
	}
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	readLimit := int64(maxBytes + 1)
	buf, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
	if err != nil {
		return bodyCapture{}, err
	}
	truncated := len(buf) > maxBytes
	captured := buf
	if truncated {
		captured = captured[:maxBytes]
	}
	resp.Body = &replayBody{
		Reader: io.MultiReader(bytes.NewReader(buf), resp.Body),
		Closer: resp.Body,
	}
	return bodyCapture{body: string(captured), truncated: truncated}, nil
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
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[key] = cp
	}
	return out
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

func safeShouldLog(
	fn func(*http.Request, *http.Response, error, time.Duration) bool, req *http.Request, resp *http.Response,
	err error, d time.Duration,
) (ok bool) {
	ok = true
	if fn == nil {
		return ok
	}
	defer func() {
		if recover() != nil {
			ok = true
		}
	}()
	return fn(req, resp, err, d)
}

func safeLog(l *slog.Logger, ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	defer func() {
		_ = recover()
	}()
	if l == nil {
		l = slog.Default()
	}
	args := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		args = append(args, attr)
	}
	l.Log(ctx, level, msg, args...)
}

func classifyTransportError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "context_deadline_exceeded"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "dns_error"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Timeout() {
			return "timeout"
		}
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return "connection_refused"
		}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return "timeout"
		}
	}
	return "transport_error"
}
