package main

import (
	"context"
	"time"

	"github.com/budistwn15/go-obskit/joblog"
	"github.com/budistwn15/go-obskit/logger"
)

func main() {
	log := logger.New(
		logger.Config{
			ServiceName: "scheduler-basic",
			Environment: "local",
			Level:       logger.LevelInfo,
		},
	)
	
	ctx := context.Background()
	ctx, run := joblog.Start(
		ctx, log, joblog.Meta{
			JobName:     "daily-report",
			TriggerType: "cron",
			Component:   "scheduler",
			Operation:   "daily_report",
		},
	)
	
	run.AddProcessed(100)
	run.AddSucceeded(99)
	run.AddFailed(1)
	run.Retry(joblog.RetryMeta{Attempt: 2, MaxAttempts: 3, Reason: "temporary timeout", Delay: 2 * time.Second})
	run.End(nil)
	
	_ = ctx
}
