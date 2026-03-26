package httplog

import "time"

// ForensicOptions enables a high-detail incident-tracing profile.
// Use this only for targeted troubleshooting windows due increased log volume.
func ForensicOptions() Options {
	opts := DefaultOptions()
	opts.CaptureHeaders = true
	opts.CaptureQuery = true
	opts.CaptureRequestBody = true
	opts.CaptureResponseBody = true
	opts.MaxBodyBytes = 16 * 1024
	opts.HeaderAllowlist = nil // capture all headers (denylist still redacted)

	opts.LogRequestStart = true
	opts.LogRequestComplete = true
	opts.LogRequestError = true
	opts.LogSuccessHeaders = true
	opts.LogSuccessBodies = true
	opts.LogErrorHeaders = true
	opts.LogErrorBodies = true

	opts.IncludeClientIP = true
	opts.IncludeUserAgent = true
	opts.IncludeReferer = true

	opts.SlowRequestThreshold = 500 * time.Millisecond
	opts.SuccessSampleEvery = 1
	return opts
}
