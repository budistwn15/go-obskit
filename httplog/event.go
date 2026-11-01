package httplog

import "log/slog"

type Event struct {
	Message string
	Level   slog.Level
	Attrs   []slog.Attr
}

func BuildRequestStart(req RequestMeta, ev EventMeta) Event {
	ev.Category = "web"
	ev.Outcome = "unknown"
	attrs := joinAttrs(
		slog.String("event.action", "http.request.start"),
		RequestAttrs(NormalizeRequestMeta(req)),
		nil,
		EventAttrs(ev),
	)
	return Event{
		Message: "http request started",
		Level:   slog.LevelInfo,
		Attrs:   attrs,
	}
}

func BuildRequestComplete(req RequestMeta, res ResponseMeta, ev EventMeta) Event {
	ev.Category = "web"
	ev.Outcome = "success"
	if res.StatusCode >= 400 {
		ev.Outcome = "failure"
	}
	attrs := joinAttrs(
		slog.String("event.action", "http.request.complete"),
		RequestAttrs(NormalizeRequestMeta(req)),
		ResponseAttrs(NormalizeResponseMeta(res)),
		EventAttrs(ev),
	)
	return Event{
		Message: "http request completed",
		Level:   slog.LevelInfo,
		Attrs:   attrs,
	}
}

func BuildRequestError(req RequestMeta, res ResponseMeta, ev EventMeta) Event {
	ev.Category = "web"
	ev.Outcome = "failure"
	attrs := joinAttrs(
		slog.String("event.action", "http.request.error"),
		RequestAttrs(NormalizeRequestMeta(req)),
		ResponseAttrs(NormalizeResponseMeta(res)),
		EventAttrs(ev),
	)
	return Event{
		Message: "http request failed",
		Level:   slog.LevelError,
		Attrs:   attrs,
	}
}

func joinAttrs(first slog.Attr, request, response, event []slog.Attr) []slog.Attr {
	attrs := make([]slog.Attr, 1, 1+len(request)+len(response)+len(event))
	attrs[0] = first
	attrs = append(attrs, request...)
	attrs = append(attrs, response...)
	return append(attrs, event...)
}

func EventArgs(attrs []slog.Attr) []any {
	out := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, attr)
	}
	return out
}
