package gormx

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/logger"
	gormlogger "gorm.io/gorm/logger"
)

func BenchmarkTraceSlowQueryWarn(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	gl := New(
		log, Options{
			Level:         gormlogger.Warn,
			SlowThreshold: 5 * time.Millisecond,
			LogSQL:        true,
			MaxSQLLen:     256,
		},
	).(gormlogger.Interface)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gl.Trace(
			context.Background(), time.Now().Add(-10*time.Millisecond), func() (string, int64) {
				return "SELECT * FROM users WHERE status = 'active'", 100
			}, nil,
		)
	}
}

func BenchmarkTraceError(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	gl := New(
		log, Options{
			Level:  gormlogger.Error,
			LogSQL: true,
		},
	).(gormlogger.Interface)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gl.Trace(
			context.Background(), time.Now(), func() (string, int64) {
				return "SELECT 1", -1
			}, errors.New("db down"),
		)
	}
}

func BenchmarkTraceSuccessSampled(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	gl := New(
		log, Options{
			Level:              gormlogger.Info,
			LogSuccess:         true,
			SuccessSampleEvery: 10,
		},
	).(gormlogger.Interface)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gl.Trace(
			context.Background(), time.Now(), func() (string, int64) {
				return "SELECT 1", 1
			}, nil,
		)
	}
}
