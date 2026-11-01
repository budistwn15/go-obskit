package httplog

import "log/slog"

func RequestAttrs(meta RequestMeta) []slog.Attr {
	attrs := make([]slog.Attr, 0, 6)
	if meta.Method != "" {
		attrs = append(attrs, slog.String("http.request.method", meta.Method))
	}
	if meta.Scheme != "" {
		attrs = append(attrs, slog.String("url.scheme", meta.Scheme))
	}
	if meta.Host != "" {
		attrs = append(attrs, slog.String("url.domain", meta.Host))
	}
	if meta.Path != "" {
		attrs = append(attrs, slog.String("url.path", meta.Path))
	}
	if meta.Route != "" {
		attrs = append(attrs, slog.String("http.route", meta.Route))
	}
	if meta.URL != "" {
		attrs = append(attrs, slog.String("url.original", meta.URL))
	}
	if len(meta.Query) > 0 {
		attrs = append(attrs, slog.Any("http.request.query", meta.Query))
	}
	if len(meta.Headers) > 0 {
		attrs = append(attrs, slog.Any("http.request.headers", meta.Headers))
	}
	if meta.UserAgent != "" {
		attrs = append(attrs, slog.String("user_agent.original", meta.UserAgent))
	}
	if meta.AgentName != "" {
		attrs = append(attrs, slog.String("agent.name", meta.AgentName))
	}
	if meta.AgentType != "" {
		attrs = append(attrs, slog.String("agent.type", meta.AgentType))
	}
	if meta.AgentDevice != "" {
		attrs = append(attrs, slog.String("agent.device", meta.AgentDevice))
	}
	if meta.Referer != "" {
		attrs = append(attrs, slog.String("http.request.referrer", meta.Referer))
	}
	if meta.SourceAddr != "" {
		attrs = append(attrs, slog.String("source.addr", meta.SourceAddr))
	}
	if meta.SourceIP != "" {
		attrs = append(attrs, slog.String("source.ip", meta.SourceIP))
	}
	if meta.SourcePort > 0 {
		attrs = append(attrs, slog.Int("source.port", meta.SourcePort))
	}
	if meta.ClientIP != "" {
		attrs = append(attrs, slog.String("client.ip", meta.ClientIP))
	}
	if meta.XForwardedFor != "" {
		attrs = append(attrs, slog.String("http.x_forwarded_for", meta.XForwardedFor))
	}
	if meta.XRealIP != "" {
		attrs = append(attrs, slog.String("http.x_real_ip", meta.XRealIP))
	}
	if meta.TargetHost != "" {
		attrs = append(attrs, slog.String("destination.address", meta.TargetHost))
	}
	if meta.TargetPort > 0 {
		attrs = append(attrs, slog.Int("destination.port", meta.TargetPort))
	}
	if meta.RequestBody != "" {
		attrs = append(attrs, slog.String("http.request.body.content", meta.RequestBody))
		attrs = append(attrs, slog.Bool("http.request.body_truncated", meta.RequestBodyTruncated))
	}
	return attrs
}

func ResponseAttrs(meta ResponseMeta) []slog.Attr {
	attrs := []slog.Attr{
		slog.Int("http.response.status_code", meta.StatusCode),
		slog.Int64("http.response.body.bytes", meta.SizeBytes),
	}
	if len(meta.Headers) > 0 {
		attrs = append(attrs, slog.Any("http.response.headers", meta.Headers))
	}
	if meta.ResponseBody != "" {
		attrs = append(attrs, slog.String("http.response.body.content", meta.ResponseBody))
		attrs = append(attrs, slog.Bool("http.response.body_truncated", meta.ResponseBodyTruncated))
	}
	return attrs
}

func EventAttrs(meta EventMeta) []slog.Attr {
	attrs := []slog.Attr{slog.Int64("event.duration", meta.Duration.Nanoseconds())}
	if meta.Category != "" {
		attrs = append(attrs, slog.String("event.category", meta.Category))
	}
	if meta.Outcome != "" {
		attrs = append(attrs, slog.String("event.outcome", meta.Outcome))
	}
	if meta.Slow {
		attrs = append(attrs, slog.Bool("event.slow", true))
	}
	if meta.CorrelationID != "" {
		attrs = append(attrs, slog.String("correlation.id", meta.CorrelationID))
	}
	if meta.RequestID != "" {
		attrs = append(attrs, slog.String("http.request.id", meta.RequestID))
	}
	if meta.TraceID != "" {
		attrs = append(attrs, slog.String("trace.id", meta.TraceID))
	}
	if meta.SpanID != "" {
		attrs = append(attrs, slog.String("span.id", meta.SpanID))
	}
	if meta.Layer != "" {
		attrs = append(attrs, slog.String("labels.layer", meta.Layer))
	}
	if meta.Component != "" {
		attrs = append(attrs, slog.String("labels.component", meta.Component))
	}
	if meta.Operation != "" {
		attrs = append(attrs, slog.String("labels.operation", meta.Operation))
	}
	if meta.ErrorKind != "" {
		attrs = append(attrs, slog.String("error.type", meta.ErrorKind))
	}
	if meta.ErrorMessage != "" {
		attrs = append(attrs, slog.String("error.message", meta.ErrorMessage))
	}
	return attrs
}
