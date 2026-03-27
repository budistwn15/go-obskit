package httpsink

import (
	"context"
	"log/slog"
)

type handler struct {
	next   slog.Handler
	parent *Middleware
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	if h == nil || h.next == nil {
		return false
	}
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, rec slog.Record) (err error) {
	if h == nil || h.next == nil {
		return nil
	}
	if h.parent != nil && h.parent.cfg.RecoverInternally {
		defer func() {
			if recover() != nil {
				err = nil
			}
		}()
	}
	err = h.next.Handle(ctx, rec)
	if h.parent != nil {
		h.parent.enqueue(rec)
	}
	return err
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if h == nil || h.next == nil {
		return h
	}
	return &handler{next: h.next.WithAttrs(attrs), parent: h.parent}
}

func (h *handler) WithGroup(name string) slog.Handler {
	if h == nil || h.next == nil {
		return h
	}
	return &handler{next: h.next.WithGroup(name), parent: h.parent}
}
