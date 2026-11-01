// Package nethttp provides the preferred incoming net/http logging middleware.
package nethttp

import (
	"net/http"

	"github.com/budistwn15/go-obskit/v2/httplog"
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
	opts.RecoverInternally = true
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
