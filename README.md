# go-obskit

`go-obskit` is a generic Go observability and logging toolkit built for shared use across many repositories.

It is modular: teams can adopt only what they need (logger only, HTTP adapters, outbound, DB, job logging, etc). It does not require ELK, OpenSearch, Loki, Datadog, Splunk, or any external collector. Logs go to stdout by default.

## Features

- Structured logging with `log/slog`
- Stdout-first behavior by default
- Correlation ID helpers for context propagation
- Custom field enrichment patterns for app-specific metadata
- Incoming HTTP logging abstraction (`httplog`)
- Optional incoming adapters for `net/http`, Gin, and Fiber
- Outbound HTTP observability via `http.RoundTripper`
- Optional GORM logging integration
- Job/scheduler/worker lifecycle logging
- Sensitive header/JSON redaction helpers
- Low-noise defaults with optional selective logging/sampling
- Modular adoption across different app types

## Installation

Core module:

```bash
go get github.com/budistwn15/go-obskit
go mod tidy
```

Optional adapters (pick only what you use):

```bash
go get github.com/budistwn15/go-obskit/adapters/nethttp
go get github.com/budistwn15/go-obskit/adapters/ginx
go get github.com/budistwn15/go-obskit/adapters/fiberx
go get github.com/budistwn15/go-obskit/adapters/gormx
go mod tidy
```

Note: replace `OWNER/REPO` in the badge URL above with your actual repository path.

## Environment Variables (.env)

Use [`.env.example`](/Users/budisetiawan/Documents/bri/rsp/dependencies/obskit/.env.example) as minimal starter.
For advanced knobs, see [`.env.full.example`](/Users/budisetiawan/Documents/bri/rsp/dependencies/obskit/.env.full.example).

Important:
- `obskit` **does not auto-read** `.env`.
- Application should load env and map values into `logger.Config`, adapter options, `elastic.Config`, `gormx.Options`, etc.

Minimal vars:
- `APP_NAME`
- `APP_ENV`
- `LOG_LEVEL`
- `OBSKIT_ELASTIC_ENABLED` (default `false`)
- `OBSKIT_ELASTIC_URL`
- `OBSKIT_ELASTIC_INDEX`
- `OBSKIT_ELASTIC_USERNAME` 
- `OBSKIT_ELASTIC_PASSWORD` 

For full tracing / advanced tuning:
- `OBSKIT_HTTP_FORENSIC=true`
- `OBSKIT_GORM_TRACING=true`

Example loader/config wiring:
- [`examples/config-from-env/main.go`](/Users/budisetiawan/Documents/bri/rsp/dependencies/obskit/examples/config-from-env/main.go)

Safe env injector utility (optional):

```bash
go run github.com/budistwn15/go-obskit/cmd/obskit-envsync@latest
```

Full profile:

```bash
go run github.com/budistwn15/go-obskit/cmd/obskit-envsync@latest -profile full
```

Behavior:
- If `.env.example` exists: missing obskit keys are appended.
- If `.env.example` does not exist: skip and exit success (no error).
- Existing values are preserved (no override).

## Quick Start

```go
package main

import (
	"log/slog"

	"github.com/budistwn15/go-obskit/logger"
)

func main() {
	log := logger.New(logger.Config{
		ServiceName: "my-service",
		Environment: "local",
		Level:       logger.LevelInfo,
	})

	log.Info("service started", slog.String("component", "bootstrap"))
}
```

## Usage

### logger

Use `logger.New` to initialize the base logger.

```go
log := logger.New(logger.Config{
	ServiceName:    "billing-api",
	ServiceVersion: "1.3.0",
	Environment:    "production",
	Level:          logger.LevelInfo,
	AddSource:      false,
})
```

Store/retrieve logger in context.

```go
ctx = logger.WithContext(ctx, log)
log2 := logger.FromContext(ctx, log)
log2.Info("context logger ready")
```

Add reusable fields with `logger.WithCommon`.

```go
workerLog := logger.WithCommon(log,
	slog.String("module", "invoice_worker"),
	slog.String("region", "ap-southeast-1"),
)
workerLog.Info("worker initialized")
```

### correlation

Set and read correlation IDs explicitly.

```go
ctx = correlation.WithID(ctx, "corr-123")
id := correlation.ID(ctx)
_ = id
```

Get existing or generate new ID.

```go
ctx, corrID := correlation.GetOrGenerate(ctx)
_ = corrID
```

Generate standalone ID.

```go
corrID := correlation.Generate()
_ = corrID
```

### custom fields

One-off fields:

```go
log.Info("invoice paid",
	slog.String("tenant_id", "t-1"),
	slog.String("invoice_id", "inv-9001"),
)
```

Derived logger with `.With(...)`:

```go
invoiceLog := log.With(
	slog.String("tenant_id", "t-1"),
	slog.String("feature_flag", "smart_retry"),
)
invoiceLog.Info("processing")
```

Context-scoped fields:

```go
ctx = logger.WithContextAttrs(ctx,
	slog.String("tenant_id", "t-1"),
	slog.String("request_source", "partner-api"),
)
ctx = logger.AppendContextAttrs(ctx, slog.String("invoice_id", "inv-9001"))
log.InfoContext(ctx, "context-enriched log")
```

### adapters/nethttp

Wrap standard `net/http` handlers with `nethttp.Middleware`.

```go
mux := http.NewServeMux()
mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
})

opts := nethttp.DefaultOptions()
opts.SuccessSampleEvery = 10
wrapped := nethttp.Middleware(log, opts)(mux)

// incident mode (detail tinggi, opt-in)
forensic := nethttp.ForensicOptions()
_ = nethttp.Middleware(log, forensic)(mux)

_ = http.ListenAndServe(":8080", wrapped)
```

### adapters/ginx

Use as standard Gin middleware.

```go
r := gin.New()
r.Use(ginx.Middleware(log, ginx.DefaultOptions()))
r.GET("/health", func(c *gin.Context) {
	c.Status(http.StatusOK)
})
```

### adapters/fiberx

Use as standard Fiber middleware.

```go
app := fiber.New()
app.Use(fiberx.Middleware(log, fiberx.DefaultOptions()))
app.Get("/health", func(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusOK)
})
```

### outbound HTTP logging

Wrap an existing transport/client with outbound observability.

```go
tr := outbound.NewTransport(nil, log, outbound.DefaultOptions())
client := &http.Client{Transport: tr}

// incident mode
forensicTr := outbound.NewTransport(nil, log, outbound.ForensicOptions())
_ = &http.Client{Transport: forensicTr}

req, _ := http.NewRequest(http.MethodGet, "https://example.com/health", nil)
resp, err := client.Do(req)
_ = resp
_ = err
```

### elastic (optional ELK/OpenSearch sink)

```go
elkMW := elastic.NewMiddleware(elastic.Config{
	Enabled:         true,
	ElasticURL:      "http://localhost:9200",
	ElasticIndex:    "obskit-logs",
	ElasticUsername: "elastic",
	ElasticPassword: "secret",

	IndexTimestampSuffix: true,    
	IndexTimestampLayout: "2006.01.02",
	IndexPattern:         "obskit-logs-*",

	Bootstrap:               true,
	BootstrapOnStart:        true,
	PipelineName:            "obskit-default-pipeline",
	TemplateName:            "obskit-default-template",
	ApplyPipelineToExisting: true,

	Timeout:       2 * time.Second,
	MaxRetries:    3,
	RetryBackoff:  150 * time.Millisecond,
	MaxBackoff:    2 * time.Second,
	QueueSize:     2048,
	BatchSize:     200,
	FlushInterval: 1 * time.Second,

	BlockOnQueueFull: false,
	EnableMonitor:    true,
	MonitorInterval:  15 * time.Second,
})

log := logger.New(logger.Config{
	ServiceName: "my-service",
	Environment: "production",
	Middlewares: []logger.HandlerMiddleware{elkMW.LoggerMiddleware()},
})
defer elkMW.Close(context.Background())
```

### adapters/gormx

Attach `gormx.New` as GORM logger implementation.

```go
opts := gormx.DefaultOptions()
opts.LogSQLOnError = true  // default: true
opts.LogSQLOnSlow = true   // default: true
opts.LogSuccess = false    // default: low-noise

// optional: attach app-specific query context for easier tracing
opts.ErrorDetailFunc = func(ctx context.Context, err error, statement string, rows int64) map[string]any {
	return map[string]any{
		"query_name": "GetActiveUsers",
		"expected":   "non-empty result",
		"actual_rows": rows,
	}
}

gormLog := gormx.New(log, opts)
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormLog})
_ = db
_ = err
```

### job/scheduler logging

Use `joblog.Start` and run methods for lifecycle logging.

```go
ctx, run := joblog.Start(ctx, log, joblog.Meta{
	JobName:     "daily-reconcile",
	TriggerType: "cron",
	Component:   "scheduler",
	Operation:   "daily_reconcile",
})

run.AddProcessed(100)
run.AddSucceeded(99)
run.AddFailed(1)
run.Retry(joblog.RetryMeta{Attempt: 2, MaxAttempts: 3, Reason: "timeout"})
run.End(nil)

_ = ctx
```

### errorsx

Wrap and extract layered error metadata.

```go
err := errors.New("downstream timeout")
err = errorsx.Wrap(err, errorsx.Meta{
	Code:      "E_TIMEOUT",
	Type:      "temporary",
	Layer:     "integration",
	Component: "payment_client",
	Operation: "charge",
})

if ex, ok := errorsx.Extract(err); ok {
	log.Error("wrapped error",
		slog.String("error.code", ex.Meta.Code),
		slog.String("error.layer", ex.Meta.Layer),
	)
}
```

## Practical API Reference

### logger

- `logger.New(cfg Config) *slog.Logger`: create base logger with stdout fallback.
- `logger.WithContext(ctx, log) context.Context`: put logger into context.
- `logger.FromContext(ctx, fallback) *slog.Logger`: read logger from context.
- `logger.WithCommon(log, attrs...) *slog.Logger`: derived logger with shared fields.
- `logger.WithContextAttrs(ctx, attrs...) context.Context`: replace context attrs.
- `logger.AppendContextAttrs(ctx, attrs...) context.Context`: append context attrs.
- `logger.ContextAttrs(ctx) ([]slog.Attr, bool)`: read context attrs.

### correlation

- `correlation.WithID(ctx, id) context.Context`
- `correlation.ID(ctx) string`
- `correlation.GetOrGenerate(ctx) (context.Context, string)`
- `correlation.Generate() string`

### errorsx

- `errorsx.Wrap(err, meta) error`
- `errorsx.Extract(err) (*errorsx.Error, bool)`

### incoming adapters

- `nethttp.Middleware(log, opts) func(http.Handler) http.Handler`
- `nethttp.ForensicOptions() Options`
- `ginx.Middleware(log, opts) gin.HandlerFunc`
- `ginx.ForensicOptions() Options`
- `fiberx.Middleware(log, opts) fiber.Handler`
- `fiberx.ForensicOptions() Options`

### outbound

- `outbound.NewTransport(base, log, opts) http.RoundTripper`
- `outbound.ForensicOptions() Options`
- `outbound.WrapClient(client, log, opts) *http.Client`

### gormx

- `gormx.New(log, opts) gormlogger.Interface`
- `gormx.TracingOptions() Options`
- `gormx.WithQueryName(ctx, queryName) context.Context`
- `gormx.WithExpected(ctx, expected) context.Context`

### joblog

- `joblog.Start(ctx, log, meta, opts...) (context.Context, *Run)`
- `Run.End(err)`
- `Run.Fail(err)`
- `Run.Complete()`
- `Run.Retry(meta RetryMeta)`
- `Run.SetCounts(counts Counts)`
- `Run.AddProcessed(n)`
- `Run.AddSucceeded(n)`
- `Run.AddFailed(n)`
- `Run.AddSkipped(n)`
- `Run.Logger() *slog.Logger`
- `Run.Context() context.Context`

## Custom Fields Patterns

One-off:

```go
log.Info("batch closed",
	slog.String("tenant_id", "t-1"),
	slog.String("batch_id", "batch-20260326-001"),
)
```

Derived logger:

```go
batchLog := log.With(
	slog.String("tenant_id", "t-1"),
	slog.String("feature_flag", "reconcile_v2"),
)
batchLog.Info("start")
```

Request/job scoped:

```go
ctx = logger.WithContextAttrs(ctx,
	slog.String("tenant_id", "t-1"),
	slog.String("invoice_id", "inv-9001"),
	slog.String("batch_id", "batch-20260326-001"),
)
log.InfoContext(ctx, "scoped log")
```

```go
// incoming HTTP
inOpts := nethttp.ForensicOptions()
inOpts.SuccessSampleEvery = 5

// outbound HTTP
outOpts := outbound.ForensicOptions()
outOpts.SuccessSampleEvery = 5

// gorm
dbOpts := gormx.TracingOptions()
dbOpts.SuccessSampleEvery = 10
```

## Linting

Run lint locally with `golangci-lint`:

```bash
golangci-lint run ./...
```

For adapter/examples modules:

```bash
cd adapters/nethttp
golangci-lint run ./...
```

```bash
cd adapters/ginx
golangci-lint run ./...
```

```bash
cd adapters/fiberx
golangci-lint run ./...
```

```bash
cd adapters/gormx
golangci-lint run ./...
```

```bash
cd examples
golangci-lint run ./...
```


## Example

Logger basic:

```json
{
  "time": "2026-03-26T13:12:40.011Z",
  "level": "INFO",
  "msg": "service started",
  "service_name": "billing-api",
  "service_version": "1.3.0",
  "environment": "production",
  "host": "node-a1",
  "instance_id": "billing-api-7fd6"
}
```

Logger + custom fields:

```json
{
  "time": "2026-03-26T13:12:40.500Z",
  "level": "INFO",
  "msg": "invoice paid",
  "service_name": "billing-api",
  "tenant_id": "t-1",
  "invoice_id": "inv-9001",
  "feature_flag": "smart_retry"
}
```

Incoming HTTP complete (net/http / Gin / Fiber):

```json
{
  "time": "2026-03-26T13:12:44.102Z",
  "level": "INFO",
  "msg": "http request completed",
  "event": "http.request.complete",
  "service_name": "billing-api",
  "correlation_id": "corr-7f6a",
  "http.method": "GET",
  "http.path": "/v1/invoices",
  "http.route": "/v1/invoices",
  "http.status_code": 200,
  "agent.name": "postman",
  "agent.type": "api_client",
  "agent.device": "desktop",
  "source.ip": "10.10.1.54",
  "source.port": 53718,
  "target.host": "api.internal.local",
  "target.port": 443,
  "duration_ms": 23,
  "slow": false,
  "threshold_ms": 1000
}
```

Incoming HTTP error:

```json
{
  "time": "2026-03-26T13:12:44.900Z",
  "level": "ERROR",
  "msg": "http request failed",
  "event": "http.request.error",
  "service_name": "billing-api",
  "correlation_id": "corr-7f6a",
  "http.method": "POST",
  "http.path": "/v1/charge",
  "http.status_code": 500,
  "error.kind": "http_error",
  "error.message": "Internal Server Error",
  "duration_ms": 145,
  "slow": false,
  "threshold_ms": 1000
}
```

Outbound HTTP complete:

```json
{
  "time": "2026-03-26T13:12:45.301Z",
  "level": "INFO",
  "msg": "outbound http request completed",
  "event": "http.outbound.complete",
  "service_name": "billing-api",
  "layer": "integration",
  "component": "http_client",
  "operation": "outbound_request",
  "correlation_id": "corr-7f6a",
  "http.method": "GET",
  "http.host": "profile.internal",
  "http.status_code": 200,
  "duration_ms": 18,
  "slow": false,
  "threshold_ms": 1000
}
```

Outbound HTTP error:

```json
{
  "time": "2026-03-26T13:12:45.781Z",
  "level": "ERROR",
  "msg": "outbound http request failed",
  "event": "http.outbound.error",
  "service_name": "billing-api",
  "layer": "integration",
  "component": "http_client",
  "operation": "outbound_request",
  "correlation_id": "corr-7f6a",
  "http.method": "POST",
  "http.host": "payments.internal",
  "error.kind": "context_deadline_exceeded",
  "error.message": "context deadline exceeded",
  "duration_ms": 2001,
  "slow": true,
  "threshold_ms": 1000
}
```

GORM query complete (jika `LogSuccess=true`):

```json
{
  "time": "2026-03-26T13:12:46.001Z",
  "level": "INFO",
  "msg": "gorm query",
  "event": "db.query.complete",
  "service_name": "billing-api",
  "layer": "repository",
  "component": "gorm",
  "operation": "db.query",
  "db.system": "gorm",
  "db.rows_affected": 3,
  "db.result_status": "success",
  "db.statement": "SELECT * FROM invoices WHERE status='paid'",
  "db.statement_truncated": false,
  "duration_ms": 12
}
```

GORM slow query:

```json
{
  "time": "2026-03-26T13:12:46.119Z",
  "level": "WARN",
  "msg": "gorm slow query",
  "event": "db.query.slow",
  "service_name": "billing-api",
  "layer": "repository",
  "component": "gorm",
  "operation": "db.query",
  "correlation_id": "corr-7f6a",
  "db.system": "gorm",
  "db.rows_affected": 124,
  "db.result_status": "success",
  "db.statement": "SELECT * FROM users WHERE status = 'active'",
  "db.statement_truncated": false,
  "duration_ms": 612,
  "slow": true,
  "threshold_ms": 250
}
```

GORM error:

```json
{
  "time": "2026-03-26T13:12:46.401Z",
  "level": "ERROR",
  "msg": "gorm query error",
  "event": "db.query.error",
  "service_name": "billing-api",
  "layer": "repository",
  "component": "gorm",
  "operation": "db.query",
  "db.query_type": "UPDATE",
  "db.statement": "UPDATE invoices SET status='paid' WHERE id='inv-9001'",
  "db.statement_truncated": false,
  "db.result_status": "error",
  "error.kind": "db_error",
  "error.message": "deadlock detected",
  "error.expected": "single-row update committed",
  "error.actual": "deadlock detected by database",
  "error.details": {
    "query_name": "MarkInvoicePaid",
    "retryable": true
  },
  "duration_ms": 41
}
```

Job started:

```json
{
  "time": "2026-03-26T13:12:46.700Z",
  "level": "INFO",
  "msg": "job started",
  "event": "job.started",
  "service_name": "billing-worker",
  "layer": "job",
  "job.run_id": "job_7fd1c2a3",
  "job.name": "daily-reconcile",
  "job.trigger_type": "cron"
}
```

Job completed:

```json
{
  "time": "2026-03-26T13:12:47.004Z",
  "level": "INFO",
  "msg": "job completed",
  "event": "job.completed",
  "service_name": "billing-worker",
  "layer": "job",
  "job.run_id": "job_7fd1c2a3",
  "job.name": "daily-reconcile",
  "duration_ms": 1840,
  "slow": false,
  "threshold_ms": 5000,
  "job.count.processed": 1000,
  "job.count.succeeded": 998,
  "job.count.failed": 2
}
```

Job retry:

```json
{
  "time": "2026-03-26T13:12:47.110Z",
  "level": "WARN",
  "msg": "job retry",
  "event": "job.retry",
  "service_name": "billing-worker",
  "layer": "job",
  "job.run_id": "job_7fd1c2a3",
  "job.retry_attempt": 2,
  "job.retry_max_attempts": 3,
  "job.retry_reason": "timeout",
  "job.retry_delay_ms": 30000
}
```

Job failed:

```json
{
  "time": "2026-03-26T13:12:47.220Z",
  "level": "ERROR",
  "msg": "job failed",
  "event": "job.failed",
  "service_name": "billing-worker",
  "layer": "job",
  "job.run_id": "job_7fd1c2a3",
  "duration_ms": 1840,
  "error.message": "downstream timeout",
  "error.code": "E_TIMEOUT",
  "error.layer": "integration",
  "error.component": "payment_client",
  "error.operation": "charge"
}
```

redaction:

```json
{
  "event": "http.request.complete",
  "http.request.headers": {
    "authorization": "***redacted***",
    "x-api-key": "***redacted***",
    "content-type": "application/json"
  },
  "http.request.body": "{\"username\":\"john\",\"password\":\"***redacted***\",\"token\":\"***redacted***\"}"
}
```

## Testing

Run all tests:

```bash
go test ./...
```

Run race tests:

```bash
go test -race ./...
```

For adapter submodules:

```bash
cd adapters/nethttp && go test ./...
cd ../ginx && go test ./...
cd ../fiberx && go test ./...
cd ../gormx && go test ./...
```

## License

This project is licensed under the MIT License.
See [LICENSE](LICENSE).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.
