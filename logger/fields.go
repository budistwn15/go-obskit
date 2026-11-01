package logger

const (
	FieldServiceName    = "service.name"
	FieldServiceVersion = "service.version"
	FieldEnvironment    = "service.environment"
	FieldSchemaVersion  = "schema.version"
	FieldHost           = "host.name"
	FieldInstanceID     = "service.node.name"
	FieldCorrelationID  = "correlation.id"
	FieldRequestID      = "http.request.id"
	FieldTraceID        = "trace.id"
	FieldSpanID         = "span.id"
	FieldTransactionID  = "transaction.id"
	FieldLayer          = "labels.layer"
	FieldComponent      = "labels.component"
	FieldOperation      = "labels.operation"
	FieldDurationMS     = "event.duration"
)

const DefaultSchemaVersion = "1"

type ContextMeta struct {
	CorrelationID string
	RequestID     string
	TraceID       string
	SpanID        string
	TransactionID string
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
