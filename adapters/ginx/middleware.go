package ginx

import (
	"log/slog"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/gin-gonic/gin"
)

func Middleware(log *slog.Logger, opts Options) gin.HandlerFunc {
	if log == nil {
		log = slog.Default()
	}
	opts = normalizeOptions(opts)
	sampler := httplog.NewSuccessSampler(opts.SuccessSampleEvery)

	return func(c *gin.Context) {
		start := time.Now()
		anyLifecycleLog := opts.LogRequestStart || opts.LogRequestComplete || opts.LogRequestError

		ctx := c.Request.Context()
		correlationID := c.GetHeader(opts.CorrelationHeader)
		if correlationID == "" {
			ctx, correlationID = correlation.GetOrGenerate(ctx)
		} else {
			ctx = correlation.WithID(ctx, correlationID)
		}
		requestID := c.GetHeader("X-Request-ID")
		traceID := c.GetHeader("X-Trace-ID")
		spanID := c.GetHeader("X-Span-ID")

		ctx = logger.WithMeta(
			ctx, logger.ContextMeta{
				CorrelationID: correlationID,
				RequestID:     requestID,
				TraceID:       traceID,
				SpanID:        spanID,
			},
		)
		c.Request = c.Request.WithContext(ctx)
		c.Request.Header.Set(opts.CorrelationHeader, correlationID)
		c.Writer.Header().Set(opts.CorrelationHeader, correlationID)

		var reqMeta httplog.RequestMeta
		reqMetaReady := opts.LogRequestStart || (opts.CaptureRequestBody && anyLifecycleLog)
		if reqMetaReady {
			reqMeta = captureRequestMeta(c, opts)
			if body, truncated := captureRequestBody(c, opts); body != "" {
				reqMeta.RequestBody = body
				reqMeta.RequestBodyTruncated = truncated
			}
		}

		if opts.LogRequestStart && httplog.ShouldLog(sampler, opts.ShouldLogStart,
			httplog.DecisionMeta{Event: "start", Request: reqMeta}, opts.RecoverInternally,
		) {
			event := httplog.BuildRequestStart(
				reqMeta, httplog.EventMeta{
					CorrelationID: correlationID,
					RequestID:     requestID,
					TraceID:       traceID,
					SpanID:        spanID,
				},
			)
			httplog.LogEvent(log, c.Request.Context(), event, opts.RecoverInternally)
		}

		captureResponseBody := anyLifecycleLog && opts.CaptureResponseBody &&
			(opts.LogErrorBodies || opts.LogSuccessBodies)
		rw := newResponseWriter(c.Writer, captureResponseBody, opts.MaxBodyBytes)
		c.Writer = rw
		c.Next()
		if !anyLifecycleLog {
			return
		}

		duration := httplog.Since(start)
		statusCode := rw.Status()
		isSlow := httplog.IsSlowRequest(duration, opts.SlowRequestThreshold)
		if len(c.Errors) == 0 && statusCode < 400 && opts.LogRequestComplete && opts.ShouldLogComplete == nil &&
			!isSlow && !sampler.ShouldLog(httplog.DecisionMeta{
			Event:     "complete",
			Response:  httplog.ResponseMeta{StatusCode: statusCode},
			EventMeta: httplog.EventMeta{Duration: duration, Slow: false},
		}) {
			return
		}
		if !reqMetaReady {
			reqMeta = captureRequestMeta(c, opts)
		}
		resMeta := captureResponseMeta(c, rw, opts)
		evMeta := httplog.EventMeta{
			CorrelationID: correlationID,
			RequestID:     requestID,
			TraceID:       traceID,
			SpanID:        spanID,
			Duration:      duration,
			DurationMS:    httplog.DurationMS(duration),
			Slow:          httplog.IsSlowRequest(duration, opts.SlowRequestThreshold),
		}

		if len(c.Errors) > 0 || resMeta.StatusCode >= 500 {
			evMeta.ErrorKind = "http_error"
			if len(c.Errors) > 0 {
				evMeta.ErrorMessage = c.Errors.String()
			}
			if opts.LogRequestError && httplog.ShouldLog(sampler, opts.ShouldLogError,
				httplog.DecisionMeta{Event: "error", Request: reqMeta, Response: resMeta, EventMeta: evMeta},
				opts.RecoverInternally,
			) {
				httplog.LogEvent(
					log, c.Request.Context(), httplog.BuildRequestError(reqMeta, resMeta, evMeta),
					opts.RecoverInternally,
				)
				return
			}
		}

		if !opts.LogSuccessHeaders && resMeta.StatusCode < 400 {
			resMeta.Headers = nil
		}
		if opts.LogRequestComplete && (opts.ShouldLogComplete == nil || httplog.ShouldLog(
			sampler, opts.ShouldLogComplete,
			httplog.DecisionMeta{Event: "complete", Request: reqMeta, Response: resMeta, EventMeta: evMeta},
			opts.RecoverInternally,
		)) {
			httplog.LogEvent(
				log, c.Request.Context(), httplog.BuildRequestComplete(reqMeta, resMeta, evMeta), opts.RecoverInternally,
			)
		}
	}
}
