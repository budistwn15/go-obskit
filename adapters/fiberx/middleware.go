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
		
		reqMeta := captureRequestMeta(c, opts)
		if body, truncated := captureRequestBody(c, opts); body != "" {
			reqMeta.RequestBody = body
			reqMeta.RequestBodyTruncated = truncated
		}
		
		if opts.LogRequestStart && shouldLog(
			sampler, opts.ShouldLogStart, "start", reqMeta, httplog.ResponseMeta{}, httplog.EventMeta{}, nil,
			opts.RecoverInternally,
		) {
			ev := httplog.BuildRequestStart(
				reqMeta, httplog.EventMeta{
					CorrelationID: correlationID,
					RequestID:     requestID,
					TraceID:       traceID,
					SpanID:        spanID,
				},
			)
			logEvent(log, ctx, ev, opts.RecoverInternally)
		}
		
		err := c.Next()
		
		duration := httplog.Since(start)
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
			if opts.LogRequestError && shouldLog(
				sampler, opts.ShouldLogError, "error", reqMeta, resMeta, evMeta, err, opts.RecoverInternally,
			) {
				logEvent(log, ctx, httplog.BuildRequestError(reqMeta, resMeta, evMeta), opts.RecoverInternally)
			}
			return err
		}
		if !opts.LogSuccessHeaders {
			resMeta.Headers = nil
		}
		if opts.LogRequestComplete && shouldLog(
			sampler, opts.ShouldLogComplete, "complete", reqMeta, resMeta, evMeta, nil, opts.RecoverInternally,
		) {
			logEvent(log, ctx, httplog.BuildRequestComplete(reqMeta, resMeta, evMeta), opts.RecoverInternally)
		}
		return err
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
