package joblog

import "time"
import "github.com/budistwn15/go-obskit/sampling"

type Options struct {
	LogStart           bool
	LogComplete        bool
	LogFail            bool
	LogRetry           bool
	IncludeCounts      bool
	IncludeTiming      bool
	SlowThreshold      time.Duration
	SuccessSampleEvery uint64
	ShouldLog          sampling.Hook
	RecoverInternally  bool
}

func DefaultOptions() Options {
	return Options{
		LogStart:           true,
		LogComplete:        true,
		LogFail:            true,
		LogRetry:           true,
		IncludeCounts:      true,
		IncludeTiming:      true,
		SlowThreshold:      0,
		SuccessSampleEvery: 1,
		RecoverInternally:  true,
	}
}
