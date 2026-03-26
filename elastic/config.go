package elastic

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config controls optional async bulk shipping to Elasticsearch/OpenSearch.
type Config struct {
	Enabled bool

	// Direct compatibility fields (as requested by app teams).
	ElasticIndex    string
	ElasticURL      string
	ElasticUsername string
	ElasticPassword string

	// Generic fields (preferred for new code).
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
	QueueSize     int
	BatchSize     int
	FlushInterval time.Duration
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
	ConnectionLogToStdout  bool
	ConnectionLogLevel     slog.Level
	ConnectionLogAllChecks bool
	ConnectionLogOutput    io.Writer

	// Connection monitor (optional).
	EnableMonitor   bool
	MonitorInterval time.Duration
	MonitorPath     string
	OnMonitor       func(ConnectionStatus)
}

func DefaultConfig() Config {
	return Config{
		Enabled:                false,
		Endpoint:               "",
		Index:                  "app-logs",
		Timeout:                2 * time.Second,
		QueueSize:              2048,
		BatchSize:              200,
		FlushInterval:          1 * time.Second,
		MaxRetries:             3,
		RetryBackoff:           150 * time.Millisecond,
		MaxBackoff:             2 * time.Second,
		RecoverInternally:      true,
		ConnectionLogToStdout:  true,
		ConnectionLogLevel:     slog.LevelInfo,
		ConnectionLogAllChecks: false,
		ConnectionLogOutput:    os.Stdout,
		EnableMonitor:          true,
		IndexTimestampLayout:   "2006.01.02",
		MonitorInterval:        15 * time.Second,
		MonitorPath:            "/",
	}
}

func normalizeConfig(cfg Config) Config {
	d := DefaultConfig()
	// Backward/direct mapping.
	if cfg.ElasticIndex != "" {
		cfg.Index = cfg.ElasticIndex
	} else if cfg.Index == "" {
		cfg.Index = cfg.ElasticIndex
	}
	if cfg.ElasticURL != "" {
		cfg.Endpoint = cfg.ElasticURL
	} else if cfg.Endpoint == "" {
		cfg.Endpoint = cfg.ElasticURL
	}
	if cfg.ElasticUsername != "" {
		cfg.Username = cfg.ElasticUsername
	} else if cfg.Username == "" {
		cfg.Username = cfg.ElasticUsername
	}
	if cfg.ElasticPassword != "" {
		cfg.Password = cfg.ElasticPassword
	} else if cfg.Password == "" {
		cfg.Password = cfg.ElasticPassword
	}
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
	if cfg.MonitorInterval <= 0 {
		cfg.MonitorInterval = d.MonitorInterval
	}
	if strings.TrimSpace(cfg.MonitorPath) == "" {
		cfg.MonitorPath = d.MonitorPath
	}
	if !strings.HasPrefix(cfg.MonitorPath, "/") {
		cfg.MonitorPath = "/" + cfg.MonitorPath
	}
	if cfg.ConnectionLogOutput == nil {
		cfg.ConnectionLogOutput = d.ConnectionLogOutput
	}
	if cfg.ConnectionLogLevel == 0 {
		cfg.ConnectionLogLevel = d.ConnectionLogLevel
	}
	// Safety-first default.
	cfg.RecoverInternally = true
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: cfg.Timeout}
	} else if cfg.HTTPClient.Timeout <= 0 {
		cfg.HTTPClient.Timeout = cfg.Timeout
	}
	return cfg
}

func (c Config) active() bool {
	return c.Enabled && c.Endpoint != "" && c.Index != ""
}
