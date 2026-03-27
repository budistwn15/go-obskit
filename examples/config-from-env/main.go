package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/budistwn15/go-obskit/adapters/ginx"
	"github.com/budistwn15/go-obskit/adapters/gormx"
	"github.com/budistwn15/go-obskit/elastic"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/gin-gonic/gin"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	elkCfg := elastic.Config{
		Enabled:         envBool("OBSKIT_ELASTIC_ENABLED", false),
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
	}
	elkMW := elastic.NewMiddleware(elkCfg)
	defer func() { _ = elkMW.Close(context.Background()) }()

	log := logger.New(logger.Config{
		ServiceName:    envString("APP_NAME", "config-from-env"),
		ServiceVersion: envString("APP_VERSION", "1.0.0"),
		Environment:    envString("APP_ENV", "local"),
		Level:          logger.Level(strings.ToLower(envString("LOG_LEVEL", "info"))),
		Format:         logger.Format(strings.ToLower(envString("LOG_FORMAT", "json"))),
		AddSource:      envBool("LOG_ADD_SOURCE", false),
		InstanceID:     envString("LOG_INSTANCE_ID", ""),
		Middlewares:    []logger.HandlerMiddleware{elkMW.LoggerMiddleware()},
	})

	var httpOpts ginx.Options
	if envBool("OBSKIT_HTTP_FORENSIC", false) {
		httpOpts = ginx.ForensicOptions()
	} else {
		httpOpts = ginx.DefaultOptions()
	}
	httpOpts.CaptureHeaders = envBool("OBSKIT_HTTP_CAPTURE_HEADERS", httpOpts.CaptureHeaders)
	httpOpts.CaptureQuery = envBool("OBSKIT_HTTP_CAPTURE_QUERY", httpOpts.CaptureQuery)
	httpOpts.CaptureRequestBody = envBool("OBSKIT_HTTP_CAPTURE_REQUEST_BODY", httpOpts.CaptureRequestBody)
	httpOpts.CaptureResponseBody = envBool("OBSKIT_HTTP_CAPTURE_RESPONSE_BODY", httpOpts.CaptureResponseBody)
	httpOpts.LogSuccessBodies = envBool("OBSKIT_HTTP_LOG_SUCCESS_BODIES", httpOpts.LogSuccessBodies)
	httpOpts.LogErrorBodies = envBool("OBSKIT_HTTP_LOG_ERROR_BODIES", httpOpts.LogErrorBodies)
	httpOpts.MaxBodyBytes = envInt("OBSKIT_HTTP_MAX_BODY_BYTES", httpOpts.MaxBodyBytes)
	httpOpts.SlowRequestThreshold = envDurationMS("OBSKIT_HTTP_SLOW_THRESHOLD_MS", int(httpOpts.SlowRequestThreshold.Milliseconds()))
	httpOpts.SuccessSampleEvery = uint64(envInt("OBSKIT_HTTP_SUCCESS_SAMPLE_EVERY", int(httpOpts.SuccessSampleEvery)))

	gormOpts := gormx.DefaultOptions()
	if envBool("OBSKIT_GORM_TRACING", false) {
		gormOpts = gormx.TracingOptions()
	}
	gormOpts.LogSuccess = envBool("OBSKIT_GORM_LOG_SUCCESS", gormOpts.LogSuccess)
	gormOpts.LogSQL = envBool("OBSKIT_GORM_LOG_SQL", gormOpts.LogSQL)
	gormOpts.LogSQLOnError = envBool("OBSKIT_GORM_LOG_SQL_ON_ERROR", gormOpts.LogSQLOnError)
	gormOpts.LogSQLOnSlow = envBool("OBSKIT_GORM_LOG_SQL_ON_SLOW", gormOpts.LogSQLOnSlow)
	gormOpts.LogSQLOnSuccess = envBool("OBSKIT_GORM_LOG_SQL_ON_SUCCESS", gormOpts.LogSQLOnSuccess)
	gormOpts.MaxSQLLen = envInt("OBSKIT_GORM_MAX_SQL_LEN", gormOpts.MaxSQLLen)
	gormOpts.SlowThreshold = envDurationMS("OBSKIT_GORM_SLOW_THRESHOLD_MS", int(gormOpts.SlowThreshold.Milliseconds()))
	gormOpts.SuccessSampleEvery = uint64(envInt("OBSKIT_GORM_SUCCESS_SAMPLE_EVERY", int(gormOpts.SuccessSampleEvery)))
	gormOpts.IgnoreRecordNotFound = envBool("OBSKIT_GORM_IGNORE_RECORD_NOT_FOUND", gormOpts.IgnoreRecordNotFound)
	gormOpts.Level = parseGormLevel(envString("OBSKIT_GORM_LEVEL", "info"), gormOpts.Level)
	_ = gormx.New(log, gormOpts) // pass into gorm.Config{Logger: ...}

	r := gin.New()
	r.Use(ginx.Middleware(log, httpOpts))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	log.Info("config-from-env ready",
		slog.Bool("elastic_enabled", elkCfg.Enabled),
		slog.String("elastic_index", elkCfg.ElasticIndex),
		slog.Bool("http_forensic", envBool("OBSKIT_HTTP_FORENSIC", false)),
		slog.Bool("gorm_tracing", envBool("OBSKIT_GORM_TRACING", false)),
	)
	_ = r.Run(":8085")
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

func parseGormLevel(v string, def gormlogger.LogLevel) gormlogger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "silent":
		return gormlogger.Silent
	case "error":
		return gormlogger.Error
	case "warn", "warning":
		return gormlogger.Warn
	case "info":
		return gormlogger.Info
	default:
		return def
	}
}
