package outbound

import (
	"time"

	"github.com/budistwn15/go-obskit/httplog"
)

type DecisionHook func(meta httplog.DecisionMeta) bool

type Options struct {
	CorrelationHeader string
	
	CaptureQuery        bool
	CaptureHeaders      bool
	CaptureRequestBody  bool
	CaptureResponseBody bool
	MaxBodyBytes        int
	
	HeaderAllowlist  []string
	HeaderDenylist   []string
	BodyJSONDenylist []string
	
	LogSuccessHeaders bool
	LogErrorHeaders   bool
	LogErrorBodies    bool
	
	LogRequestStart    bool
	LogRequestComplete bool
	LogRequestError    bool
	
	SlowThreshold time.Duration
	
	ShouldLogStart    DecisionHook
	ShouldLogComplete DecisionHook
	ShouldLogError    DecisionHook
	
	// SuccessSampleEvery controls deterministic sampling for non-slow successful
	// complete events. Value <= 1 disables sampling.
	SuccessSampleEvery uint64
	
	RecoverInternally bool
}

func DefaultOptions() Options {
	base := httplog.DefaultOptions()
	return Options{
		CorrelationHeader: base.CorrelationHeader,
		
		CaptureQuery:        true,
		CaptureHeaders:      false,
		CaptureRequestBody:  false,
		CaptureResponseBody: false,
		MaxBodyBytes:        base.MaxBodyBytes,
		
		HeaderAllowlist:  base.HeaderAllowlist,
		HeaderDenylist:   base.HeaderDenylist,
		BodyJSONDenylist: base.BodyJSONDenylist,
		
		LogSuccessHeaders: false,
		LogErrorHeaders:   true,
		LogErrorBodies:    false,
		
		LogRequestStart:    false,
		LogRequestComplete: true,
		LogRequestError:    true,
		
		SlowThreshold:      1 * time.Second,
		SuccessSampleEvery: 1,
		
		RecoverInternally: true,
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
	if opts.HeaderAllowlist == nil {
		opts.HeaderAllowlist = def.HeaderAllowlist
	}
	if opts.HeaderDenylist == nil {
		opts.HeaderDenylist = def.HeaderDenylist
	}
	if opts.BodyJSONDenylist == nil {
		opts.BodyJSONDenylist = def.BodyJSONDenylist
	}
	if opts.SlowThreshold <= 0 {
		opts.SlowThreshold = def.SlowThreshold
	}
	if opts.SuccessSampleEvery == 0 {
		opts.SuccessSampleEvery = def.SuccessSampleEvery
	}
	return opts
}
