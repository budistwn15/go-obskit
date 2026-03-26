package joblog

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/budistwn15/go-obskit/sampling"
)

var jobCompleteSampleCounter atomic.Uint64

func Start(ctx context.Context, log *slog.Logger, meta Meta, opts ...Options) (context.Context, *Run) {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = slog.Default()
	}
	opt := DefaultOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	
	runID := "job_" + correlation.Generate()
	ctx = WithJobRunID(ctx, runID)
	ctx = WithJobName(ctx, meta.JobName)
	ctx = logger.WithMeta(
		ctx, logger.ContextMeta{
			Layer:     "job",
			Component: meta.Component,
			Operation: meta.Operation,
		},
	)
	
	enriched := log.With(
		slog.String("job.run_id", runID),
		slog.String("job.name", meta.JobName),
		slog.String("job.trigger_type", meta.TriggerType),
		slog.String("layer", "job"),
		slog.String("component", meta.Component),
		slog.String("operation", meta.Operation),
	)
	
	r := &Run{
		ctx:       ctx,
		log:       enriched,
		meta:      meta,
		opts:      opt,
		runID:     runID,
		startedAt: time.Now(),
	}
	
	if opt.LogStart && shouldLog(
		opt.SuccessSampleEvery, opt.ShouldLog, sampling.Decision{
			EventType: "job.started",
			Layer:     "job",
			Component: meta.Component,
			Operation: meta.Operation,
		}, opt.RecoverInternally,
	) {
		ev := buildStartedEvent(ctx, runID, meta, opt, r.startedAt)
		safeDo(
			opt.RecoverInternally, func() {
				enriched.LogAttrs(ctx, ev.level, ev.msg, ev.attrs...)
			},
		)
	}
	
	return ctx, r
}

func (r *Run) End(err error) {
	if err != nil {
		r.Fail(err)
		return
	}
	r.Complete()
}

func (r *Run) Fail(err error) {
	if r == nil || !r.ended.CompareAndSwap(false, true) {
		return
	}
	if !r.opts.LogFail {
		return
	}
	endedAt := time.Now()
	if !shouldLog(
		r.opts.SuccessSampleEvery, r.opts.ShouldLog, sampling.Decision{
			EventType: "job.failed",
			Layer:     "job",
			Component: r.meta.Component,
			Operation: r.meta.Operation,
			Duration:  endedAt.Sub(r.startedAt),
			HasError:  err != nil,
		}, r.opts.RecoverInternally,
	) {
		return
	}
	ev := buildFailedEvent(r.ctx, r.runID, r.meta, r.Counts(), r.startedAt, endedAt, err, r.opts)
	safeDo(
		r.opts.RecoverInternally, func() {
			r.log.LogAttrs(r.ctx, ev.level, ev.msg, ev.attrs...)
		},
	)
}

func (r *Run) Complete() {
	if r == nil || !r.ended.CompareAndSwap(false, true) {
		return
	}
	if !r.opts.LogComplete {
		return
	}
	endedAt := time.Now()
	duration := endedAt.Sub(r.startedAt)
	isSlow := r.opts.SlowThreshold > 0 && duration >= r.opts.SlowThreshold
	if !shouldLog(
		r.opts.SuccessSampleEvery, r.opts.ShouldLog, sampling.Decision{
			EventType: "job.completed",
			Layer:     "job",
			Component: r.meta.Component,
			Operation: r.meta.Operation,
			Duration:  duration,
			Slow:      isSlow,
		}, r.opts.RecoverInternally,
	) {
		return
	}
	ev := buildCompletedEvent(r.ctx, r.runID, r.meta, r.Counts(), r.startedAt, endedAt, r.opts)
	safeDo(
		r.opts.RecoverInternally, func() {
			r.log.LogAttrs(r.ctx, ev.level, ev.msg, ev.attrs...)
		},
	)
}

func (r *Run) Retry(meta RetryMeta) {
	if r == nil || !r.opts.LogRetry {
		return
	}
	if !shouldLog(
		r.opts.SuccessSampleEvery, r.opts.ShouldLog, sampling.Decision{
			EventType: "job.retry",
			Layer:     "job",
			Component: r.meta.Component,
			Operation: r.meta.Operation,
		}, r.opts.RecoverInternally,
	) {
		return
	}
	ev := buildRetryEvent(r.ctx, r.runID, r.meta, meta, r.opts)
	safeDo(
		r.opts.RecoverInternally, func() {
			r.log.LogAttrs(r.ctx, ev.level, ev.msg, ev.attrs...)
		},
	)
}

func shouldLog(successEvery uint64, hook sampling.Hook, decision sampling.Decision, recoverInternally bool) bool {
	if !shouldKeepBySample(successEvery, decision) {
		return false
	}
	if hook == nil {
		return true
	}
	allowed := true
	safeDo(
		recoverInternally, func() {
			allowed = hook(decision)
		},
	)
	return allowed
}

func shouldKeepBySample(every uint64, decision sampling.Decision) bool {
	if every <= 1 {
		return true
	}
	if sampling.IsImportant(decision) {
		return true
	}
	if !strings.Contains(strings.ToLower(decision.EventType), "complete") {
		return true
	}
	n := jobCompleteSampleCounter.Add(1)
	return n%every == 0
}

func (r *Run) SetCounts(counts Counts) {
	if r == nil {
		return
	}
	r.processed.Store(counts.Processed)
	r.succeeded.Store(counts.Succeeded)
	r.failed.Store(counts.Failed)
	r.skipped.Store(counts.Skipped)
}

func (r *Run) AddProcessed(n int64) {
	if r != nil {
		r.processed.Add(n)
	}
}

func (r *Run) AddSucceeded(n int64) {
	if r != nil {
		r.succeeded.Add(n)
	}
}

func (r *Run) AddFailed(n int64) {
	if r != nil {
		r.failed.Add(n)
	}
}

func (r *Run) AddSkipped(n int64) {
	if r != nil {
		r.skipped.Add(n)
	}
}

func (r *Run) Counts() Counts {
	if r == nil {
		return Counts{}
	}
	return Counts{
		Processed: r.processed.Load(),
		Succeeded: r.succeeded.Load(),
		Failed:    r.failed.Load(),
		Skipped:   r.skipped.Load(),
	}
}

func (r *Run) Logger() *slog.Logger {
	if r == nil || r.log == nil {
		return slog.Default()
	}
	return r.log
}

func (r *Run) Context() context.Context {
	if r == nil {
		return context.Background()
	}
	return r.ctx
}
