package logger

import (
	"io"
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
	Level          Level
	Format         Format
	Output         io.Writer
	AddSource      bool
	InstanceID     string
}

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
