package httplog

import "time"

type RequestMeta struct {
	Method string
	Scheme string
	Host   string
	Path   string
	Route  string
	URL    string

	Query   map[string]any
	Headers map[string]any

	UserAgent  string
	Referer    string
	ClientIP   string
	SourceIP   string
	SourcePort int
	SourceAddr string

	XForwardedFor string
	XRealIP       string

	TargetHost string
	TargetPort int

	AgentName   string
	AgentType   string
	AgentDevice string

	RequestBody          string
	RequestBodyTruncated bool
}

type ResponseMeta struct {
	StatusCode int
	SizeBytes  int64

	Headers map[string]any

	ResponseBody          string
	ResponseBodyTruncated bool
}

type EventMeta struct {
	CorrelationID string
	RequestID     string
	TraceID       string
	SpanID        string

	Layer     string
	Component string
	Operation string

	Duration        time.Duration
	DurationMS      int64
	Slow            bool
	SlowThresholdMS int64

	ErrorKind    string
	ErrorMessage string
}

type DecisionMeta struct {
	Event     string
	Request   RequestMeta
	Response  ResponseMeta
	EventMeta EventMeta
	Err       error
}
