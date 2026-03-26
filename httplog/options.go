package httplog

import (
	"time"

	"github.com/budistwn15/go-obskit/redact"
)

type DecisionHook func(meta DecisionMeta) bool

type Options struct {
	CorrelationHeader string
	
	CaptureHeaders      bool
	CaptureQuery        bool
	CaptureRequestBody  bool
	CaptureResponseBody bool
	MaxBodyBytes        int
	
	HeaderAllowlist  []string
	HeaderDenylist   []string
	BodyJSONDenylist []string
	
	LogRequestStart    bool
	LogRequestComplete bool
	LogRequestError    bool
	
	LogSuccessHeaders bool
	LogErrorHeaders   bool
	LogErrorBodies    bool
	
	IncludeClientIP  bool
	IncludeUserAgent bool
	IncludeReferer   bool
	
	SlowRequestThreshold time.Duration
	RecoverInternally    bool
	
	ShouldLogStart    DecisionHook
	ShouldLogComplete DecisionHook
	ShouldLogError    DecisionHook
	
	// SuccessSampleEvery controls deterministic sampling for non-slow successful
	// "complete" events. Value <= 1 disables sampling.
	SuccessSampleEvery uint64
}

func DefaultOptions() Options {
	rules := redact.DefaultRules()
	return Options{
		CorrelationHeader: "X-Correlation-ID",
		
		CaptureHeaders:      false,
		CaptureQuery:        true,
		CaptureRequestBody:  false,
		CaptureResponseBody: false,
		MaxBodyBytes:        4 * 1024,
		
		HeaderAllowlist: []string{
			"Content-Type",
			"X-Request-ID",
			"X-Correlation-ID",
		},
		HeaderDenylist:   keys(rules.HeaderKeys),
		BodyJSONDenylist: keys(rules.JSONKeys),
		
		LogRequestStart:    false,
		LogRequestComplete: true,
		LogRequestError:    true,
		
		LogSuccessHeaders: false,
		LogErrorHeaders:   true,
		LogErrorBodies:    false,
		
		IncludeClientIP:  true,
		IncludeUserAgent: true,
		IncludeReferer:   false,
		
		SlowRequestThreshold: 1 * time.Second,
		RecoverInternally:    true,
		SuccessSampleEvery:   1,
	}
}

func keys(input map[string]struct{}) []string {
	out := make([]string, 0, len(input))
	for k := range input {
		out = append(out, k)
	}
	return out
}
