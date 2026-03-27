package gormx

import "context"
import gormlogger "gorm.io/gorm/logger"

// TracingOptions returns an opt-in verbose profile for deep query tracing.
// Use this only on targeted services/environments due higher log volume.
//
// Stability contract:
// - This preset is treated as a stable cross-version profile.
// - No default value inside this preset should change silently in minor/patch releases.
// - If a value must change, it should be announced as a breaking preset change.
func TracingOptions() Options {
	opts := DefaultOptions()
	opts.Level = gormlogger.Info
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
