package sampling

import (
	"strings"
	"sync/atomic"
	"time"
)

type Decision struct {
	EventType  string
	Layer      string
	Component  string
	Operation  string
	StatusCode int
	Duration   time.Duration
	HasError   bool
	Slow       bool
}

type Hook func(Decision) bool

type Deterministic struct {
	every   uint64
	counter atomic.Uint64
}

func NewDeterministic(every uint64) *Deterministic {
	if every <= 1 {
		return nil
	}
	return &Deterministic{every: every}
}

func (s *Deterministic) ShouldKeep(d Decision) bool {
	if s == nil || s.every <= 1 {
		return true
	}
	if IsImportant(d) {
		return true
	}
	if !isSuccessCandidate(d) {
		return true
	}
	n := s.counter.Add(1)
	return n%s.every == 0
}

func IsImportant(d Decision) bool {
	if d.HasError || d.Slow {
		return true
	}
	if d.StatusCode >= 400 {
		return true
	}
	switch d.EventType {
	case "error", "failed", "retry":
		return true
	default:
		return false
	}
}

func isSuccessCandidate(d Decision) bool {
	if d.HasError || d.Slow || d.StatusCode >= 400 {
		return false
	}
	ev := strings.ToLower(strings.TrimSpace(d.EventType))
	return strings.Contains(ev, "complete")
}
