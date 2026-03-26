package joblog

import (
	"context"
	"io"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
)

func BenchmarkJobLifecycle(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	meta := Meta{
		JobName:     "sync-users",
		TriggerType: "cron",
		Component:   "scheduler",
		Operation:   "sync_users",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, run := Start(context.Background(), log, meta)
		run.AddProcessed(100)
		run.AddSucceeded(100)
		run.End(nil)
	}
}

func BenchmarkJobLifecycleSampledComplete(b *testing.B) {
	log := logger.New(
		logger.Config{
			ServiceName: "bench",
			Environment: "production",
			Output:      io.Discard,
		},
	)
	meta := Meta{
		JobName:     "sync-users",
		TriggerType: "cron",
		Component:   "scheduler",
		Operation:   "sync_users",
	}
	opts := DefaultOptions()
	opts.SuccessSampleEvery = 10
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, run := Start(context.Background(), log, meta, opts)
		run.AddProcessed(100)
		run.AddSucceeded(100)
		run.End(nil)
	}
}
