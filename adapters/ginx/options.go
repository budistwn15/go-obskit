package ginx

import "github.com/budistwn15/go-obskit/httplog"

type Options struct {
	httplog.Options
}

func DefaultOptions() Options {
	return Options{
		Options: httplog.DefaultOptions(),
	}
}

// ForensicOptions returns the high-detail parity preset for the Gin adapter.
//
// Stability contract:
// - This preset is treated as a stable cross-version profile.
// - No default value inside this preset should change silently in minor/patch releases.
// - If a value must change, it should be announced as a breaking preset change.
func ForensicOptions() Options {
	return Options{
		Options: httplog.ForensicOptions(),
	}
}

func normalizeOptions(opts Options) Options {
	def := DefaultOptions()
	if opts.CorrelationHeader == "" {
		opts.CorrelationHeader = def.CorrelationHeader
	}
	if opts.MaxBodyBytes <= 0 {
		opts.MaxBodyBytes = def.MaxBodyBytes
	}
	if opts.SuccessSampleEvery == 0 {
		opts.SuccessSampleEvery = def.SuccessSampleEvery
	}
	return opts
}
