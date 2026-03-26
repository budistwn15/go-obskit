package httpin

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

type Middleware struct {
	cfg atomic.Pointer[runtimeConfig]
}

func New(cfg Config) *Middleware {
	m := &Middleware{}
	m.Update(cfg)
	return m
}

func (m *Middleware) Update(cfg Config) {
	n := normalizeConfig(cfg)
	m.cfg.Store(&n)
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	if next == nil {
		next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			rc := m.cfg.Load()
			if rc == nil {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			
			correlationID := r.Header.Get(rc.correlationHeader)
			if correlationID == "" {
				correlationID = rc.correlationIDFactory()
			}
			if correlationID != "" {
				w.Header().Set(rc.correlationHeader, correlationID)
			}
			
			traceID, spanID := extractTraceSpanID(r)
			meta := logger.ContextMeta{
				CorrelationID: correlationID,
				RequestID:     r.Header.Get("X-Request-ID"),
				TraceID:       traceID,
				SpanID:        spanID,
				Layer:         string(logger.LayerHandler),
				Component:     "http_server",
				Operation:     "incoming_request",
			}
			ctx := logger.WithMeta(r.Context(), meta)
			r = r.WithContext(ctx)
			
			reqCapture := bodyCapture{}
			reqCaptureErr := ""
			reqCT := r.Header.Get("Content-Type")
			if rc.captureRequestBody && shouldCaptureBody(reqCT) {
				var err error
				reqCapture, err = captureRequestBody(r, rc.maxBodyCaptureBytes)
				if err != nil {
					reqCaptureErr = err.Error()
				}
			}
			
			rw := newResponseCaptureWriter(w, rc.captureResponseBody, rc.maxBodyCaptureBytes)
			next.ServeHTTP(rw, r)
			
			duration := time.Since(start)
			meta.DurationMS = duration.Milliseconds()
			ctx = logger.WithMeta(ctx, meta)
			
			if !safeShouldLog(rc.shouldLog, r, rw.status, duration) {
				return
			}
			
			summaryAttrs := []slog.Attr{
				slog.String("event_kind", "http_in"),
				slog.String("http_method", r.Method),
				slog.String("http_scheme", requestScheme(r)),
				slog.String("http_host", r.Host),
				slog.String("http_path", r.URL.Path),
				slog.String("http_route", requestRoute(r, rc.routeExtractor)),
				slog.String("http_url", r.URL.String()),
				slog.Any("http_query", sanitizeQuery(r.URL.Query(), rc.redactor)),
				slog.String("user_agent", r.UserAgent()),
				slog.String("referer", r.Referer()),
				slog.String("client_ip", clientIP(r, rc.clientIPExtractor)),
				slog.String("x_forwarded_for", r.Header.Get("X-Forwarded-For")),
				slog.String("x_real_ip", r.Header.Get("X-Real-IP")),
				slog.Int("http_status_code", rw.status),
				slog.Int64(logger.FieldDurationMS, duration.Milliseconds()),
				slog.Int64("response_size_bytes", rw.sizeBytes),
			}
			
			if rc.logRequestHeaders {
				summaryAttrs = append(
					summaryAttrs,
					slog.Any("http_request_headers", filterHeaders(r.Header, rc.requestHeaders, rc.redactor)),
				)
			}
			if rc.logResponseHeaders {
				summaryAttrs = append(
					summaryAttrs,
					slog.Any("http_response_headers", filterHeaders(rw.Header(), rc.responseHeaders, rc.redactor)),
				)
			}
			
			safeLog(rc.logger, ctx, slog.LevelInfo, "incoming http request", summaryAttrs...)
			
			if !safeShouldLog(rc.shouldLogDetail, r, rw.status, duration) {
				return
			}
			
			if rc.logRequestBody && rc.captureRequestBody && shouldCaptureBody(reqCT) {
				reqBody := reqCapture.body
				if reqCaptureErr == "" && rc.redactJSONBody && isJSONContentType(reqCT) {
					redactedBody, err := redactJSONBody(reqBody, rc.redactor)
					if err == nil {
						reqBody = redactedBody
					} else {
						reqCaptureErr = err.Error()
					}
				}
				detailAttrs := []slog.Attr{
					slog.String("event_kind", "http_in_request_body"),
					slog.String("http_request_body", reqBody),
					slog.Bool("http_request_body_truncated", reqCapture.truncated),
				}
				if reqCaptureErr != "" {
					detailAttrs = append(detailAttrs, slog.String("go-obskit_degraded_reason", reqCaptureErr))
				}
				safeLog(rc.logger, ctx, slog.LevelDebug, "incoming request body", detailAttrs...)
			}
			
			if rc.logResponseBody && rc.captureResponseBody {
				respCT := rw.Header().Get("Content-Type")
				if shouldCaptureBody(respCT) {
					respBody := rw.bodyString()
					respCaptureErr := ""
					if rc.redactJSONBody && isJSONContentType(respCT) {
						redactedBody, err := redactJSONBody(respBody, rc.redactor)
						if err == nil {
							respBody = redactedBody
						} else {
							respCaptureErr = err.Error()
						}
					}
					detailAttrs := []slog.Attr{
						slog.String("event_kind", "http_in_response_body"),
						slog.String("http_response_body", respBody),
						slog.Bool("http_response_body_truncated", rw.truncated),
					}
					if respCaptureErr != "" {
						detailAttrs = append(detailAttrs, slog.String("go-obskit_degraded_reason", respCaptureErr))
					}
					safeLog(rc.logger, ctx, slog.LevelDebug, "incoming response body", detailAttrs...)
				}
			}
		},
	)
}

func Handler(cfg Config, next http.Handler) http.Handler {
	return New(cfg).Wrap(next)
}

func safeShouldLog(
	fn func(*http.Request, int, time.Duration) bool, r *http.Request, status int, d time.Duration,
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
	return fn(r, status, d)
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

func isJSONContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.Contains(ct, "application/json") || strings.Contains(ct, "+json")
}
