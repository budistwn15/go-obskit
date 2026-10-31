package fiberx

import (
	"context"
	"log/slog"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/gofiber/fiber/v2"
)

func Middleware(log *slog.Logger, opts Options) fiber.Handler {
	if log == nil {
		log = slog.Default()
	}
	opts = normalizeOptions(opts)
	sampler := httplog.NewSuccessSampler(opts.SuccessSampleEvery)

	return func(c *fiber.Ctx) error {
		start := time.Now()
		anyLifecycleLog := opts.LogRequestStart || opts.LogRequestComplete || opts.LogRequestError
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}

		correlationID := c.Get(opts.CorrelationHeader)
		if correlationID == "" {
			ctx, correlationID = correlation.GetOrGenerate(ctx)
		} else {
			ctx = correlation.WithID(ctx, correlationID)
		}
		requestID := c.Get("X-Request-ID")
		traceID := c.Get("X-Trace-ID")
		spanID := c.Get("X-Span-ID")
		ctx = logger.WithMeta(
			ctx, logger.ContextMeta{
				CorrelationID: correlationID,
				RequestID:     requestID,
				TraceID:       traceID,
				SpanID:        spanID,
			},
		)
		c.SetUserContext(ctx)

		c.Request().Header.Set(opts.CorrelationHeader, correlationID)
		c.Set(opts.CorrelationHeader, correlationID)

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
			ev := httplog.BuildRequestStart(
				reqMeta, httplog.EventMeta{
					CorrelationID: correlationID,
					RequestID:     requestID,
					TraceID:       traceID,
					SpanID:        spanID,
				},
			)
			httplog.LogEvent(log, ctx, ev, opts.RecoverInternally)
		}

		err := c.Next()
		if !anyLifecycleLog {
			return err
		}

		duration := httplog.Since(start)
		statusCode := c.Response().StatusCode()
		isSlow := httplog.IsSlowRequest(duration, opts.SlowRequestThreshold)
		if err == nil && statusCode < 400 && opts.LogRequestComplete && opts.ShouldLogComplete == nil &&
			!isSlow && !sampler.ShouldLog(httplog.DecisionMeta{
			Event:     "complete",
			Response:  httplog.ResponseMeta{StatusCode: statusCode},
			EventMeta: httplog.EventMeta{Duration: duration, Slow: false},
		}) {
			return nil
		}
		if !reqMetaReady {
			reqMeta = captureRequestMeta(c, opts)
		}
		resMeta := captureResponseMeta(c, opts)
		evMeta := httplog.EventMeta{
			CorrelationID: correlationID,
			RequestID:     requestID,
			TraceID:       traceID,
			SpanID:        spanID,
			Duration:      duration,
			DurationMS:    httplog.DurationMS(duration),
			Slow:          httplog.IsSlowRequest(duration, opts.SlowRequestThreshold),
		}
		if err != nil || resMeta.StatusCode >= 500 {
			evMeta.ErrorKind = "http_error"
			if err != nil {
				evMeta.ErrorMessage = err.Error()
			}
			if opts.LogRequestError && httplog.ShouldLog(sampler, opts.ShouldLogError,
				httplog.DecisionMeta{Event: "error", Request: reqMeta, Response: resMeta, EventMeta: evMeta, Err: err},
				opts.RecoverInternally,
			) {
				httplog.LogEvent(log, ctx, httplog.BuildRequestError(reqMeta, resMeta, evMeta), opts.RecoverInternally)
			}
			return err
		}
		if !opts.LogSuccessHeaders {
			resMeta.Headers = nil
		}
		if opts.LogRequestComplete && (opts.ShouldLogComplete == nil || httplog.ShouldLog(
			sampler, opts.ShouldLogComplete,
			httplog.DecisionMeta{Event: "complete", Request: reqMeta, Response: resMeta, EventMeta: evMeta},
			opts.RecoverInternally,
		)) {
			httplog.LogEvent(log, ctx, httplog.BuildRequestComplete(reqMeta, resMeta, evMeta), opts.RecoverInternally)
		}
		return err
	}
}
