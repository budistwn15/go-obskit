// Package elastic ships slog records asynchronously to Elasticsearch or OpenSearch.
package elastic

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/budistwn15/go-obskit/v2/internal/slogsnapshot"
)

// Config controls optional async bulk shipping to Elasticsearch/OpenSearch.
type Config struct {
	Enabled bool

	// Endpoint example: http://localhost:9200
	Endpoint string
	Index    string

	// Auth is optional.
	APIKey   string
	Username string
	Password string

	// Optional date suffix for index naming, e.g. "app-logs-2026.03.26".
	IndexTimestampSuffix bool
	IndexTimestampLayout string

	// HTTP timeout per bulk request.
	Timeout time.Duration

	// Batch and buffering.
	QueueSize      int
	BatchSize      int
	MaxRecordBytes int
	MaxBatchBytes  int
	FlushInterval  time.Duration
	// BlockOnQueueFull will block request path when queue is full.
	// Default false (non-blocking).
	BlockOnQueueFull bool

	// Retry strategy for transient failures.
	MaxRetries   int
	RetryBackoff time.Duration
	MaxBackoff   time.Duration

	// RecoverInternally prevents sink panics from affecting app flow.
	// Default true.
	RecoverInternally bool

	// Optional static fields appended to all shipped documents.
	StaticFields map[string]any

	// Optional client override.
	HTTPClient *http.Client

	// Optional callback for sink internal errors.
	OnError func(error)

	// Connection/sink status logs to stdout (structured).
	ConnectionLogToStdout bool
	ConnectionLogLevel    slog.Level
	ConnectionLogOutput   io.Writer
}

func DefaultConfig() Config {
	return Config{
		Enabled:               false,
		Endpoint:              "",
		Index:                 "app-logs",
		Timeout:               2 * time.Second,
		QueueSize:             2048,
		BatchSize:             200,
		MaxRecordBytes:        64 * 1024,
		MaxBatchBytes:         1 * 1024 * 1024,
		FlushInterval:         1 * time.Second,
		MaxRetries:            3,
		RetryBackoff:          150 * time.Millisecond,
		MaxBackoff:            2 * time.Second,
		RecoverInternally:     true,
		ConnectionLogToStdout: true,
		ConnectionLogLevel:    slog.LevelInfo,
		ConnectionLogOutput:   os.Stdout,
		IndexTimestampLayout:  "2006.01.02",
	}
}

func normalizeConfig(cfg Config) Config {
	d := DefaultConfig()
	if cfg.Index == "" {
		cfg.Index = d.Index
	}
	cfg.Endpoint = strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if cfg.Timeout <= 0 {
		cfg.Timeout = d.Timeout
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = d.QueueSize
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = d.BatchSize
	}
	if cfg.MaxRecordBytes <= 0 {
		cfg.MaxRecordBytes = d.MaxRecordBytes
	}
	if cfg.MaxBatchBytes <= 0 {
		cfg.MaxBatchBytes = d.MaxBatchBytes
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = d.FlushInterval
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = d.MaxRetries
	}
	if cfg.RetryBackoff <= 0 {
		cfg.RetryBackoff = d.RetryBackoff
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = d.MaxBackoff
	}
	if strings.TrimSpace(cfg.IndexTimestampLayout) == "" {
		cfg.IndexTimestampLayout = d.IndexTimestampLayout
	}
	if cfg.ConnectionLogOutput == nil {
		cfg.ConnectionLogOutput = d.ConnectionLogOutput
	}
	if cfg.ConnectionLogLevel == 0 {
		cfg.ConnectionLogLevel = d.ConnectionLogLevel
	}
	cfg.StaticFields = cloneAnyMap(cfg.StaticFields)
	// Safety-first default.
	cfg.RecoverInternally = true
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	} else {
		client := *cfg.HTTPClient
		if client.Timeout <= 0 {
			client.Timeout = cfg.Timeout
		}
		cfg.HTTPClient = &client
	}
	return cfg
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = slogsnapshot.Value(v)
	}
	return out
}

func (c Config) active() bool {
	return c.Enabled && c.Endpoint != "" && c.Index != ""
}
