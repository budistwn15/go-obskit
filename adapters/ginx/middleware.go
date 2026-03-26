package ginx

import (
	"context"
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
		
		reqMeta := captureRequestMeta(c, opts)
		if body, truncated := captureRequestBody(c, opts); body != "" {
			reqMeta.RequestBody = body
			reqMeta.RequestBodyTruncated = truncated
		}
		
		if opts.LogRequestStart && shouldLog(
			sampler, opts.ShouldLogStart, "start", reqMeta, httplog.ResponseMeta{}, httplog.EventMeta{}, nil,
			opts.RecoverInternally,
		) {
			event := httplog.BuildRequestStart(
				reqMeta, httplog.EventMeta{
					CorrelationID: correlationID,
					RequestID:     requestID,
					TraceID:       traceID,
					SpanID:        spanID,
				},
			)
			logEvent(log, c.Request.Context(), event, opts.RecoverInternally)
		}
		
		rw := newResponseWriter(c.Writer, opts.CaptureResponseBody, opts.MaxBodyBytes)
		c.Writer = rw
		c.Next()
		
		duration := httplog.Since(start)
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
			if opts.LogRequestError && shouldLog(
				sampler, opts.ShouldLogError, "error", reqMeta, resMeta, evMeta, nil, opts.RecoverInternally,
			) {
				logEvent(
					log, c.Request.Context(), httplog.BuildRequestError(reqMeta, resMeta, evMeta),
					opts.RecoverInternally,
				)
				return
			}
		}
		
		if !opts.LogSuccessHeaders && resMeta.StatusCode < 400 {
			resMeta.Headers = nil
		}
		if opts.LogRequestComplete && shouldLog(
			sampler, opts.ShouldLogComplete, "complete", reqMeta, resMeta, evMeta, nil, opts.RecoverInternally,
		) {
			logEvent(
				log, c.Request.Context(), httplog.BuildRequestComplete(reqMeta, resMeta, evMeta), opts.RecoverInternally,
			)
		}
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
