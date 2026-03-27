package sinkenv

import (
	"context"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/budistwn15/go-obskit/elastic"
	"github.com/budistwn15/go-obskit/httpsink"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/budistwn15/go-obskit/lokisink"
)

type Runtime struct {
	Provider    string
	Middlewares []logger.HandlerMiddleware
	closeFn     func(context.Context) error
}

func (r Runtime) Close(ctx context.Context) error {
	if r.closeFn == nil {
		return nil
	}
	return r.closeFn(ctx)
}

// FromEnv builds optional log sinks from environment.
// Supported providers:
// - stdout/none: no external sink
// - elastic/elk: Elasticsearch sink
// - loki/grafana-loki: Grafana Loki sink
// - http/webhook: generic HTTP sink
func FromEnv() Runtime {
	provider := strings.ToLower(strings.TrimSpace(envString("OBSKIT_SINK_PROVIDER", "stdout")))
	switch provider {
	case "elastic", "elk":
		m := elastic.NewMiddleware(elastic.Config{
			Enabled:         envBool("OBSKIT_ELASTIC_ENABLED", true),
			ElasticURL:      envString("OBSKIT_ELASTIC_URL", ""),
			ElasticIndex:    envString("OBSKIT_ELASTIC_INDEX", "app-logs"),
			ElasticUsername: envString("OBSKIT_ELASTIC_USERNAME", ""),
			ElasticPassword: envString("OBSKIT_ELASTIC_PASSWORD", ""),
			APIKey:          envString("OBSKIT_ELASTIC_API_KEY", ""),

			IndexTimestampSuffix: envBool("OBSKIT_ELASTIC_INDEX_TIMESTAMP_SUFFIX", true),
			IndexTimestampLayout: envString("OBSKIT_ELASTIC_INDEX_TIMESTAMP_LAYOUT", "2006.01.02"),
			IndexPattern:         envString("OBSKIT_ELASTIC_INDEX_PATTERN", envString("OBSKIT_ELASTIC_INDEX", "app-logs")+"-*"),

			Timeout:       envDurationMS("OBSKIT_ELASTIC_TIMEOUT_MS", 2000),
			QueueSize:     envInt("OBSKIT_ELASTIC_QUEUE_SIZE", 2048),
			BatchSize:     envInt("OBSKIT_ELASTIC_BATCH_SIZE", 200),
			FlushInterval: envDurationMS("OBSKIT_ELASTIC_FLUSH_INTERVAL_MS", 1000),

			BlockOnQueueFull: envBool("OBSKIT_ELASTIC_BLOCK_ON_QUEUE_FULL", false),
			MaxRetries:       envInt("OBSKIT_ELASTIC_MAX_RETRIES", 3),
			RetryBackoff:     envDurationMS("OBSKIT_ELASTIC_RETRY_BACKOFF_MS", 150),
			MaxBackoff:       envDurationMS("OBSKIT_ELASTIC_MAX_BACKOFF_MS", 2000),

			EnableMonitor:   envBool("OBSKIT_ELASTIC_ENABLE_MONITOR", true),
			MonitorInterval: envDurationMS("OBSKIT_ELASTIC_MONITOR_INTERVAL_MS", 15000),
			MonitorPath:     envString("OBSKIT_ELASTIC_MONITOR_PATH", "/"),

			Bootstrap:               envBool("OBSKIT_ELASTIC_BOOTSTRAP", true),
			BootstrapOnStart:        envBool("OBSKIT_ELASTIC_BOOTSTRAP_ON_START", true),
			PipelineName:            envString("OBSKIT_ELASTIC_PIPELINE_NAME", "obskit-default-pipeline"),
			TemplateName:            envString("OBSKIT_ELASTIC_TEMPLATE_NAME", "obskit-default-template"),
			ApplyPipelineToExisting: envBool("OBSKIT_ELASTIC_APPLY_PIPELINE_TO_EXISTING", true),

			ConnectionLogToStdout:  envBool("OBSKIT_ELASTIC_CONNECTION_LOG_TO_STDOUT", true),
			ConnectionLogAllChecks: envBool("OBSKIT_ELASTIC_CONNECTION_LOG_ALL_CHECKS", false),
		})
		return Runtime{
			Provider:    "elastic",
			Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
			closeFn:     m.Close,
		}
	case "http", "webhook":
		headers := parseHeaders(envString("OBSKIT_HTTP_SINK_HEADERS", ""))
		m := httpsink.NewMiddleware(httpsink.Config{
			Enabled:  envBool("OBSKIT_HTTP_SINK_ENABLED", true),
			Endpoint: envString("OBSKIT_HTTP_SINK_URL", ""),
			Format:   httpsink.PayloadFormat(envString("OBSKIT_HTTP_SINK_FORMAT", string(httpsink.FormatNDJSON))),
			Headers:  headers,
			APIKey:   envString("OBSKIT_HTTP_SINK_API_KEY", ""),
			Username: envString("OBSKIT_HTTP_SINK_USERNAME", ""),
			Password: envString("OBSKIT_HTTP_SINK_PASSWORD", ""),

			Timeout:       envDurationMS("OBSKIT_HTTP_SINK_TIMEOUT_MS", 2000),
			QueueSize:     envInt("OBSKIT_HTTP_SINK_QUEUE_SIZE", 2048),
			BatchSize:     envInt("OBSKIT_HTTP_SINK_BATCH_SIZE", 200),
			FlushInterval: envDurationMS("OBSKIT_HTTP_SINK_FLUSH_INTERVAL_MS", 1000),

			BlockOnQueueFull: envBool("OBSKIT_HTTP_SINK_BLOCK_ON_QUEUE_FULL", false),
			MaxRetries:       envInt("OBSKIT_HTTP_SINK_MAX_RETRIES", 3),
			RetryBackoff:     envDurationMS("OBSKIT_HTTP_SINK_RETRY_BACKOFF_MS", 150),
			MaxBackoff:       envDurationMS("OBSKIT_HTTP_SINK_MAX_BACKOFF_MS", 2000),

			ConnectionLogToStdout: envBool("OBSKIT_HTTP_SINK_CONNECTION_LOG_TO_STDOUT", true),
		})
		return Runtime{
			Provider:    "http",
			Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
			closeFn:     m.Close,
		}
	case "loki", "grafana-loki":
		m := lokisink.NewMiddleware(lokisink.Config{
			Enabled:  envBool("OBSKIT_LOKI_ENABLED", true),
			Endpoint: envString("OBSKIT_LOKI_URL", ""),
			Labels:   parseHeaders(envString("OBSKIT_LOKI_LABELS", "source:obskit")),

			APIKey:   envString("OBSKIT_LOKI_API_KEY", ""),
			Username: envString("OBSKIT_LOKI_USERNAME", ""),
			Password: envString("OBSKIT_LOKI_PASSWORD", ""),

			Timeout:       envDurationMS("OBSKIT_LOKI_TIMEOUT_MS", 2000),
			QueueSize:     envInt("OBSKIT_LOKI_QUEUE_SIZE", 2048),
			BatchSize:     envInt("OBSKIT_LOKI_BATCH_SIZE", 200),
			FlushInterval: envDurationMS("OBSKIT_LOKI_FLUSH_INTERVAL_MS", 1000),

			BlockOnQueueFull: envBool("OBSKIT_LOKI_BLOCK_ON_QUEUE_FULL", false),
			MaxRetries:       envInt("OBSKIT_LOKI_MAX_RETRIES", 3),
			RetryBackoff:     envDurationMS("OBSKIT_LOKI_RETRY_BACKOFF_MS", 150),
			MaxBackoff:       envDurationMS("OBSKIT_LOKI_MAX_BACKOFF_MS", 2000),

			ConnectionLogToStdout: envBool("OBSKIT_LOKI_CONNECTION_LOG_TO_STDOUT", true),
		})
		return Runtime{
			Provider:    "loki",
			Middlewares: []logger.HandlerMiddleware{m.LoggerMiddleware()},
			closeFn:     m.Close,
		}
	default:
		return Runtime{Provider: "stdout", Middlewares: nil}
	}
}

func parseHeaders(raw string) map[string]string {
	out := map[string]string{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	pairs := strings.Split(raw, ",")
	for _, p := range pairs {
		kv := strings.SplitN(strings.TrimSpace(p), ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := strings.TrimSpace(kv[1])
		if k == "" {
			continue
		}
		out[k] = v
	}
	return out
}

func envString(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return def
	}
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDurationMS(key string, defMS int) time.Duration {
	ms := envInt(key, defMS)
	if ms <= 0 {
		ms = defMS
	}
	return time.Duration(ms) * time.Millisecond
}
