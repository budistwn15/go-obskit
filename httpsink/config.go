// Package httpsink ships slog records asynchronously to a generic HTTP endpoint.
package httpsink

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/budistwn15/go-obskit/v2/internal/slogsnapshot"
)

type PayloadFormat string

const (
	FormatNDJSON    PayloadFormat = "ndjson"
	FormatJSONArray PayloadFormat = "json_array"
)

type Config struct {
	Enabled  bool
	Endpoint string

	Format PayloadFormat

	Headers  map[string]string
	APIKey   string
	Username string
	Password string

	Timeout          time.Duration
	QueueSize        int
	BatchSize        int
	MaxRecordBytes   int
	MaxBatchBytes    int
	FlushInterval    time.Duration
	BlockOnQueueFull bool

	MaxRetries   int
	RetryBackoff time.Duration
	MaxBackoff   time.Duration

	RecoverInternally bool
	StaticFields      map[string]any
	HTTPClient        *http.Client
	OnError           func(error)

	ConnectionLogToStdout bool
	ConnectionLogLevel    slog.Level
	ConnectionLogOutput   io.Writer
}

func DefaultConfig() Config {
	return Config{
		Enabled:               false,
		Endpoint:              "",
		Format:                FormatNDJSON,
		Timeout:               2 * time.Second,
		QueueSize:             2048,
		BatchSize:             200,
		MaxRecordBytes:        64 * 1024,
		MaxBatchBytes:         1 * 1024 * 1024,
		FlushInterval:         1 * time.Second,
		BlockOnQueueFull:      false,
		MaxRetries:            3,
		RetryBackoff:          150 * time.Millisecond,
		MaxBackoff:            2 * time.Second,
		RecoverInternally:     true,
		ConnectionLogToStdout: true,
		ConnectionLogLevel:    slog.LevelInfo,
		ConnectionLogOutput:   os.Stdout,
	}
}

func normalizeConfig(cfg Config) Config {
	d := DefaultConfig()
	cfg.Endpoint = strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if cfg.Format == "" {
		cfg.Format = d.Format
	}
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
	if cfg.ConnectionLogOutput == nil {
		cfg.ConnectionLogOutput = d.ConnectionLogOutput
	}
	if cfg.ConnectionLogLevel == 0 {
		cfg.ConnectionLogLevel = d.ConnectionLogLevel
	}
	if cfg.Headers == nil {
		cfg.Headers = map[string]string{}
	} else {
		cfg.Headers = cloneStringMap(cfg.Headers)
	}
	cfg.StaticFields = cloneAnyMap(cfg.StaticFields)
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	} else {
		client := *cfg.HTTPClient
		if client.Timeout <= 0 {
			client.Timeout = cfg.Timeout
		}
		cfg.HTTPClient = &client
	}
	cfg.RecoverInternally = true
	return cfg
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
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
	return c.Enabled && c.Endpoint != ""
}
