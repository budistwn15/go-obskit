package nethttp

import (
	"net/http"

	"github.com/budistwn15/go-obskit/httplog"
)

type Options struct {
	httplog.Options
	RouteExtractor func(*http.Request) string
}

func DefaultOptions() Options {
	return Options{
		Options: httplog.DefaultOptions(),
	}
}

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
