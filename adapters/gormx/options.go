package gormx

import (
	"context"
	"time"

	"github.com/budistwn15/go-obskit/sampling"
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
	LogSQLArgs      bool
	// IncludeWhereDetails extracts WHERE columns/values/conditions from SQL statement.
	IncludeWhereDetails bool
	// MaxWhereConditions bounds extraction work for hot path safety.
	MaxWhereConditions int
	// RedactWhereSensitiveValues masks sensitive WHERE values (email/password/token/etc).
	RedactWhereSensitiveValues bool

	SuccessSampleEvery      uint64
	ShouldLog               sampling.Hook
	ErrorDetailFunc         func(ctx context.Context, err error, statement string, rows int64) map[string]any
	IncludeExpectationHints bool

	IgnoreRecordNotFound bool
	RecoverInternally    bool
}

func DefaultOptions() Options {
	return Options{
		Level:                      gormlogger.Warn,
		DBSystem:                   "sql",
		SlowThreshold:              250 * time.Millisecond,
		LogSuccess:                 false,
		LogSQL:                     false,
		MaxSQLLen:                  2048,
		LogSQLOnError:              true,
		LogSQLOnSlow:               true,
		LogSQLOnSuccess:            false,
		LogRowsAffected:            true,
		LogSQLArgs:                 false,
		IncludeWhereDetails:        true,
		MaxWhereConditions:         16,
		RedactWhereSensitiveValues: true,
		SuccessSampleEvery:         1,
		IncludeExpectationHints:    true,
		IgnoreRecordNotFound:       true,
		RecoverInternally:          true,
	}
}

func normalizeOptions(opts Options) Options {
	def := DefaultOptions()
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
	if opts.MaxWhereConditions <= 0 {
		opts.MaxWhereConditions = def.MaxWhereConditions
	}
	return opts
}
