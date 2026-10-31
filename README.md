# go-obskit

Small, stdout-first structured logging for Go 1.26.

- Default path: JSON lines to stdout, no goroutine, no network.
- Optional delivery: bounded asynchronous HTTP, Elasticsearch/OpenSearch, or Loki.
- Optional instrumentation: net/http, Gin, Fiber, GORM, outbound HTTP, jobs.
- Optional APM correlation: OpenTelemetry in an independent module.

## Quick Start

```go
package main

import (
	"log/slog"

	obskit "github.com/budistwn15/go-obskit"
)

func main() {
	log := obskit.New(obskit.Config{
		Service:     "billing-api",
		Environment: "production",
	})
	log.Info("ready", slog.String("region", "ap-southeast-1"))
}
```

Output is one JSON object per line. Stdout keeps `log/slog` keys `time`, `level`, and `msg` because renaming them adds an allocation to every call. Application fields use ECS-compatible names such as `service.name`, `service.environment`, `event.action`, `trace.id`, and `error.type`.

## Install

```bash
go get github.com/budistwn15/go-obskit
```

Install only adapters used by the application:

```bash
go get github.com/budistwn15/go-obskit/adapters/nethttp
go get github.com/budistwn15/go-obskit/adapters/ginx
go get github.com/budistwn15/go-obskit/adapters/fiberx
go get github.com/budistwn15/go-obskit/adapters/gormx
go get github.com/budistwn15/go-obskit/adapters/otel
```

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
- Enqueue is non-blocking by default; full queues increment `Dropped`.
- Retry waits observe shutdown cancellation.
- Only transport errors, HTTP 429, and HTTP 5xx are retried.
- HTTP 4xx responses are permanent failures.
- Response error bodies are capped at 1 KiB.
- `Close(ctx)` stops intake, flushes accepted records, cancels in-flight work, and waits for the worker.
- Stdout logging remains available when delivery is slow, down, or saturated.

Inspect counters with `sink.Stats()`: `Enqueued`, `Dropped`, `Sent`, `Failed`, and `Retried`.

Known ceiling: one unusually large record can create a large encoded payload. Queue length and batch count are bounded; a strict per-record byte limit remains required before the v2 release tag.

## HTTP Instrumentation

net/http:

```go
handler := nethttp.Middleware(log, nethttp.DefaultOptions())(mux)
http.ListenAndServe(":8080", handler)
```

Gin:

```go
router.Use(ginx.Middleware(log, ginx.DefaultOptions()))
```

Fiber:

```go
app.Use(fiberx.Middleware(log, fiberx.DefaultOptions()))
```

Default options do not capture headers, query values, or bodies. `ForensicOptions()` is explicit and should be limited to incident debugging.

## Outbound HTTP

```go
client := outbound.WrapClient(http.DefaultClient, log, outbound.DefaultOptions())
```

The wrapper clones the client and request only when propagation requires it. Caller-owned clients, requests, and bodies remain usable.

## GORM

```go
gormLogger := gormx.New(log, gormx.DefaultOptions())
db, err := gorm.Open(dialector, &gorm.Config{Logger: gormLogger})
```

Fast successful queries are ignored by default and do not call GORM's SQL callback. SQL value extraction, WHERE parsing, and query fingerprinting are not provided.

## OpenTelemetry Correlation

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
| `logger.New` for common setup | `obskit.New` |
| manual middleware and sink close | `obskit.Open` and `Runtime.Close` |
| `httpin` | `adapters/nethttp` |
| `httpout` | `outbound` |
| `sinkenv` | explicit provider config |
| `configenv`, `cmd/obskit-envsync` | application-owned config |
| Elastic bootstrap and monitor | deployment tooling and platform health checks |
| `logger.WithCommon` | `logger.With` |
| `logger.WithContext` | `logger.WithLogger` |
| Elastic compatibility aliases | `Endpoint`, `Index`, `Username`, `Password` |
| automatic JSON expansion | explicit structured attributes |
| GORM WHERE values/fingerprint | query type and table only |

This migration is intentionally breaking. Compatibility forwarding packages were not retained because they recreate the package count and maintenance cost removed by v2.

## Performance

Measured on Intel Core Ultra 5 235U, Windows, Go 1.26:

| Path | Result |
| --- | --- |
| Basic stdout | `0 B/op`, `0 allocs/op` |
| Disabled stdout | about `8 ns/op`, `0 allocs/op` |
| Direct enqueue | `65-73 ns/op`, `0 allocs/op` |
| Full queue/drop | `33-35 ns/op`, `0 allocs/op` |
| HTTP NDJSON, 100 records | `42-44 us`, `30.5 KB`, `101 allocs` |
| GORM ignored fast success | `10-11 ns/op`, `0 allocs/op` |
| OTel without active span | `9.65-10.02 ns/op`, `0 allocs/op` |

CI enforces zero-allocation stdout/enqueue and a maximum of 120 allocations for a 100-record HTTP batch.

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

Linux CI runs test, vet, race, and bounded overload smoke for every module. A 30-60 minute soak and successful Linux race run remain mandatory before tagging v2.

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
