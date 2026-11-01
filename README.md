# go-obskit

Small, stdout-first structured logging for Go 1.26.

- Default path: JSON lines to stdout, no goroutine, no network.
- Optional delivery: bounded asynchronous HTTP, Elasticsearch/OpenSearch, or Loki.
- Optional instrumentation: net/http, Gin, Fiber, GORM, outbound HTTP, jobs.
- Optional APM correlation: OpenTelemetry in an independent module.

## Requirements

- Go 1.26 or newer.
- A stdout collector such as Filebeat, Fluent Bit, or Logstash is recommended for production.
- Direct HTTP delivery is optional. Applications using it must call `Close` during shutdown.

## Install

Install the core package:

```bash
go get github.com/budistwn15/go-obskit/v2
```

Framework adapters are independent Go modules. Install only the adapters used by the application:

```bash
go get github.com/budistwn15/go-obskit/adapters/nethttp/v2
go get github.com/budistwn15/go-obskit/adapters/ginx/v2
go get github.com/budistwn15/go-obskit/adapters/fiberx/v2
go get github.com/budistwn15/go-obskit/adapters/gormx/v2
go get github.com/budistwn15/go-obskit/adapters/grpcx/v2
go get github.com/budistwn15/go-obskit/adapters/otel/v2
```

`outbound` and `joblog` are part of the core module. They need no separate installation.

## Basic Usage

Create one logger during application startup, then pass it to handlers, services, repositories, and workers:

```go
package main

import (
	"log/slog"

	obskit "github.com/budistwn15/go-obskit/v2"
)

func main() {
	log := obskit.New(obskit.Config{
		Service:     "billing-api",
		Version:     "1.0.0",
		Environment: "production",
	})
	log.Info("ready", slog.String("region", "ap-southeast-1"))
}
```

`obskit.New` writes JSON lines to stdout. It starts no goroutine and makes no network request. This is the recommended setup when Filebeat, Fluent Bit, or Logstash already collects application output.

Output is one JSON object per line. Stdout keeps `log/slog` keys `time`, `level`, and `msg` because renaming them adds an allocation to every call. Application fields follow one ECS-compatible contract:

| Scope | Fields |
| --- | --- |
| Common | `service.*`, `event.action`, `event.category`, `event.outcome`, `event.duration` |
| Correlation | `trace.id`, `span.id`, `transaction.id`, `correlation.id` |
| REST/outbound | `http.*`, `url.*`, `user_agent.*`, `source.*`, `destination.*` |
| gRPC | `rpc.system`, `rpc.service`, `rpc.method`, `rpc.grpc.status_code` |
| Database | `db.system`, `db.operation`, `db.collection.name` |
| Scheduler | `job.name`, `job.run_id`, `job.*` |
| Errors | `error.type`, `error.message`, `error.code` |

`event.duration` and retry delays use nanoseconds, matching ECS. Domain fields appear only when relevant.

## Collector Setup

Stdout JSON works with Filebeat, Fluent Bit, or Logstash. Configure the collector to read one JSON object per line; do not parse nested JSON strings automatically.

Filebeat:

```yaml
filebeat.inputs:
  - type: filestream
    paths: [/var/log/app/*.log]
    parsers:
      - ndjson:
          target: ""
          add_error_key: true
```

Logstash:

```text
input { stdin { codec => json_lines } }
output { elasticsearch { hosts => ["http://elasticsearch:9200"] } }
```

`collector_compat_test.go` verifies newline framing and stable field types.

## Direct Delivery

Use direct delivery only when no stdout collector exists. Runtime owns shutdown:

```go
sink := httpsink.NewMiddleware(httpsink.Config{
	Enabled:  true,
	Endpoint: "https://collector.example.com/logs",
	APIKey:   os.Getenv("LOG_API_KEY"),
})
runtime := obskit.Open(obskit.Config{Service: "billing-api"}, sink)
defer runtime.Close(context.Background())

log := runtime.Logger()
log.Info("ready")
```

Available providers:

| Package | Target |
| --- | --- |
| `httpsink` | Generic NDJSON or JSON-array HTTP endpoint |
| `elastic` | Elasticsearch/OpenSearch bulk API |
| `lokisink` | Loki push API |

Delivery behavior:

- Queue and batch count are bounded.
- Records default to 64 KiB maximum and batches to 1 MiB maximum.
- Enqueue is non-blocking by default; full queues increment `Dropped`.
- Retry waits observe shutdown cancellation.
- Only transport errors, HTTP 429, and HTTP 5xx are retried.
- HTTP 4xx responses are permanent failures.
- Collector response bodies and credentials are omitted from sink errors.
- Elasticsearch Bulk responses with item failures are counted as failed, even when HTTP status is 200.
- `Close(ctx)` stops intake, flushes accepted records, cancels in-flight work, and waits for the worker.
- Stdout logging remains available when delivery is slow, down, or saturated.

Inspect counters with `sink.Stats()`: `Enqueued`, `Dropped`, `Oversized`, `Sent`, `Failed`, and `Retried`.

Mutable `slog.Any` values are snapshotted before asynchronous delivery. Simple scalar records keep the zero-allocation enqueue path.

## Adapter Guide

Adapters add protocol-specific ECS fields without changing business handlers. Framework dependencies stay outside the core module.

| Integration | Package | Use when | Main entry point |
| --- | --- | --- | --- |
| Standard HTTP server | `adapters/nethttp` | Application uses `net/http`, Chi, or another `http.Handler` router | `nethttp.Middleware` |
| Gin server | `adapters/ginx` | Application uses Gin | `ginx.Middleware` |
| Fiber server | `adapters/fiberx` | Application uses Fiber | `fiberx.Middleware` |
| gRPC server/client | `adapters/grpcx` | Application uses unary or streaming gRPC | gRPC interceptors |
| GORM database | `adapters/gormx` | Application uses GORM | `gormx.New` |
| OpenTelemetry correlation | `adapters/otel` | Logs must inherit active trace/span IDs | `otel.WithTrace` |
| Outbound HTTP | `outbound` | Application calls another HTTP service | `outbound.WrapClient` |
| Scheduler/worker | `joblog` | Cron, queue consumer, batch, or background job needs lifecycle logs | `joblog.Start` |

Default adapters capture metadata only. Headers, query values, bodies, SQL values, and gRPC messages are not captured by default.

### net/http

Wrap the final application handler:

```go
handler := nethttp.Middleware(log, nethttp.DefaultOptions())(mux)
http.ListenAndServe(":8080", handler)
```

Emits request method, route/path, response status and size, duration, client/server address, user agent, and correlation fields.

### Gin

```go
router.Use(ginx.Middleware(log, ginx.DefaultOptions()))
```

Uses Gin route templates, so logs group `/users/123` under a stable route such as `/users/:id`.

### Fiber

```go
app.Use(fiberx.Middleware(log, fiberx.DefaultOptions()))
```

Uses Fiber route templates and emits the same HTTP schema as net/http and Gin.

Default options do not capture headers, query values, or bodies. `ForensicOptions()` is explicit and should be limited to incident debugging.

### Outbound HTTP

```go
client := outbound.WrapClient(http.DefaultClient, log, outbound.DefaultOptions())
```

Emits destination, method, status, duration, and error fields. The wrapper clones the client and request only when propagation requires it. Caller-owned clients, requests, and bodies remain usable.

### GORM

```go
gormLogger := gormx.New(log, gormx.DefaultOptions())
db, err := gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
```

Emits database system, operation, collection/table, rows affected, duration, and error fields. Fast successful queries are ignored by default and do not call GORM's SQL callback. SQL value extraction, WHERE parsing, and query fingerprinting are not provided.

### gRPC

```go
server := grpc.NewServer(
	grpc.UnaryInterceptor(grpcx.UnaryServerInterceptor(log)),
	grpc.StreamInterceptor(grpcx.StreamServerInterceptor(log)),
)
```

Client interceptors are `grpcx.UnaryClientInterceptor` and `grpcx.StreamClientInterceptor`. Server and client interceptors emit service, method, status code, direction, duration, correlation, and error fields. Request and response messages are never captured.

### Scheduler and Workers

`joblog` works with cron libraries, queue consumers, batch commands, and custom worker loops:

```go
ctx, run := joblog.Start(ctx, log, joblog.Meta{
	JobName:     "sync-users",
	TriggerType: "cron",
	Component:   "users",
	Operation:   "sync",
})

run.AddProcessed(100)
run.AddSucceeded(100)
run.End(nil)
```

Pass an error to `run.End(err)` for a failed run. The run emits consistent `job.*`, `event.*`, duration, correlation, and error fields.

### OpenTelemetry Correlation

OpenTelemetry stays outside the root dependency graph:

```go
ctx = otel.WithTrace(ctx)
log.InfoContext(ctx, "request complete")
```

An active span adds `trace.id` and `span.id`. No active span returns the original context in about 10 ns with zero allocations. OpenTelemetry has no transaction ID; vendor integrations can use dependency-free `obskit.WithTrace`.

The package correlates logs only. It does not create or export traces or metrics.

## Configuration

`go-obskit` does not read `.env` files. Applications own configuration loading and pass explicit values to `obskit.Config` or provider configs.

Stdout defaults:

| Behavior | Default |
| --- | --- |
| Format | JSON lines |
| Output | `os.Stdout` |
| Level | info |
| Direct delivery | disabled |
| Network calls | none |
| Background goroutines | none |
| Header/query/body capture | disabled |
| Sampling | disabled |

## v1 to v2 Migration

| Removed v1 API | v2 replacement |
| --- | --- |
| `github.com/budistwn15/go-obskit` | `github.com/budistwn15/go-obskit/v2` |
| `github.com/budistwn15/go-obskit/adapters/<name>` | `github.com/budistwn15/go-obskit/adapters/<name>/v2` |
| `logger.New` for common setup | `obskit.New` |
| manual middleware and sink close | `obskit.Open` and `Runtime.Close` |
| `httpin` | `adapters/nethttp` |
| `httpout` | `outbound` |
| `sinkenv` | explicit provider config |
| `configenv`, `cmd/obskit-envsync` | application-owned config |
| Elastic bootstrap and monitor | deployment tooling and platform health checks |
| `ElasticURL`, `ElasticIndex`, `ElasticUsername`, `ElasticPassword` | `Endpoint`, `Index`, `Username`, `Password` |
| `EnableMonitor`, `MonitorInterval`, `MonitorPath`, `OnMonitor`, `ConnectionLogAllChecks` | external health monitoring |
| `Bootstrap`, `BootstrapOnStart`, `PipelineName`, `TemplateName`, `ApplyPipelineToExisting` | deployment-managed templates and pipelines |
| `IndexTimestampSuffix`, `IndexTimestampLayout`, `IndexPattern` | explicit `Index` routing |
| `ExpandJSONFields` | explicit structured `slog` attributes |
| `logger.WithCommon` | `logger.With` |
| `logger.WithContext` | `logger.WithLogger` |
| GORM `LogSQLArgs`, `IncludeWhereDetails`, `MaxWhereConditions`, `RedactWhereSensitiveValues` | SQL values are never captured |
| `db.fingerprint`, `db.where.*`, `db.query_type`, `db.table` | `db.operation`, `db.collection.name` |
| `duration_ms`, `threshold_ms`, `slow_threshold_ms` | nanosecond `event.duration` |
| `http.method`, `http.status_code`, `http.path` | `http.request.method`, `http.response.status_code`, `url.path` |
| `service_name`, `service_version`, `correlation_id`, `trace_id`, `span_id` | `service.name`, `service.version`, `correlation.id`, `trace.id`, `span.id` |
| root `layer`, `component`, `operation` | `labels.layer`, `labels.component`, `labels.operation` |

This migration is intentionally breaking. Compatibility forwarding packages were not retained because they recreate the package count and maintenance cost removed by v2.

## Performance

Reference benchmark results with Go 1.26:

| Path | Result |
| --- | --- |
| Stdout encoding to `io.Discard` | `392-425 ns/op`, `0 B/op`, `0 allocs/op` |
| Stdout encoding to null file | `891-930 ns/op`, `0 B/op`, `0 allocs/op` |
| Disabled stdout | about `8 ns/op`, `0 allocs/op` |
| Direct enqueue | `86-99 ns/op`, `0 allocs/op` |
| Full queue/drop | `60-65 ns/op`, `0 allocs/op` |
| HTTP NDJSON, 100 records | `42-44 us`, `30.5 KB`, `101 allocs` |
| net/http middleware | `5.48-6.55 us`, `5445 B/op`, `43 allocs` |
| GORM ignored fast success | `10-11 ns/op`, `0 allocs/op` |
| OTel without active span | `9.65-10.02 ns/op`, `0 allocs/op` |
| gRPC unary server interceptor | `1.04-1.12 us`, `160 B/op`, `1 alloc` |
| gRPC stream server interceptor | `1.00-1.14 us`, `160 B/op`, `1 alloc` |

CI enforces zero-allocation stdout/enqueue and a maximum of 120 allocations for a 100-record HTTP batch.

Stdout writes are synchronous by design: application logging inherits backpressure from its configured writer. Use Filebeat or another local collector on stdout/file output for normal deployments. Use an async direct-delivery sink only when bounded queueing and explicit drop accounting are preferred over writer backpressure.

## Validation

Run all modules:

```powershell
./test-all.ps1
```

Run optional soak locally:

```powershell
$env:OBSKIT_SOAK_DURATION = "60s"
go test -run '^TestSoak(Stdout|DirectHTTP)$' -v .
```

Linux CI runs test, vet, race, and bounded overload smoke for every module. Release proof uses a repeatable 60-90 second overload run plus a successful Linux race run.

## Repository Layout

```text
logger/          lower-level slog core
httplog/         shared HTTP capture and event policy
outbound/        outbound HTTP transport
joblog/          job lifecycle logging
redact/          header and JSON redaction
sampling/        deterministic sampling
elastic/         optional Elastic/OpenSearch delivery
httpsink/        optional generic HTTP delivery
lokisink/        optional Loki delivery
adapters/        independent framework/APM modules
examples/        runnable integrations
```

Runnable examples live under `examples/`. Public package import paths are stable within v2.
