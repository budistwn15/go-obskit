// Package gormx provides structured, low-overhead logging for GORM.
package gormx

import (
	"context"
	"time"

	"github.com/budistwn15/go-obskit/v2/sampling"
	gormlogger "gorm.io/gorm/logger"
)

type Options struct {
	Level gormlogger.LogLevel

	// DBSystem should reflect actual database target, e.g. "postgresql", "mysql", "sqlserver".
	// Default: "sql".
	DBSystem string

	SlowThreshold time.Duration

	LogSuccess      bool
	LogSQL          bool
	MaxSQLLen       int
	LogSQLOnError   bool
	LogSQLOnSlow    bool
	LogSQLOnSuccess bool

	LogRowsAffected bool

	SuccessSampleEvery      uint64
	ShouldLog               sampling.Hook
	ErrorDetailFunc         func(ctx context.Context, err error, statement string, rows int64) map[string]any
	IncludeExpectationHints bool

	IgnoreRecordNotFound bool
	RecoverInternally    bool
}

func DefaultOptions() Options {
	return Options{
		Level:                   gormlogger.Warn,
		DBSystem:                "sql",
		SlowThreshold:           250 * time.Millisecond,
		LogSuccess:              false,
		LogSQL:                  false,
		MaxSQLLen:               2048,
		LogSQLOnError:           true,
		LogSQLOnSlow:            true,
		LogSQLOnSuccess:         false,
		LogRowsAffected:         true,
		SuccessSampleEvery:      1,
		IncludeExpectationHints: true,
		IgnoreRecordNotFound:    true,
		RecoverInternally:       true,
	}
}

func normalizeOptions(opts Options) Options {
	def := DefaultOptions()
	opts.RecoverInternally = true
	if opts.Level == 0 {
		opts.Level = def.Level
	}
	if opts.SlowThreshold <= 0 {
		opts.SlowThreshold = def.SlowThreshold
	}
	if opts.MaxSQLLen <= 0 {
		opts.MaxSQLLen = def.MaxSQLLen
	}
	if opts.SuccessSampleEvery == 0 {
		opts.SuccessSampleEvery = def.SuccessSampleEvery
	}
	if opts.DBSystem == "" {
		opts.DBSystem = def.DBSystem
	}
	return opts
}
