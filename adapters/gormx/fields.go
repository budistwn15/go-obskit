package gormx

const (
	FieldEvent                = "event.action"
	FieldDBSystem             = "db.system"
	FieldDBStatement          = "db.statement"
	FieldDBQueryType          = "db.operation"
	FieldDBTable              = "db.collection.name"
	FieldDBStatementTruncated = "db.statement_truncated"
	FieldDBRows               = "db.rows_affected"
	FieldDBResultStatus       = "event.outcome"
	FieldDurationMS           = "event.duration"
	FieldCorrelation          = "correlation.id"
	FieldLayer                = "labels.layer"
	FieldComponent            = "labels.component"
	FieldOperation            = "labels.operation"
	FieldErrorKind            = "error.type"
	FieldErrorMessage         = "error.message"
	FieldErrorExpected        = "error.expected"
	FieldErrorActual          = "error.actual"
)
