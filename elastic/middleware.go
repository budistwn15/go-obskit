package elastic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/budistwn15/go-obskit/logger"
)

type Stats struct {
	Enqueued uint64
	Dropped  uint64
	Sent     uint64
	Failed   uint64
	Retried  uint64
}

type Middleware struct {
	cfg       Config
	statusLog *slog.Logger

	queue chan slog.Record
	stop  chan struct{}
	wg    sync.WaitGroup
	once  sync.Once

	monitorMu sync.RWMutex
	monitor   ConnectionStatus

	enqueued atomic.Uint64
	dropped  atomic.Uint64
	sent     atomic.Uint64
	failed   atomic.Uint64
	retried  atomic.Uint64
}

func NewMiddleware(cfg Config) *Middleware {
	cfg = normalizeConfig(cfg)
	m := &Middleware{cfg: cfg}
	m.statusLog = newStatusLogger(cfg)
	if !cfg.active() {
		if cfg.Enabled {
			m.emitStatus(
				slog.LevelWarn, "elastic sink disabled: incomplete config",
				slog.Bool("elastic.enabled", cfg.Enabled),
				slog.String("elastic.endpoint", cfg.Endpoint),
				slog.String("elastic.index", cfg.Index),
			)
		}
		return m
	}
	m.queue = make(chan slog.Record, cfg.QueueSize)
	m.stop = make(chan struct{})
	m.emitStatus(
		slog.LevelInfo, "elastic sink enabled",
		slog.String("elastic.endpoint", cfg.Endpoint),
		slog.String("elastic.index", cfg.Index),
		slog.Int("elastic.queue_size", cfg.QueueSize),
		slog.Int("elastic.batch_size", cfg.BatchSize),
	)
	m.wg.Add(1)
	go m.run()
	if cfg.EnableMonitor {
		m.wg.Add(1)
		go m.runMonitor()
	}
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
	m.once.Do(func() { close(m.stop) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.wg.Wait()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		m.emitStatus(slog.LevelInfo, "elastic sink closed")
		return nil
	}
}

func (m *Middleware) Stats() Stats {
	if m == nil {
		return Stats{}
	}
	return Stats{
		Enqueued: m.enqueued.Load(),
		Dropped:  m.dropped.Load(),
		Sent:     m.sent.Load(),
		Failed:   m.failed.Load(),
		Retried:  m.retried.Load(),
	}
}

func (m *Middleware) enqueue(rec slog.Record) {
	if m == nil || !m.cfg.active() {
		return
	}
	r := rec.Clone()
	if m.cfg.BlockOnQueueFull {
		select {
		case m.queue <- r:
			m.enqueued.Add(1)
		case <-m.stop:
			m.dropped.Add(1)
		}
		return
	}
	select {
	case m.queue <- r:
		m.enqueued.Add(1)
	default:
		m.dropped.Add(1)
	}
}

func (m *Middleware) handleErr(err error) {
	if err == nil {
		return
	}
	m.failed.Add(1)
	m.emitStatus(
		slog.LevelWarn, "elastic sink error",
		slog.String("error.message", err.Error()),
	)
	if m.cfg.OnError != nil {
		safeCall(m.cfg.RecoverInternally, func() { m.cfg.OnError(err) })
	}
}

func (m *Middleware) retryErr(attempt int, err error) {
	m.retried.Add(1)
	m.emitStatus(
		slog.LevelWarn, "elastic sink retry",
		slog.Int("retry.attempt", attempt),
		slog.String("error.message", err.Error()),
	)
	m.handleErr(fmt.Errorf("elastic retry %d: %w", attempt, err))
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
		slog.String("component", "elastic_sink"),
		slog.String("layer", "observability"),
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
