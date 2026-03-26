package httplog

import "log/slog"

func RequestAttrs(meta RequestMeta) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("http.method", meta.Method),
		slog.String("http.scheme", meta.Scheme),
		slog.String("http.host", meta.Host),
		slog.String("http.path", meta.Path),
		slog.String("http.route", meta.Route),
		slog.String("http.url", meta.URL),
	}
	if len(meta.Query) > 0 {
		attrs = append(attrs, slog.Any("http.query", meta.Query))
	}
	if len(meta.Headers) > 0 {
		attrs = append(attrs, slog.Any("http.request.headers", meta.Headers))
	}
	if meta.UserAgent != "" {
		attrs = append(attrs, slog.String("http.user_agent", meta.UserAgent))
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
		attrs = append(attrs, slog.String("http.referer", meta.Referer))
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
		attrs = append(attrs, slog.String("target.host", meta.TargetHost))
	}
	if meta.TargetPort > 0 {
		attrs = append(attrs, slog.Int("target.port", meta.TargetPort))
	}
	if meta.RequestBody != "" {
		attrs = append(attrs, slog.String("http.request.body", meta.RequestBody))
		attrs = append(attrs, slog.Bool("http.request.body_truncated", meta.RequestBodyTruncated))
	}
	return attrs
}

func ResponseAttrs(meta ResponseMeta) []slog.Attr {
	attrs := []slog.Attr{
		slog.Int("http.status_code", meta.StatusCode),
		slog.Int64("http.response.size_bytes", meta.SizeBytes),
	}
	if len(meta.Headers) > 0 {
		attrs = append(attrs, slog.Any("http.response.headers", meta.Headers))
	}
	if meta.ResponseBody != "" {
		attrs = append(attrs, slog.String("http.response.body", meta.ResponseBody))
		attrs = append(attrs, slog.Bool("http.response.body_truncated", meta.ResponseBodyTruncated))
	}
	return attrs
}

func EventAttrs(meta EventMeta) []slog.Attr {
	attrs := []slog.Attr{
		slog.Int64("duration_ms", meta.DurationMS),
		slog.Bool("slow", meta.Slow),
	}
	if meta.CorrelationID != "" {
		attrs = append(attrs, slog.String("correlation_id", meta.CorrelationID))
	}
	if meta.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", meta.RequestID))
	}
	if meta.TraceID != "" {
		attrs = append(attrs, slog.String("trace_id", meta.TraceID))
	}
	if meta.SpanID != "" {
		attrs = append(attrs, slog.String("span_id", meta.SpanID))
	}
	if meta.Layer != "" {
		attrs = append(attrs, slog.String("layer", meta.Layer))
	}
	if meta.Component != "" {
		attrs = append(attrs, slog.String("component", meta.Component))
	}
	if meta.Operation != "" {
		attrs = append(attrs, slog.String("operation", meta.Operation))
	}
	if meta.SlowThresholdMS > 0 {
		attrs = append(attrs, slog.Int64("threshold_ms", meta.SlowThresholdMS))
		attrs = append(attrs, slog.Int64("slow_threshold_ms", meta.SlowThresholdMS))
	}
	if meta.ErrorKind != "" {
		attrs = append(attrs, slog.String("error.kind", meta.ErrorKind))
	}
	if meta.ErrorMessage != "" {
		attrs = append(attrs, slog.String("error.message", meta.ErrorMessage))
	}
	return attrs
}
