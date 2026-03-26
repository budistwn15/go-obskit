package joblog

import (
	"context"
	"log/slog"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/errorsx"
)

type event struct {
	level slog.Level
	msg   string
	attrs []slog.Attr
}

func buildStartedEvent(ctx context.Context, runID string, meta Meta, opts Options, startedAt time.Time) event {
	attrs := baseAttrs(ctx, runID, meta)
	if opts.IncludeTiming {
		attrs = append(attrs, slog.Time("job.started_at", startedAt.UTC()))
	}
	return event{
		level: slog.LevelInfo,
		msg:   "job started",
		attrs: append([]slog.Attr{slog.String("event", "job.started")}, attrs...),
	}
}

func buildCompletedEvent(
	ctx context.Context, runID string, meta Meta, counts Counts, startedAt, endedAt time.Time, opts Options,
) event {
	attrs := baseAttrs(ctx, runID, meta)
	if opts.IncludeTiming {
		duration := endedAt.Sub(startedAt)
		attrs = append(
			attrs,
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.Time("job.started_at", startedAt.UTC()),
			slog.Time("job.ended_at", endedAt.UTC()),
		)
		if opts.SlowThreshold > 0 {
			attrs = append(
				attrs,
				slog.Bool("slow", duration >= opts.SlowThreshold),
				slog.Int64("threshold_ms", opts.SlowThreshold.Milliseconds()),
				slog.Int64("slow_threshold_ms", opts.SlowThreshold.Milliseconds()),
			)
		}
	}
	if opts.IncludeCounts {
		attrs = append(attrs, countAttrs(counts)...)
	}
	return event{
		level: slog.LevelInfo,
		msg:   "job completed",
		attrs: append([]slog.Attr{slog.String("event", "job.completed")}, attrs...),
	}
}

func buildFailedEvent(
	ctx context.Context, runID string, meta Meta, counts Counts, startedAt, endedAt time.Time, err error, opts Options,
) event {
	attrs := baseAttrs(ctx, runID, meta)
	if opts.IncludeTiming {
		duration := endedAt.Sub(startedAt)
		attrs = append(
			attrs,
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.Time("job.started_at", startedAt.UTC()),
			slog.Time("job.ended_at", endedAt.UTC()),
		)
		if opts.SlowThreshold > 0 {
			attrs = append(
				attrs,
				slog.Bool("slow", duration >= opts.SlowThreshold),
				slog.Int64("threshold_ms", opts.SlowThreshold.Milliseconds()),
				slog.Int64("slow_threshold_ms", opts.SlowThreshold.Milliseconds()),
			)
		}
	}
	if opts.IncludeCounts {
		attrs = append(attrs, countAttrs(counts)...)
	}
	if err != nil {
		attrs = append(attrs, slog.String("error.message", err.Error()))
	}
	if e, ok := errorsx.Extract(err); ok && e != nil {
		if e.Meta.Code != "" {
			attrs = append(attrs, slog.String("error.code", e.Meta.Code))
		}
		if e.Meta.Type != "" {
			attrs = append(attrs, slog.String("error.type", e.Meta.Type))
		}
		if e.Meta.Layer != "" {
			attrs = append(attrs, slog.String("error.layer", e.Meta.Layer))
		}
		if e.Meta.Component != "" {
			attrs = append(attrs, slog.String("error.component", e.Meta.Component))
		}
		if e.Meta.Operation != "" {
			attrs = append(attrs, slog.String("error.operation", e.Meta.Operation))
		}
	}
	return event{
		level: slog.LevelError,
		msg:   "job failed",
		attrs: append([]slog.Attr{slog.String("event", "job.failed")}, attrs...),
	}
}

func buildRetryEvent(ctx context.Context, runID string, meta Meta, retry RetryMeta, opts Options) event {
	attrs := baseAttrs(ctx, runID, meta)
	attrs = append(
		attrs,
		slog.Int("job.retry_attempt", retry.Attempt),
		slog.Int("job.retry_max_attempts", retry.MaxAttempts),
		slog.String("job.retry_reason", retry.Reason),
		slog.Int64("job.retry_delay_ms", retry.Delay.Milliseconds()),
	)
	return event{
		level: slog.LevelWarn,
		msg:   "job retry",
		attrs: append([]slog.Attr{slog.String("event", "job.retry")}, attrs...),
	}
}

func baseAttrs(ctx context.Context, runID string, meta Meta) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("job.run_id", runID),
		slog.String("job.name", meta.JobName),
		slog.String("job.trigger_type", meta.TriggerType),
		slog.String("component", meta.Component),
		slog.String("operation", meta.Operation),
		slog.String("layer", "job"),
	}
	if corr := correlation.ID(ctx); corr != "" {
		attrs = append(attrs, slog.String("correlation_id", corr))
	}
	if len(meta.Fields) > 0 {
		attrs = append(attrs, slog.Any("job.fields", meta.Fields))
	}
	return attrs
}

func countAttrs(c Counts) []slog.Attr {
	return []slog.Attr{
		slog.Int64("job.count.processed", c.Processed),
		slog.Int64("job.count.succeeded", c.Succeeded),
		slog.Int64("job.count.failed", c.Failed),
		slog.Int64("job.count.skipped", c.Skipped),
	}
}
