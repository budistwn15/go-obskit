package gormx

import "context"

// TracingOptions returns an opt-in verbose profile for deep query tracing.
// Use this only on targeted services/environments due higher log volume.
func TracingOptions() Options {
	opts := DefaultOptions()
	opts.LogSuccess = true
	opts.LogSQL = false
	opts.LogSQLOnSuccess = true
	opts.LogSQLOnError = true
	opts.LogSQLOnSlow = true
	opts.IncludeExpectationHints = true
	opts.ErrorDetailFunc = DefaultErrorDetailFunc
	return opts
}

func DefaultErrorDetailFunc(ctx context.Context, err error, statement string, rows int64) map[string]any {
	out := map[string]any{
		"actual_rows": rows,
	}
	if qn := QueryName(ctx); qn != "" {
		out["query_name"] = qn
	}
	if ex := Expected(ctx); ex != "" {
		out["expected"] = ex
	}
	return out
}
