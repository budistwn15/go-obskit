package gormx

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/budistwn15/go-obskit/sampling"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type slogGormLogger struct {
	log     *slog.Logger
	opts    Options
	sampler *sampling.Deterministic
}

func New(log *slog.Logger, opts Options) gormlogger.Interface {
	if log == nil {
		log = slog.Default()
	}
	normalized := normalizeOptions(opts)
	return &slogGormLogger{
		log:     log,
		opts:    normalized,
		sampler: sampling.NewDeterministic(normalized.SuccessSampleEvery),
	}
}

func (l *slogGormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	copied := *l
	copied.opts.Level = level
	return &copied
}

func (l *slogGormLogger) Info(ctx context.Context, msg string, data ...any) {
	if l.opts.Level < gormlogger.Info {
		return
	}
	l.safeLog(
		func() {
			l.log.InfoContext(ctx, msg)
		},
	)
}

func (l *slogGormLogger) Warn(ctx context.Context, msg string, data ...any) {
	if l.opts.Level < gormlogger.Warn {
		return
	}
	l.safeLog(
		func() {
			l.log.WarnContext(ctx, msg)
		},
	)
}

func (l *slogGormLogger) Error(ctx context.Context, msg string, data ...any) {
	if l.opts.Level < gormlogger.Error {
		return
	}
	l.safeLog(
		func() {
			l.log.ErrorContext(ctx, msg)
		},
	)
}

func (l *slogGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.opts.Level == gormlogger.Silent {
		return
	}
	elapsed := time.Since(begin)

	statement, rows := fc()
	statementTruncated := false
	if l.opts.MaxSQLLen > 0 && len(statement) > l.opts.MaxSQLLen {
		statement = statement[:l.opts.MaxSQLLen]
		statementTruncated = true
	}
	queryType := inferQueryType(statement)
	table := inferTable(statement)

	attrs := []slog.Attr{
		slog.String(FieldDBSystem, "gorm"),
		slog.Int64(FieldDurationMS, elapsed.Milliseconds()),
		slog.String(FieldLayer, "repository"),
		slog.String(FieldComponent, "gorm"),
		slog.String(FieldOperation, "db.query"),
		slog.String(FieldDBQueryType, queryType),
	}
	if table != "" {
		attrs = append(attrs, slog.String(FieldDBTable, table))
	}
	if corrID := correlation.ID(ctx); corrID != "" {
		attrs = append(attrs, slog.String(FieldCorrelation, corrID))
	}
	if l.opts.LogRowsAffected {
		attrs = append(attrs, slog.Int64(FieldDBRows, rows))
	}
	if l.opts.LogSQL {
		attrs = append(attrs, slog.String(FieldDBStatement, statement))
		attrs = append(attrs, slog.Bool(FieldDBStatementTruncated, statementTruncated))
	}

	decision := sampling.Decision{
		EventType: "db.query.complete",
		Layer:     "repository",
		Component: "gorm",
		Operation: "db.query",
		Duration:  elapsed,
	}

	if err != nil && l.opts.Level >= gormlogger.Error {
		if l.opts.IgnoreRecordNotFound && errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		decision.EventType = "db.query.error"
		decision.HasError = true
		attrs = append(attrs, slog.String(FieldDBResultStatus, "error"))
		if !l.opts.LogSQL && l.opts.LogSQLOnError {
			attrs = append(attrs, slog.String(FieldDBStatement, statement))
			attrs = append(attrs, slog.Bool(FieldDBStatementTruncated, statementTruncated))
		}
		attrs = append(
			attrs,
			slog.String(FieldEvent, "db.query.error"),
			slog.String(FieldErrorKind, classifyError(err)),
			slog.String(FieldErrorMessage, err.Error()),
		)
		if l.opts.IncludeExpectationHints {
			if expected, actual, ok := inferExpectation(err, rows); ok {
				attrs = append(attrs, slog.String(FieldErrorExpected, expected))
				attrs = append(attrs, slog.String(FieldErrorActual, actual))
			}
			if expected := Expected(ctx); expected != "" {
				attrs = append(attrs, slog.String(FieldErrorExpected, expected))
			}
		}
		if l.opts.ErrorDetailFunc != nil {
			if details, ok := safeErrorDetails(l.opts, ctx, err, statement, rows); ok && len(details) > 0 {
				attrs = append(attrs, slog.Any("error.details", details))
			}
		}
		if !shouldLog(l.sampler, l.opts.ShouldLog, decision, l.opts.RecoverInternally) {
			return
		}
		l.safeLog(
			func() {
				l.log.LogAttrs(ctx, slog.LevelError, "gorm query error", attrs...)
			},
		)
		return
	}

	if l.opts.SlowThreshold > 0 && elapsed > l.opts.SlowThreshold && l.opts.Level >= gormlogger.Warn {
		decision.EventType = "db.query.slow"
		decision.Slow = true
		attrs = append(attrs, slog.String(FieldDBResultStatus, "success"))
		if !l.opts.LogSQL && l.opts.LogSQLOnSlow {
			attrs = append(attrs, slog.String(FieldDBStatement, statement))
			attrs = append(attrs, slog.Bool(FieldDBStatementTruncated, statementTruncated))
		}
		attrs = append(
			attrs,
			slog.String(FieldEvent, "db.query.slow"),
			slog.Bool("slow", true),
			slog.Int64("threshold_ms", l.opts.SlowThreshold.Milliseconds()),
			slog.Int64("slow_threshold_ms", l.opts.SlowThreshold.Milliseconds()),
		)
		if !shouldLog(l.sampler, l.opts.ShouldLog, decision, l.opts.RecoverInternally) {
			return
		}
		l.safeLog(
			func() {
				l.log.LogAttrs(ctx, slog.LevelWarn, "gorm slow query", attrs...)
			},
		)
		return
	}

	if l.opts.LogSuccess && l.opts.Level >= gormlogger.Info {
		attrs = append(attrs, slog.String(FieldDBResultStatus, "success"))
		if !l.opts.LogSQL && l.opts.LogSQLOnSuccess {
			attrs = append(attrs, slog.String(FieldDBStatement, statement))
			attrs = append(attrs, slog.Bool(FieldDBStatementTruncated, statementTruncated))
		}
		if !shouldLog(l.sampler, l.opts.ShouldLog, decision, l.opts.RecoverInternally) {
			return
		}
		attrs = append(attrs, slog.String(FieldEvent, "db.query.complete"))
		l.safeLog(
			func() {
				l.log.LogAttrs(ctx, slog.LevelInfo, "gorm query", attrs...)
			},
		)
	}
}

func inferQueryType(statement string) string {
	s := strings.TrimSpace(statement)
	if s == "" {
		return "unknown"
	}
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.ToUpper(parts[0])
}

func inferTable(statement string) string {
	s := strings.TrimSpace(statement)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) < 2 {
		return ""
	}
	verb := strings.ToUpper(parts[0])
	switch verb {
	case "SELECT", "DELETE":
		return tokenAfter(parts, "FROM")
	case "UPDATE":
		return cleanTable(parts[1])
	case "INSERT":
		return tokenAfter(parts, "INTO")
	}
	return ""
}

func tokenAfter(parts []string, kw string) string {
	for i := 0; i < len(parts)-1; i++ {
		if strings.EqualFold(parts[i], kw) {
			return cleanTable(parts[i+1])
		}
	}
	return ""
}

func cleanTable(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"")
	s = strings.TrimSuffix(s, ";")
	s = strings.TrimSuffix(s, ",")
	s = strings.TrimPrefix(s, "(")
	return s
}

func inferExpectation(err error, rows int64) (expected string, actual string, ok bool) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "rows_affected > 0", "rows_affected = 0", true
	}
	return "", "", false
}

func safeErrorDetails(opts Options, ctx context.Context, err error, statement string, rows int64) (map[string]any, bool) {
	if opts.ErrorDetailFunc == nil {
		return nil, false
	}
	out := map[string]any{}
	ok, _ := httplog.SafeValue(opts.RecoverInternally, true, func() bool {
		v := opts.ErrorDetailFunc(ctx, err, statement, rows)
		if v == nil {
			return false
		}
		out = v
		return true
	})
	return out, ok
}

func shouldLog(
	sampler *sampling.Deterministic, hook sampling.Hook, decision sampling.Decision, recoverInternally bool,
) bool {
	if sampler != nil && !sampler.ShouldKeep(decision) {
		return false
	}
	if hook == nil {
		return true
	}
	ok, _ := httplog.SafeValue(
		recoverInternally, true, func() bool {
			return hook(decision)
		},
	)
	return ok
}

func (l *slogGormLogger) safeLog(fn func()) {
	if !l.opts.RecoverInternally {
		fn()
		return
	}
	defer func() {
		_ = recover()
	}()
	fn()
}
