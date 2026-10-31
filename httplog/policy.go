package httplog

import (
	"context"
	"log/slog"
)

func ShouldLog(
	sampler *SuccessSampler, hook DecisionHook, meta DecisionMeta, recoverInternally bool,
) bool {
	if sampler != nil && !sampler.ShouldLog(meta) {
		return false
	}
	if hook == nil {
		return true
	}
	ok, _ := SafeValue(recoverInternally, true, func() bool { return hook(meta) })
	return ok
}

func LogEvent(log *slog.Logger, ctx context.Context, event Event, recoverInternally bool) {
	if log == nil {
		return
	}
	SafeExecute(recoverInternally, func() {
		log.LogAttrs(ctx, event.Level, event.Message, event.Attrs...)
	})
}
