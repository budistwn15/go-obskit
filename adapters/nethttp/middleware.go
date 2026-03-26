package nethttp

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/budistwn15/go-obskit/logger"
)

func Middleware(log *slog.Logger, opts Options) func(http.Handler) http.Handler {
	if log == nil {
		log = slog.Default()
	}
	opts = normalizeOptions(opts)
	sampler := httplog.NewSuccessSampler(opts.SuccessSampleEvery)
	
	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		}
		
		return http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
				start := time.Now()
				ctx := req.Context()
				anyLifecycleLog := opts.LogRequestStart || opts.LogRequestComplete || opts.LogRequestError
				
				correlationID := req.Header.Get(opts.CorrelationHeader)
				if correlationID == "" {
					ctx, correlationID = correlation.GetOrGenerate(ctx)
				} else {
					ctx = correlation.WithID(ctx, correlationID)
				}
				
				requestID := req.Header.Get("X-Request-ID")
				traceID, spanID := traceIDs(req)
				ctx = logger.WithMeta(
					ctx, logger.ContextMeta{
						CorrelationID: correlationID,
						RequestID:     requestID,
						TraceID:       traceID,
						SpanID:        spanID,
					},
				)
				req = req.WithContext(ctx)
				req.Header.Set(opts.CorrelationHeader, correlationID)
				w.Header().Set(opts.CorrelationHeader, correlationID)
				
				if !anyLifecycleLog {
					next.ServeHTTP(w, req)
					return
				}
				
				requestMeta := requestMetaFromRequest(req, opts)
				shouldCaptureReqBody := opts.CaptureRequestBody && anyLifecycleLog
				bodyCapture := captureRequestBody(req, opts, shouldCaptureReqBody)
				if bodyCapture.value != "" {
					requestMeta.RequestBody = bodyCapture.value
					requestMeta.RequestBodyTruncated = bodyCapture.truncated
				}
				
				if opts.LogRequestStart && shouldLog(
					sampler, opts.ShouldLogStart, "start", requestMeta, httplog.ResponseMeta{}, httplog.EventMeta{},
					nil, opts.RecoverInternally,
				) {
					event := httplog.BuildRequestStart(
						requestMeta, eventMeta(0, opts.SlowRequestThreshold, correlationID, requestID, traceID, spanID),
					)
					logEvent(log, ctx, event, opts.RecoverInternally)
				}
				
				captureRespBody := opts.CaptureResponseBody && opts.LogErrorBodies
				rw := newResponseWriter(w, captureRespBody, opts.MaxBodyBytes)
				next.ServeHTTP(rw, req)
				
				duration := httplog.Since(start)
				responseMeta := responseMetaFromWriter(rw, opts)
				if captureRespBody && rw.status >= 400 && rw.body.Len() > 0 {
					contentType := rw.Header().Get("Content-Type")
					bodyCap := httplog.CaptureBody(
						contentType, rw.body.Bytes(), opts.MaxBodyBytes, opts.BodyJSONDenylist,
					)
					responseMeta.ResponseBody = bodyCap.Value
					responseMeta.ResponseBodyTruncated = bodyCap.Truncated || rw.bodyTruncated
				}
				
				evMeta := eventMeta(duration, opts.SlowRequestThreshold, correlationID, requestID, traceID, spanID)
				if rw.status >= http.StatusInternalServerError {
					evMeta.ErrorKind = "http_error"
					evMeta.ErrorMessage = http.StatusText(rw.status)
				}
				if rw.status >= 400 {
					if !opts.LogErrorHeaders {
						responseMeta.Headers = nil
					}
				} else if !opts.LogSuccessHeaders {
					responseMeta.Headers = nil
				}
				
				if rw.status >= 500 && opts.LogRequestError && shouldLog(
					sampler, opts.ShouldLogError, "error", requestMeta, responseMeta, evMeta, nil,
					opts.RecoverInternally,
				) {
					event := httplog.BuildRequestError(requestMeta, responseMeta, evMeta)
					logEvent(log, ctx, event, opts.RecoverInternally)
					return
				}
				if opts.LogRequestComplete && shouldLog(
					sampler, opts.ShouldLogComplete, "complete", requestMeta, responseMeta, evMeta, nil,
					opts.RecoverInternally,
				) {
					event := httplog.BuildRequestComplete(requestMeta, responseMeta, evMeta)
					logEvent(log, ctx, event, opts.RecoverInternally)
				}
			},
		)
	}
}

func shouldLog(
	sampler *httplog.SuccessSampler, hook httplog.DecisionHook, event string, req httplog.RequestMeta,
	res httplog.ResponseMeta, ev httplog.EventMeta, err error, recoverInternally bool,
) bool {
	meta := httplog.DecisionMeta{
		Event:     event,
		Request:   req,
		Response:  res,
		EventMeta: ev,
		Err:       err,
	}
	if !sampler.ShouldLog(meta) {
		return false
	}
	if hook == nil {
		return true
	}
	result, _ := httplog.SafeValue(
		recoverInternally, true, func() bool {
			return hook(meta)
		},
	)
	return result
}

func logEvent(log *slog.Logger, ctx context.Context, event httplog.Event, recoverInternally bool) {
	_, _ = httplog.SafeValue(
		recoverInternally, 0, func() int {
			log.LogAttrs(ctx, event.Level, event.Message, event.Attrs...)
			return 0
		},
	)
}

func traceIDs(req *http.Request) (string, string) {
	traceID := req.Header.Get("X-Trace-ID")
	spanID := req.Header.Get("X-Span-ID")
	return traceID, spanID
}
