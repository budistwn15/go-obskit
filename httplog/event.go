package httplog

import "log/slog"

type Event struct {
	Message string
	Level   slog.Level
	Attrs   []slog.Attr
}

func BuildRequestStart(req RequestMeta, ev EventMeta) Event {
	attrs := []slog.Attr{slog.String("event", "http.request.start")}
	attrs = append(attrs, RequestAttrs(NormalizeRequestMeta(req))...)
	attrs = append(attrs, EventAttrs(ev)...)
	return Event{
		Message: "http request started",
		Level:   slog.LevelInfo,
		Attrs:   attrs,
	}
}

func BuildRequestComplete(req RequestMeta, res ResponseMeta, ev EventMeta) Event {
	attrs := []slog.Attr{slog.String("event", "http.request.complete")}
	attrs = append(attrs, RequestAttrs(NormalizeRequestMeta(req))...)
	attrs = append(attrs, ResponseAttrs(NormalizeResponseMeta(res))...)
	attrs = append(attrs, EventAttrs(ev)...)
	return Event{
		Message: "http request completed",
		Level:   slog.LevelInfo,
		Attrs:   attrs,
	}
}

func BuildRequestError(req RequestMeta, res ResponseMeta, ev EventMeta) Event {
	attrs := []slog.Attr{slog.String("event", "http.request.error")}
	attrs = append(attrs, RequestAttrs(NormalizeRequestMeta(req))...)
	attrs = append(attrs, ResponseAttrs(NormalizeResponseMeta(res))...)
	attrs = append(attrs, EventAttrs(ev)...)
	return Event{
		Message: "http request failed",
		Level:   slog.LevelError,
		Attrs:   attrs,
	}
}

func EventArgs(attrs []slog.Attr) []any {
	out := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, attr)
	}
	return out
}
