package lokisink

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/budistwn15/go-obskit/v2/internal/slogsnapshot"
	"github.com/budistwn15/go-obskit/v2/logger"
)

type Stats struct {
	Enqueued  uint64
	Dropped   uint64
	Sent      uint64
	Failed    uint64
	Retried   uint64
	Oversized uint64
}

type Middleware struct {
	cfg       Config
	statusLog *slog.Logger

	queue        chan slog.Record
	stop         chan struct{}
	enqueuesDone chan struct{}
	done         chan struct{}
	once         sync.Once
	enqueueMu    sync.RWMutex
	closed       atomic.Bool
	ctx          context.Context
	cancel       context.CancelFunc
	closeCtx     context.Context

	enqueued  atomic.Uint64
	dropped   atomic.Uint64
	sent      atomic.Uint64
	failed    atomic.Uint64
	retried   atomic.Uint64
	oversized atomic.Uint64
}

func NewMiddleware(cfg Config) *Middleware {
	cfg = normalizeConfig(cfg)
	m := &Middleware{cfg: cfg}
	m.statusLog = newStatusLogger(cfg)
	if !cfg.active() {
		if cfg.Enabled {
			m.emitStatus(slog.LevelWarn, "loki sink disabled: incomplete config", slog.String("sink.endpoint", cfg.Endpoint))
		}
		return m
	}
	m.queue = make(chan slog.Record, cfg.QueueSize)
	m.stop = make(chan struct{})
	m.enqueuesDone = make(chan struct{})
	m.done = make(chan struct{})
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.emitStatus(
		slog.LevelInfo, "loki sink enabled",
		slog.String("sink.endpoint", cfg.Endpoint),
		slog.Int("sink.queue_size", cfg.QueueSize),
		slog.Int("sink.batch_size", cfg.BatchSize),
	)
	go m.run()
	return m
}

func (m *Middleware) Wrap(next slog.Handler) slog.Handler {
	if next == nil {
		next = slog.NewJSONHandler(io.Discard, nil)
	}
	if m == nil || !m.cfg.active() {
		return next
	}
	return &handler{next: next, parent: m}
}

func (m *Middleware) LoggerMiddleware() logger.HandlerMiddleware {
	return m.Wrap
}

func (m *Middleware) Close(ctx context.Context) error {
	if m == nil || !m.cfg.active() {
		return nil
	}
	m.once.Do(func() {
		m.closeCtx = ctx
		m.closed.Store(true)
		m.cancel()
		close(m.stop)
		m.enqueueMu.Lock()
		close(m.enqueuesDone)
		m.enqueueMu.Unlock()
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.done:
		m.emitStatus(slog.LevelInfo, "loki sink closed")
		return nil
	}
}

func (m *Middleware) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	return Stats{
		Enqueued:  m.enqueued.Load(),
		Dropped:   m.dropped.Load(),
		Sent:      m.sent.Load(),
		Failed:    m.failed.Load(),
		Retried:   m.retried.Load(),
		Oversized: m.oversized.Load(),
	}
}

func (m *Middleware) enqueue(rec slog.Record) {
	if m == nil || !m.cfg.active() {
		return
	}
	m.enqueueMu.RLock()
	if m.closed.Load() {
		m.dropped.Add(1)
		m.enqueueMu.RUnlock()
		return
	}
	r, size := slogsnapshot.Record(rec)
	if m.cfg.MaxRecordBytes > 0 && size > m.cfg.MaxRecordBytes {
		m.oversized.Add(1)
		m.dropped.Add(1)
		m.enqueueMu.RUnlock()
		return
	}
	if m.cfg.BlockOnQueueFull {
		select {
		case m.queue <- r:
			m.enqueued.Add(1)
		case <-m.stop:
			m.dropped.Add(1)
		}
		m.enqueueMu.RUnlock()
		return
	}
	select {
	case m.queue <- r:
		m.enqueued.Add(1)
	default:
		m.dropped.Add(1)
	}
	m.enqueueMu.RUnlock()
}

func (m *Middleware) handleErr(err error) {
	if err == nil {
		return
	}
	m.emitStatus(slog.LevelWarn, "loki sink error", slog.String("error.message", err.Error()))
	if m.cfg.OnError != nil {
		safeCall(m.cfg.RecoverInternally, func() { m.cfg.OnError(err) })
	}
}

func (m *Middleware) retryErr(attempt int, err error) {
	m.retried.Add(1)
	m.emitStatus(
		slog.LevelWarn, "loki sink retry",
		slog.Int("retry.attempt", attempt),
		slog.String("error.message", err.Error()),
	)
}

func joinErr(base error, next error) error {
	if base == nil {
		return next
	}
	if next == nil {
		return base
	}
	return errors.Join(base, next)
}

func newStatusLogger(cfg Config) *slog.Logger {
	if !cfg.ConnectionLogToStdout {
		return nil
	}
	h := slog.NewJSONHandler(cfg.ConnectionLogOutput, &slog.HandlerOptions{Level: cfg.ConnectionLogLevel})
	return slog.New(h).With(
		slog.String("labels.component", "loki_sink"),
		slog.String("labels.layer", "observability"),
	)
}

func (m *Middleware) emitStatus(level slog.Level, msg string, attrs ...slog.Attr) {
	if m == nil || m.statusLog == nil {
		return
	}
	safeCall(m.cfg.RecoverInternally, func() {
		m.statusLog.LogAttrs(context.Background(), level, msg, attrs...)
	})
}
