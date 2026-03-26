package joblog

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

type Meta struct {
	JobName     string
	TriggerType string
	Component   string
	Operation   string
	Fields      map[string]any
}

type Counts struct {
	Processed int64
	Succeeded int64
	Failed    int64
	Skipped   int64
}

type RetryMeta struct {
	Attempt     int
	MaxAttempts int
	Reason      string
	Delay       time.Duration
}

type Run struct {
	ctx       context.Context
	log       *slog.Logger
	meta      Meta
	opts      Options
	runID     string
	startedAt time.Time
	ended     atomic.Bool

	processed atomic.Int64
	succeeded atomic.Int64
	failed    atomic.Int64
	skipped   atomic.Int64
}
