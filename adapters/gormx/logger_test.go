package gormx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/logger"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func parseLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid log line: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestSlowQueryDetection(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:         gormlogger.Warn,
			SlowThreshold: 10 * time.Millisecond,
			LogSQL:        true,
			MaxSQLLen:     1024,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now().Add(-20*time.Millisecond), func() (string, int64) {
			return "SELECT * FROM users", 3
		}, nil,
	)

	logs := parseLines(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("expected slow query log")
	}
	last := logs[len(logs)-1]
	if last[FieldEvent] != "db.query.slow" {
		t.Fatalf("expected slow event, got=%v", last[FieldEvent])
	}
	if last[FieldDBStatement] == nil {
		t.Fatalf("expected db.statement on slow log")
	}
	if last[FieldDBQueryType] != "SELECT" {
		t.Fatalf("expected query type SELECT, got=%v", last[FieldDBQueryType])
	}
	if last[FieldDBTable] != "users" {
		t.Fatalf("expected db.table users, got=%v", last[FieldDBTable])
	}
	if slow, ok := last["slow"].(bool); !ok || !slow {
		t.Fatalf("expected slow=true, got=%v", last["slow"])
	}
	if _, ok := last["slow_threshold_ms"]; !ok {
		t.Fatalf("expected slow_threshold_ms field")
	}
	if _, ok := last["threshold_ms"]; !ok {
		t.Fatalf("expected threshold_ms field")
	}
}

func TestErrorLogging(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:  gormlogger.Error,
			LogSQL: true,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM invoices WHERE id=1", -1
		}, errors.New("db down"),
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	if last[FieldEvent] != "db.query.error" {
		t.Fatalf("expected db.query.error")
	}
	if last[FieldDBStatement] != "SELECT * FROM invoices WHERE id=1" {
		t.Fatalf("expected query in error log, got=%v", last[FieldDBStatement])
	}
	if last[FieldDBQueryType] != "SELECT" {
		t.Fatalf("expected query type SELECT")
	}
	if last[FieldDBTable] != "invoices" {
		t.Fatalf("expected db.table invoices, got=%v", last[FieldDBTable])
	}
	if _, ok := last[FieldDBFingerprint].(string); !ok {
		t.Fatalf("expected db.fingerprint in error log")
	}
	if last[FieldDBSystem] != "sql" {
		t.Fatalf("expected default db.system=sql, got=%v", last[FieldDBSystem])
	}
}

func TestDBSystemConfiguredFromOptionsAndContext(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:    gormlogger.Error,
			DBSystem: "postgresql",
			LogSQL:   true,
		},
	).(gormlogger.Interface)

	ctx := WithDBSystem(context.Background(), "mysql")
	gl.Trace(
		ctx, time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE id=1", -1
		}, errors.New("db down"),
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	if last[FieldDBSystem] != "mysql" {
		t.Fatalf("expected ctx db.system override=mysql, got=%v", last[FieldDBSystem])
	}
}

func TestWhereExtraction(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:                      gormlogger.Error,
			LogSQL:                     true,
			IncludeWhereDetails:        true,
			RedactWhereSensitiveValues: true,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE id = 42 AND email = 'john@example.com' AND status = 'active'", -1
		}, errors.New("db down"),
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	cols, ok := last[FieldDBWhereColumns].([]any)
	if !ok || len(cols) == 0 {
		t.Fatalf("expected db.where.columns")
	}
	values, ok := last[FieldDBWhereValues].(map[string]any)
	if !ok {
		t.Fatalf("expected db.where.values")
	}
	if values["id"] != "42" {
		t.Fatalf("expected id value 42, got=%v", values["id"])
	}
	if values["email"] != "***redacted***" {
		t.Fatalf("expected email redacted, got=%v", values["email"])
	}
}

func TestSQLTruncation(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:         gormlogger.Error,
			LogSQL:        true,
			MaxSQLLen:     8,
			SlowThreshold: 1 * time.Hour,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM very_long_table_name", 0
		}, errors.New("x"),
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	if stmt, ok := last[FieldDBStatement].(string); !ok || len(stmt) > 8 {
		t.Fatalf("expected truncated sql")
	}
}

func TestContextCorrelationPropagation(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:  gormlogger.Error,
			LogSQL: false,
		},
	).(gormlogger.Interface)

	ctx := correlation.WithID(context.Background(), "corr-db-1")
	gl.Trace(ctx, time.Now(), func() (string, int64) { return "SELECT 1", 1 }, errors.New("x"))

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	if last[FieldCorrelation] != "corr-db-1" {
		t.Fatalf("expected correlation_id in log")
	}
}

func TestLowNoiseDefaults(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(slog, DefaultOptions()).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT 1", 1
		}, nil,
	)
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("default should not log successful fast query")
	}
}

func TestIgnoreRecordNotFoundByDefault(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(slog, DefaultOptions()).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE id=1", 0
		}, gorm.ErrRecordNotFound,
	)
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("record not found should be ignored by default")
	}
}

func TestErrorExpectationHint(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:                   gormlogger.Error,
			IgnoreRecordNotFound:    false,
			LogSQL:                  false,
			IncludeExpectationHints: true,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE id=1", 0
		}, gorm.ErrRecordNotFound,
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	if last[FieldErrorExpected] == nil || last[FieldErrorActual] == nil {
		t.Fatalf("expected expectation hint fields on record not found")
	}
}

func TestErrorDetailHook(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level: gormlogger.Error,
			ErrorDetailFunc: func(ctx context.Context, err error, statement string, rows int64) map[string]any {
				return map[string]any{
					"query_name":  "GetActiveUsers",
					"expected":    "non-empty result",
					"actual_rows": rows,
				}
			},
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE status='active'", 0
		}, errors.New("db down"),
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	detail, ok := last["error.details"].(map[string]any)
	if !ok || detail["query_name"] != "GetActiveUsers" {
		t.Fatalf("expected error.details from hook, got=%v", last["error.details"])
	}
}

func TestSuccessSampling(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:              gormlogger.Info,
			LogSuccess:         true,
			SuccessSampleEvery: 3,
		},
	).(gormlogger.Interface)

	for i := 0; i < 2; i++ {
		gl.Trace(
			context.Background(), time.Now(), func() (string, int64) {
				return "SELECT 1", 1
			}, nil,
		)
	}
	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("first two success logs should be sampled out")
	}
	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT 1", 1
		}, nil,
	)
	logs := parseLines(t, buf.String())
	if len(logs) == 0 || logs[len(logs)-1][FieldEvent] != "db.query.complete" {
		t.Fatalf("third success log should pass sampling")
	}
}

func TestSuccessLogsQueryWithLogSQLOnSuccess(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:           gormlogger.Info,
			LogSuccess:      true,
			LogSQL:          false,
			LogSQLOnSuccess: true,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users", 2
		}, nil,
	)

	logs := parseLines(t, buf.String())
	last := logs[len(logs)-1]
	if last[FieldDBStatement] == nil {
		t.Fatalf("expected db.statement on success when LogSQLOnSuccess=true")
	}
	if last[FieldDBResultStatus] != "success" {
		t.Fatalf("expected db.result_status=success")
	}
}

func TestTracingOptionsPreset(t *testing.T) {
	opts := TracingOptions()
	if opts.Level != gormlogger.Info {
		t.Fatalf("expected Level=Info, got %v", opts.Level)
	}
	if !opts.LogSuccess {
		t.Fatalf("tracing preset should enable success logs")
	}
	if opts.LogSQL {
		t.Fatalf("tracing preset should keep base LogSQL=false")
	}
	if !opts.LogSQLOnSuccess || !opts.LogSQLOnError || !opts.LogSQLOnSlow {
		t.Fatalf("tracing preset should enable SQL capture on success/error/slow")
	}
	if !opts.IncludeExpectationHints {
		t.Fatalf("tracing preset should include expectation hints")
	}
	if opts.ErrorDetailFunc == nil {
		t.Fatalf("tracing preset should include default error detail func")
	}
}

func TestFingerprintStableAcrossDifferentLiteralValues(t *testing.T) {
	var buf bytes.Buffer
	slog := logger.New(logger.Config{ServiceName: "svc", Environment: "production", Output: &buf})
	gl := New(
		slog, Options{
			Level:      gormlogger.Error,
			LogSuccess: false,
		},
	).(gormlogger.Interface)

	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE id=1 AND status='active'", 1
		}, errors.New("x1"),
	)
	gl.Trace(
		context.Background(), time.Now(), func() (string, int64) {
			return "SELECT * FROM users WHERE id=999 AND status='inactive'", 1
		}, errors.New("x2"),
	)

	logs := parseLines(t, buf.String())
	if len(logs) < 2 {
		t.Fatalf("expected two logs")
	}
	fp1, _ := logs[len(logs)-2][FieldDBFingerprint].(string)
	fp2, _ := logs[len(logs)-1][FieldDBFingerprint].(string)
	if fp1 == "" || fp2 == "" {
		t.Fatalf("expected db.fingerprint on both logs")
	}
	if fp1 != fp2 {
		t.Fatalf("expected same fingerprint for same query shape, got %s vs %s", fp1, fp2)
	}
}

func TestDefaultErrorDetailFuncReadsContext(t *testing.T) {
	ctx := WithQueryName(context.Background(), "GetInvoicesByStatus")
	ctx = WithExpected(ctx, "rows_affected > 0")
	detail := DefaultErrorDetailFunc(ctx, errors.New("x"), "SELECT 1", 0)
	if detail["query_name"] != "GetInvoicesByStatus" {
		t.Fatalf("expected query_name from context")
	}
	if detail["expected"] != "rows_affected > 0" {
		t.Fatalf("expected expected from context")
	}
}
