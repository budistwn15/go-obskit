package configenv

type Entry struct {
	Key   string
	Value string
}

type Profile string

const (
	ProfileMinimal Profile = "minimal"
	ProfileFull    Profile = "full"
	ProfileLoki    Profile = "loki"
)

func Defaults() []Entry {
	return DefaultsByProfile(ProfileMinimal)
}

func DefaultsByProfile(profile Profile) []Entry {
	switch profile {
	case ProfileFull:
		return fullDefaults()
	case ProfileLoki:
		return lokiDefaults()
	default:
		return minimalDefaults()
	}
}

func lokiDefaults() []Entry {
	return []Entry{
		{Key: "APP_NAME", Value: "my-service"},
		{Key: "APP_ENV", Value: "production"},
		{Key: "LOG_LEVEL", Value: "info"},
		{Key: "OBSKIT_SINK_PROVIDER", Value: "loki"},
		{Key: "OBSKIT_LOKI_URL", Value: "http://localhost:3100"},
	}
}

func minimalDefaults() []Entry {
	return []Entry{
		{Key: "APP_NAME", Value: "my-service"},
		{Key: "APP_ENV", Value: "production"},
		{Key: "LOG_LEVEL", Value: "info"},
		{Key: "OBSKIT_SINK_PROVIDER", Value: "stdout"},
		{Key: "OBSKIT_ELASTIC_ENABLED", Value: "false"},
		{Key: "OBSKIT_ELASTIC_URL", Value: "http://localhost:9200"},
		{Key: "OBSKIT_ELASTIC_INDEX", Value: "xeanees-logs"},
		{Key: "OBSKIT_ELASTIC_USERNAME", Value: ""},
		{Key: "OBSKIT_ELASTIC_PASSWORD", Value: ""},
	}
}

func fullDefaults() []Entry {
	return []Entry{
		{Key: "APP_NAME", Value: "my-service"},
		{Key: "APP_VERSION", Value: "1.0.0"},
		{Key: "APP_ENV", Value: "production"},
		{Key: "LOG_LEVEL", Value: "info"},
		{Key: "LOG_FORMAT", Value: "json"},
		{Key: "LOG_ADD_SOURCE", Value: "false"},
		{Key: "LOG_INSTANCE_ID", Value: ""},
		{Key: "OBSKIT_SINK_PROVIDER", Value: "stdout"},

		{Key: "OBSKIT_HTTP_FORENSIC", Value: "false"},
		{Key: "OBSKIT_HTTP_CAPTURE_HEADERS", Value: "false"},
		{Key: "OBSKIT_HTTP_CAPTURE_QUERY", Value: "true"},
		{Key: "OBSKIT_HTTP_CAPTURE_REQUEST_BODY", Value: "false"},
		{Key: "OBSKIT_HTTP_CAPTURE_RESPONSE_BODY", Value: "false"},
		{Key: "OBSKIT_HTTP_LOG_SUCCESS_BODIES", Value: "false"},
		{Key: "OBSKIT_HTTP_LOG_ERROR_BODIES", Value: "true"},
		{Key: "OBSKIT_HTTP_MAX_BODY_BYTES", Value: "16384"},
		{Key: "OBSKIT_HTTP_SLOW_THRESHOLD_MS", Value: "1000"},
		{Key: "OBSKIT_HTTP_SUCCESS_SAMPLE_EVERY", Value: "1"},

		{Key: "OBSKIT_OUTBOUND_FORENSIC", Value: "false"},
		{Key: "OBSKIT_OUTBOUND_CAPTURE_HEADERS", Value: "false"},
		{Key: "OBSKIT_OUTBOUND_CAPTURE_QUERY", Value: "true"},
		{Key: "OBSKIT_OUTBOUND_CAPTURE_REQUEST_BODY", Value: "false"},
		{Key: "OBSKIT_OUTBOUND_CAPTURE_RESPONSE_BODY", Value: "false"},
		{Key: "OBSKIT_OUTBOUND_MAX_BODY_BYTES", Value: "16384"},
		{Key: "OBSKIT_OUTBOUND_SLOW_THRESHOLD_MS", Value: "1000"},
		{Key: "OBSKIT_OUTBOUND_SUCCESS_SAMPLE_EVERY", Value: "1"},

		{Key: "OBSKIT_GORM_TRACING", Value: "true"},
		{Key: "OBSKIT_GORM_LEVEL", Value: "info"},
		{Key: "OBSKIT_GORM_LOG_SUCCESS", Value: "true"},
		{Key: "OBSKIT_GORM_LOG_SQL", Value: "true"},
		{Key: "OBSKIT_GORM_LOG_SQL_ON_ERROR", Value: "true"},
		{Key: "OBSKIT_GORM_LOG_SQL_ON_SLOW", Value: "true"},
		{Key: "OBSKIT_GORM_LOG_SQL_ON_SUCCESS", Value: "true"},
		{Key: "OBSKIT_GORM_MAX_SQL_LEN", Value: "4096"},
		{Key: "OBSKIT_GORM_SLOW_THRESHOLD_MS", Value: "250"},
		{Key: "OBSKIT_GORM_SUCCESS_SAMPLE_EVERY", Value: "1"},
		{Key: "OBSKIT_GORM_IGNORE_RECORD_NOT_FOUND", Value: "true"},

		{Key: "OBSKIT_ELASTIC_ENABLED", Value: "true"},
		{Key: "OBSKIT_ELASTIC_URL", Value: "http://localhost:9200"},
		{Key: "OBSKIT_ELASTIC_INDEX", Value: "xeanees-logs"},
		{Key: "OBSKIT_ELASTIC_USERNAME", Value: "elastic"},
		{Key: "OBSKIT_ELASTIC_PASSWORD", Value: "secret"},
		{Key: "OBSKIT_ELASTIC_API_KEY", Value: ""},
		{Key: "OBSKIT_ELASTIC_INDEX_TIMESTAMP_SUFFIX", Value: "true"},
		{Key: "OBSKIT_ELASTIC_INDEX_TIMESTAMP_LAYOUT", Value: "2006.01.02"},
		{Key: "OBSKIT_ELASTIC_INDEX_PATTERN", Value: "xeanees-logs-*"},
		{Key: "OBSKIT_ELASTIC_TIMEOUT_MS", Value: "2000"},
		{Key: "OBSKIT_ELASTIC_QUEUE_SIZE", Value: "2048"},
		{Key: "OBSKIT_ELASTIC_BATCH_SIZE", Value: "200"},
		{Key: "OBSKIT_ELASTIC_FLUSH_INTERVAL_MS", Value: "1000"},
		{Key: "OBSKIT_ELASTIC_BLOCK_ON_QUEUE_FULL", Value: "false"},
		{Key: "OBSKIT_ELASTIC_MAX_RETRIES", Value: "3"},
		{Key: "OBSKIT_ELASTIC_RETRY_BACKOFF_MS", Value: "150"},
		{Key: "OBSKIT_ELASTIC_MAX_BACKOFF_MS", Value: "2000"},
		{Key: "OBSKIT_ELASTIC_ENABLE_MONITOR", Value: "true"},
		{Key: "OBSKIT_ELASTIC_MONITOR_INTERVAL_MS", Value: "15000"},
		{Key: "OBSKIT_ELASTIC_MONITOR_PATH", Value: "/"},
		{Key: "OBSKIT_ELASTIC_BOOTSTRAP", Value: "true"},
		{Key: "OBSKIT_ELASTIC_BOOTSTRAP_ON_START", Value: "true"},
		{Key: "OBSKIT_ELASTIC_PIPELINE_NAME", Value: "obskit-default-pipeline"},
		{Key: "OBSKIT_ELASTIC_TEMPLATE_NAME", Value: "obskit-default-template"},
		{Key: "OBSKIT_ELASTIC_APPLY_PIPELINE_TO_EXISTING", Value: "true"},
		{Key: "OBSKIT_ELASTIC_CONNECTION_LOG_TO_STDOUT", Value: "true"},
		{Key: "OBSKIT_ELASTIC_CONNECTION_LOG_ALL_CHECKS", Value: "false"},

		{Key: "OBSKIT_HTTP_SINK_ENABLED", Value: "false"},
		{Key: "OBSKIT_HTTP_SINK_URL", Value: "http://localhost:8088/logs"},
		{Key: "OBSKIT_HTTP_SINK_FORMAT", Value: "ndjson"},
		{Key: "OBSKIT_HTTP_SINK_HEADERS", Value: "X-Api-Key:token"},
		{Key: "OBSKIT_HTTP_SINK_API_KEY", Value: ""},
		{Key: "OBSKIT_HTTP_SINK_USERNAME", Value: ""},
		{Key: "OBSKIT_HTTP_SINK_PASSWORD", Value: ""},
		{Key: "OBSKIT_HTTP_SINK_TIMEOUT_MS", Value: "2000"},
		{Key: "OBSKIT_HTTP_SINK_QUEUE_SIZE", Value: "2048"},
		{Key: "OBSKIT_HTTP_SINK_BATCH_SIZE", Value: "200"},
		{Key: "OBSKIT_HTTP_SINK_FLUSH_INTERVAL_MS", Value: "1000"},
		{Key: "OBSKIT_HTTP_SINK_BLOCK_ON_QUEUE_FULL", Value: "false"},
		{Key: "OBSKIT_HTTP_SINK_MAX_RETRIES", Value: "3"},
		{Key: "OBSKIT_HTTP_SINK_RETRY_BACKOFF_MS", Value: "150"},
		{Key: "OBSKIT_HTTP_SINK_MAX_BACKOFF_MS", Value: "2000"},
		{Key: "OBSKIT_HTTP_SINK_CONNECTION_LOG_TO_STDOUT", Value: "true"},

		{Key: "OBSKIT_LOKI_ENABLED", Value: "false"},
		{Key: "OBSKIT_LOKI_URL", Value: "http://localhost:3100"},
		{Key: "OBSKIT_LOKI_LABELS", Value: "source:obskit,env:production"},
		{Key: "OBSKIT_LOKI_API_KEY", Value: ""},
		{Key: "OBSKIT_LOKI_USERNAME", Value: ""},
		{Key: "OBSKIT_LOKI_PASSWORD", Value: ""},
		{Key: "OBSKIT_LOKI_TIMEOUT_MS", Value: "2000"},
		{Key: "OBSKIT_LOKI_QUEUE_SIZE", Value: "2048"},
		{Key: "OBSKIT_LOKI_BATCH_SIZE", Value: "200"},
		{Key: "OBSKIT_LOKI_FLUSH_INTERVAL_MS", Value: "1000"},
		{Key: "OBSKIT_LOKI_BLOCK_ON_QUEUE_FULL", Value: "false"},
		{Key: "OBSKIT_LOKI_MAX_RETRIES", Value: "3"},
		{Key: "OBSKIT_LOKI_RETRY_BACKOFF_MS", Value: "150"},
		{Key: "OBSKIT_LOKI_MAX_BACKOFF_MS", Value: "2000"},
		{Key: "OBSKIT_LOKI_CONNECTION_LOG_TO_STDOUT", Value: "true"},
	}
}
