package outbound

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/budistwn15/go-obskit/logger"
)

type transport struct {
	base    http.RoundTripper
	log     *slog.Logger
	opts    Options
	sampler *httplog.SuccessSampler
}

func NewTransport(base http.RoundTripper, log *slog.Logger, opts Options) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	if log == nil {
		log = slog.Default()
	}
	normalized := normalizeOptions(opts)
	return &transport{
		base:    base,
		log:     log,
		opts:    normalized,
		sampler: httplog.NewSuccessSampler(normalized.SuccessSampleEvery),
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return t.base.RoundTrip(req)
	}
	start := time.Now()
	ctx := req.Context()
	anyLifecycleLog := t.opts.LogRequestStart || t.opts.LogRequestComplete || t.opts.LogRequestError

	corrID := correlation.ID(ctx)
	outReq := req
	if corrID != "" {
		cloned := req.Clone(ctx)
		cloned.Header.Set(t.opts.CorrelationHeader, corrID)
		outReq = cloned
	}
	requestID := outReq.Header.Get("X-Request-ID")
	traceID, spanID := traceIDs(outReq)

	logCtx := logger.WithMeta(
		ctx, logger.ContextMeta{
			CorrelationID: corrID,
			RequestID:     requestID,
			TraceID:       traceID,
			SpanID:        spanID,
			Layer:         string(logger.LayerIntegration),
			Component:     "http_client",
			Operation:     "outbound_request",
		},
	)

	var reqMeta httplog.RequestMeta
	if anyLifecycleLog {
		needReqBody := t.opts.CaptureRequestBody && (t.opts.LogRequestStart || t.opts.LogRequestComplete || t.opts.LogRequestError)
		reqMeta, _ = captureRequest(outReq, t.opts, needReqBody)
	}
	if t.opts.LogRequestStart {
		if shouldLog(
			t.sampler, t.opts.ShouldLogStart, "start", reqMeta, httplog.ResponseMeta{}, httplog.EventMeta{}, nil,
			t.opts.RecoverInternally,
		) {
			startEvent := httplog.Event{
				Message: "outbound http request started",
				Level:   slog.LevelInfo,
				Attrs: append(
					[]slog.Attr{slog.String("event", "http.outbound.start")},
					append(
						httplog.RequestAttrs(reqMeta),
						httplog.EventAttrs(
							httplog.EventMeta{
								CorrelationID: corrID,
								RequestID:     requestID,
								TraceID:       traceID,
								SpanID:        spanID,
							},
						)...,
					)...,
				),
			}
			safeLog(
				t.opts.RecoverInternally, func() {
					t.log.LogAttrs(logCtx, startEvent.Level, startEvent.Message, startEvent.Attrs...)
				},
			)
		}
	}

	resp, err := t.base.RoundTrip(outReq)
	duration := time.Since(start)
	evMeta := httplog.EventMeta{
		CorrelationID:   corrID,
		RequestID:       requestID,
		TraceID:         traceID,
		SpanID:          spanID,
		Layer:           string(logger.LayerIntegration),
		Component:       "http_client",
		Operation:       "outbound_request",
		Duration:        duration,
		DurationMS:      httplog.DurationMS(duration),
		Slow:            httplog.IsSlowRequest(duration, t.opts.SlowThreshold),
		SlowThresholdMS: t.opts.SlowThreshold.Milliseconds(),
	}

	if err != nil {
		if t.opts.LogRequestError {
			if !shouldLog(
				t.sampler, t.opts.ShouldLogError, "error", reqMeta, httplog.ResponseMeta{}, evMeta, err,
				t.opts.RecoverInternally,
			) {
				return resp, err
			}
			evMeta.ErrorKind = classifyError(err)
			evMeta.ErrorMessage = err.Error()
			event := httplog.Event{
				Message: "outbound http request failed",
				Level:   slog.LevelError,
				Attrs: append(
					[]slog.Attr{slog.String("event", "http.outbound.error")},
					append(httplog.RequestAttrs(reqMeta), httplog.EventAttrs(evMeta)...)...,
				),
			}
			safeLog(
				t.opts.RecoverInternally, func() {
					t.log.LogAttrs(logCtx, event.Level, event.Message, event.Attrs...)
				},
			)
		}
		return resp, err
	}

	var resMeta httplog.ResponseMeta
	if t.opts.LogRequestComplete {
		needRespBody := t.opts.CaptureResponseBody && t.opts.LogErrorBodies
		resMeta, _ = captureResponse(resp, t.opts, needRespBody)
	}
	if !t.opts.LogSuccessHeaders && resMeta.StatusCode < 400 {
		resMeta.Headers = nil
	}
	if !t.opts.LogErrorHeaders && resMeta.StatusCode >= 400 {
		resMeta.Headers = nil
	}
	if !t.opts.LogErrorBodies {
		resMeta.ResponseBody = ""
		resMeta.ResponseBodyTruncated = false
	} else if resMeta.StatusCode < 400 {
		resMeta.ResponseBody = ""
		resMeta.ResponseBodyTruncated = false
	}

	if t.opts.LogRequestComplete {
		if !shouldLog(
			t.sampler, t.opts.ShouldLogComplete, "complete", reqMeta, resMeta, evMeta, nil, t.opts.RecoverInternally,
		) {
			return resp, err
		}
		event := httplog.Event{
			Message: "outbound http request completed",
			Level:   slog.LevelInfo,
			Attrs: append(
				[]slog.Attr{slog.String("event", "http.outbound.complete")},
				append(
					append(httplog.RequestAttrs(reqMeta), httplog.ResponseAttrs(resMeta)...),
					httplog.EventAttrs(evMeta)...,
				)...,
			),
		}
		safeLog(
			t.opts.RecoverInternally, func() {
				t.log.LogAttrs(logCtx, event.Level, event.Message, event.Attrs...)
			},
		)
	}

	return resp, err
}

func shouldLog(
	sampler *httplog.SuccessSampler, hook DecisionHook, event string, req httplog.RequestMeta, res httplog.ResponseMeta,
	ev httplog.EventMeta, err error, recoverInternally bool,
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
	ok, _ := httplog.SafeValue(
		recoverInternally, true, func() bool {
			return hook(meta)
		},
	)
	return ok
}

func WrapClient(client *http.Client, log *slog.Logger, opts Options) *http.Client {
	if client == nil {
		client = &http.Client{}
	}
	client.Transport = NewTransport(client.Transport, log, opts)
	return client
}
