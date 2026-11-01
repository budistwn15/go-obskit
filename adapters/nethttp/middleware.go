package nethttp

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/budistwn15/go-obskit/v2/correlation"
	"github.com/budistwn15/go-obskit/v2/httplog"
	"github.com/budistwn15/go-obskit/v2/logger"
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

				shouldCaptureReqBody := opts.CaptureRequestBody && anyLifecycleLog
				var requestMeta httplog.RequestMeta
				requestMetaReady := opts.LogRequestStart || shouldCaptureReqBody
				if requestMetaReady {
					requestMeta = requestMetaFromRequest(req, opts)
					bodyCapture := captureRequestBody(req, opts, shouldCaptureReqBody)
					if bodyCapture.value != "" {
						requestMeta.RequestBody = bodyCapture.value
						requestMeta.RequestBodyTruncated = bodyCapture.truncated
					}
				}

				if opts.LogRequestStart && httplog.ShouldLog(sampler, opts.ShouldLogStart, httplog.DecisionMeta{
					Event: "start", Request: requestMeta,
				}, opts.RecoverInternally) {
					event := httplog.BuildRequestStart(
						requestMeta, eventMeta(0, opts.SlowRequestThreshold, correlationID, requestID, traceID, spanID),
					)
					httplog.LogEvent(log, ctx, event, opts.RecoverInternally)
				}

				captureRespBody := anyLifecycleLog && opts.CaptureResponseBody &&
					(opts.LogErrorBodies || opts.LogSuccessBodies)
				rw := newResponseWriter(w, captureRespBody, opts.MaxBodyBytes)
				next.ServeHTTP(rw, req)

				duration := httplog.Since(start)
				isSlow := httplog.IsSlowRequest(duration, opts.SlowRequestThreshold)
				if rw.status < 400 && opts.LogRequestComplete && opts.ShouldLogComplete == nil && !isSlow &&
					!sampler.ShouldLog(httplog.DecisionMeta{
						Event:     "complete",
						Response:  httplog.ResponseMeta{StatusCode: rw.status},
						EventMeta: httplog.EventMeta{Duration: duration, Slow: false},
					}) {
					return
				}
				if !requestMetaReady {
					requestMeta = requestMetaFromRequest(req, opts)
				}
				responseMeta := responseMetaFromWriter(rw, opts)
				shouldLogRespBody := (rw.status >= 400 && opts.LogErrorBodies) || (rw.status < 400 && opts.LogSuccessBodies)
				if captureRespBody && shouldLogRespBody && rw.body.Len() > 0 {
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

				if rw.status >= 500 && opts.LogRequestError && httplog.ShouldLog(sampler, opts.ShouldLogError,
					httplog.DecisionMeta{Event: "error", Request: requestMeta, Response: responseMeta, EventMeta: evMeta},
					opts.RecoverInternally,
				) {
					event := httplog.BuildRequestError(requestMeta, responseMeta, evMeta)
					httplog.LogEvent(log, ctx, event, opts.RecoverInternally)
					return
				}
				if opts.LogRequestComplete && (opts.ShouldLogComplete == nil || httplog.ShouldLog(
					sampler, opts.ShouldLogComplete,
					httplog.DecisionMeta{Event: "complete", Request: requestMeta, Response: responseMeta, EventMeta: evMeta},
					opts.RecoverInternally,
				)) {
					event := httplog.BuildRequestComplete(requestMeta, responseMeta, evMeta)
					httplog.LogEvent(log, ctx, event, opts.RecoverInternally)
				}
			},
		)
	}
}

func traceIDs(req *http.Request) (string, string) {
	traceID := req.Header.Get("X-Trace-ID")
	spanID := req.Header.Get("X-Span-ID")
	return traceID, spanID
}
