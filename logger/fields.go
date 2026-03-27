package logger

const (
	FieldServiceName    = "service_name"
	FieldServiceVersion = "service_version"
	FieldEnvironment    = "environment"
	FieldSchemaVersion  = "schema.version"
	FieldHost           = "host"
	FieldInstanceID     = "instance_id"
	FieldCorrelationID  = "correlation_id"
	FieldRequestID      = "request_id"
	FieldTraceID        = "trace_id"
	FieldSpanID         = "span_id"
	FieldLayer          = "layer"
	FieldComponent      = "component"
	FieldOperation      = "operation"
	FieldDurationMS     = "duration_ms"
)

const DefaultSchemaVersion = "1"

type ContextMeta struct {
	CorrelationID string
	RequestID     string
	TraceID       string
	SpanID        string
	Layer         string
	Component     string
	Operation     string
	DurationMS    int64
}

type Layer string

const (
	// Layer values are optional shared conventions.
	// Applications can use their own layer values when needed.
	LayerHandler     Layer = "handler"
	LayerUsecase     Layer = "usecase"
	LayerRepository  Layer = "repository"
	LayerContract    Layer = "contract"
	LayerIntegration Layer = "integration"
	LayerJob         Layer = "job"
)
