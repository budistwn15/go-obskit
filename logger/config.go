package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	SchemaVersion  string
	Level          Level
	Format         Format
	Output         io.Writer
	AddSource      bool
	InstanceID     string
	// Middlewares allows optional handler wrapping (e.g. async external sinks).
	// Middlewares are applied in order.
	Middlewares []HandlerMiddleware
}

// HandlerMiddleware wraps a slog.Handler.
type HandlerMiddleware func(next slog.Handler) slog.Handler

func normalizeConfig(cfg Config) Config {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	if cfg.Level == "" {
		cfg.Level = LevelInfo
	}
	if cfg.Format == "" {
		cfg.Format = defaultFormat(cfg.Environment)
	}
	if strings.TrimSpace(cfg.SchemaVersion) == "" {
		cfg.SchemaVersion = DefaultSchemaVersion
	}
	return cfg
}

func defaultFormat(environment string) Format {
	env := strings.ToLower(strings.TrimSpace(environment))
	switch env {
	case "local", "dev", "development":
		return FormatText
	default:
		return FormatJSON
	}
}
