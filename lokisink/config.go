// Package lokisink ships slog records asynchronously to Grafana Loki.
package lokisink

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	Enabled  bool
	Endpoint string

	Labels map[string]string

	APIKey   string
	Username string
	Password string

	Timeout          time.Duration
	QueueSize        int
	BatchSize        int
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
		Labels:                map[string]string{"source": "obskit"},
		Timeout:               2 * time.Second,
		QueueSize:             2048,
		BatchSize:             200,
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
	if cfg.Timeout <= 0 {
		cfg.Timeout = d.Timeout
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = d.QueueSize
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = d.BatchSize
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
	if cfg.Labels == nil {
		cfg.Labels = cloneStringMap(d.Labels)
	} else {
		cfg.Labels = cloneStringMap(cfg.Labels)
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
		out[k] = v
	}
	return out
}

func (c Config) active() bool {
	return c.Enabled && c.Endpoint != ""
}
