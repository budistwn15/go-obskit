package nethttp

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/budistwn15/go-obskit/httplog"
)

type requestBodyCapture struct {
	value     string
	truncated bool
	skipped   bool
}

func captureRequestBody(req *http.Request, opts Options, enabled bool) requestBodyCapture {
	if req == nil || req.Body == nil || !enabled {
		return requestBodyCapture{}
	}
	contentType := req.Header.Get("Content-Type")
	if !httplog.IsSafeBodyContentType(contentType) {
		return requestBodyCapture{skipped: true}
	}

	maxBytes := opts.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = httplog.DefaultOptions().MaxBodyBytes
	}
	readLimit := int64(maxBytes + 1)
	buf, err := io.ReadAll(io.LimitReader(req.Body, readLimit))
	if err != nil {
		return requestBodyCapture{skipped: true}
	}
	req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), req.Body))

	captured := httplog.CaptureBody(contentType, buf, maxBytes, opts.BodyJSONDenylist)
	return requestBodyCapture{
		value:     captured.Value,
		truncated: captured.Truncated,
		skipped:   captured.Skipped,
	}
}

func requestMetaFromRequest(req *http.Request, opts Options) httplog.RequestMeta {
	meta := httplog.RequestMeta{
		Method: req.Method,
		Scheme: requestScheme(req),
		Host:   req.Host,
		Path:   req.URL.Path,
		URL:    req.URL.String(),
	}
	httplog.FillSourceFromRemoteAddr(&meta, req.RemoteAddr)
	httplog.FillTargetFromRequest(&meta, req)
	if opts.RouteExtractor != nil {
		meta.Route = strings.TrimSpace(opts.RouteExtractor(req))
	}
	if opts.CaptureQuery {
		meta.Query = httplog.NormalizeQuery(req.URL.Query(), opts.BodyJSONDenylist)
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHTTPHeaders(req.Header, opts.HeaderAllowlist, opts.HeaderDenylist)
	}
	if opts.IncludeUserAgent {
		meta.UserAgent = req.UserAgent()
	}
	if opts.IncludeReferer {
		meta.Referer = req.Referer()
	}
	if opts.IncludeClientIP {
		meta.XForwardedFor = req.Header.Get("X-Forwarded-For")
		meta.XRealIP = req.Header.Get("X-Real-IP")
		meta.ClientIP = clientIP(req)
	}
	return httplog.NormalizeRequestMeta(meta)
}

func responseMetaFromWriter(w *responseWriter, opts Options) httplog.ResponseMeta {
	meta := httplog.ResponseMeta{
		StatusCode: w.status,
		SizeBytes:  w.sizeBytes,
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHTTPHeaders(w.Header(), opts.HeaderAllowlist, opts.HeaderDenylist)
	}
	return httplog.NormalizeResponseMeta(meta)
}

func eventMeta(
	duration time.Duration, slowThreshold time.Duration, correlationID, requestID, traceID, spanID string,
) httplog.EventMeta {
	return httplog.EventMeta{
		CorrelationID:   correlationID,
		RequestID:       requestID,
		TraceID:         traceID,
		SpanID:          spanID,
		Duration:        duration,
		DurationMS:      httplog.DurationMS(duration),
		Slow:            httplog.IsSlowRequest(duration, slowThreshold),
		SlowThresholdMS: slowThreshold.Milliseconds(),
	}
}

func requestScheme(req *http.Request) string {
	if req.Header.Get("X-Forwarded-Proto") != "" {
		return req.Header.Get("X-Forwarded-Proto")
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

func clientIP(req *http.Request) string {
	if xrip := strings.TrimSpace(req.Header.Get("X-Real-IP")); xrip != "" {
		return xrip
	}
	if xff := strings.TrimSpace(req.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	host := req.RemoteAddr
	if host == "" {
		return ""
	}
	return host
}
