package httplog

import "github.com/budistwn15/go-obskit/sampling"

// SuccessSampler deterministically samples high-volume successful complete logs.
// Important signals (errors and slow events) always pass through.
type SuccessSampler struct {
	inner *sampling.Deterministic
}

func NewSuccessSampler(every uint64) *SuccessSampler {
	if every <= 1 {
		return nil
	}
	return &SuccessSampler{inner: sampling.NewDeterministic(every)}
}

func (s *SuccessSampler) ShouldLog(meta DecisionMeta) bool {
	if s == nil || s.inner == nil {
		return true
	}
	return s.inner.ShouldKeep(
		sampling.Decision{
			EventType:  meta.Event,
			Layer:      meta.EventMeta.Layer,
			Component:  meta.EventMeta.Component,
			Operation:  meta.EventMeta.Operation,
			StatusCode: meta.Response.StatusCode,
			Duration:   meta.EventMeta.Duration,
			HasError:   meta.Err != nil || meta.EventMeta.ErrorKind != "" || meta.EventMeta.ErrorMessage != "",
			Slow:       meta.EventMeta.Slow,
		},
	)
}
