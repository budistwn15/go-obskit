package httpout

import (
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

type Transport struct {
	base http.RoundTripper
	cfg  atomic.Pointer[runtimeConfig]
}

func New(base http.RoundTripper, cfg Config) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	t := &Transport{base: base}
	t.Update(cfg)
	return t
}

func (t *Transport) Update(cfg Config) {
	n := normalizeConfig(cfg)
	t.cfg.Store(&n)
}

func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	if req == nil {
		return t.base.RoundTrip(req)
	}
	
	rc := t.cfg.Load()
	if rc == nil {
		return t.base.RoundTrip(req)
	}
	
	start := time.Now()
	ctx := req.Context()
	
	correlationID := logger.CorrelationID(ctx)
	if correlationID != "" {
		req.Header.Set(rc.correlationHeader, correlationID)
	}
	
	traceID, spanID := extractTraceSpanID(req)
	meta := logger.ContextMeta{
		CorrelationID: correlationID,
		RequestID:     req.Header.Get("X-Request-ID"),
		TraceID:       traceID,
		SpanID:        spanID,
		Layer:         string(rc.layer),
		Component:     rc.component,
		Operation:     rc.operation,
	}
	ctx = logger.WithMeta(ctx, meta)
	req = req.WithContext(ctx)
	
	reqCT := req.Header.Get("Content-Type")
	reqBody := bodyCapture{}
	reqBodyErr := ""
	if rc.captureRequestBody && shouldCaptureBody(reqCT) {
		reqBody, err = captureRequestBody(req, rc.maxBodyCaptureBytes)
		if err != nil {
			reqBodyErr = err.Error()
		}
	}
	
	resp, err = t.base.RoundTrip(req)
	duration := time.Since(start)
	meta.DurationMS = duration.Milliseconds()
	ctx = logger.WithMeta(ctx, meta)
	
	if !safeShouldLog(rc.shouldLog, req, resp, err, duration) {
		return resp, err
	}
	
	target := req.URL.Host
	if rc.targetResolver != nil {
		if v := rc.targetResolver(req); v != "" {
			target = v
		}
	}
	
	statusCode := 0
	respSize := int64(0)
	respHeaders := map[string]any{}
	if resp != nil {
		statusCode = resp.StatusCode
		respSize = resp.ContentLength
		if rc.logResponseHeaders {
			respHeaders = filterHeaders(resp.Header, rc.responseHeaders, rc.redactor)
		}
	}
	
	summaryAttrs := []slog.Attr{
		slog.String("event_kind", "http_out"),
		slog.String("target_service", target),
		slog.String("http_method", req.Method),
		slog.String("http_url", req.URL.String()),
		slog.Any("http_query", sanitizeQuery(req.URL.Query(), rc.redactor)),
		slog.Int("http_status_code", statusCode),
		slog.Int64(logger.FieldDurationMS, duration.Milliseconds()),
		slog.Int64("response_size_bytes", respSize),
	}
	if err != nil {
		summaryAttrs = append(
			summaryAttrs,
			slog.String("error_kind", classifyTransportError(err)),
			slog.String("error_message", err.Error()),
		)
	}
	if rc.logRequestHeaders {
		summaryAttrs = append(
			summaryAttrs, slog.Any("http_request_headers", filterHeaders(req.Header, rc.requestHeaders, rc.redactor)),
		)
	}
	if rc.logResponseHeaders && resp != nil {
		summaryAttrs = append(summaryAttrs, slog.Any("http_response_headers", respHeaders))
	}
	safeLog(rc.logger, ctx, slog.LevelInfo, "outbound http request", summaryAttrs...)
	
	if !safeShouldLog(rc.shouldLogDetail, req, resp, err, duration) {
		return resp, err
	}
	
	if rc.logRequestBody && rc.captureRequestBody && shouldCaptureBody(reqCT) {
		body := reqBody.body
		if reqBodyErr == "" && rc.redactJSONBody && isJSONContentType(reqCT) {
			redactedBody, rerr := redactJSONBody(body, rc.redactor)
			if rerr == nil {
				body = redactedBody
			} else {
				reqBodyErr = rerr.Error()
			}
		}
		attrs := []slog.Attr{
			slog.String("event_kind", "http_out_request_body"),
			slog.String("http_request_body", body),
			slog.Bool("http_request_body_truncated", reqBody.truncated),
		}
		if reqBodyErr != "" {
			attrs = append(attrs, slog.String("go-obskit_degraded_reason", reqBodyErr))
		}
		safeLog(rc.logger, ctx, slog.LevelDebug, "outbound request body", attrs...)
	}
	
	if rc.logResponseBody && rc.captureResponseBody && resp != nil {
		respCT := resp.Header.Get("Content-Type")
		if shouldCaptureBody(respCT) {
			respBody, rerr := captureResponseBody(resp, rc.maxBodyCaptureBytes)
			respBodyErr := ""
			if rerr != nil {
				respBodyErr = rerr.Error()
			}
			body := respBody.body
			if respBodyErr == "" && rc.redactJSONBody && isJSONContentType(respCT) {
				redactedBody, redErr := redactJSONBody(body, rc.redactor)
				if redErr == nil {
					body = redactedBody
				} else {
					respBodyErr = redErr.Error()
				}
			}
			attrs := []slog.Attr{
				slog.String("event_kind", "http_out_response_body"),
				slog.String("http_response_body", body),
				slog.Bool("http_response_body_truncated", respBody.truncated),
			}
			if respBodyErr != "" {
				attrs = append(attrs, slog.String("go-obskit_degraded_reason", respBodyErr))
			}
			safeLog(rc.logger, ctx, slog.LevelDebug, "outbound response body", attrs...)
		}
	}
	
	return resp, err
}

func WrapClient(client *http.Client, cfg Config) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	client.Transport = New(base, cfg)
	return client
}

func extractTraceSpanID(r *http.Request) (string, string) {
	traceID := r.Header.Get("X-Trace-ID")
	spanID := r.Header.Get("X-Span-ID")
	if traceID != "" || spanID != "" {
		return traceID, spanID
	}
	traceparent := r.Header.Get("traceparent")
	if traceparent == "" {
		return "", ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) < 4 {
		return "", ""
	}
	return parts[1], parts[2]
}
