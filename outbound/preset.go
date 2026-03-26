package outbound

import "time"

// ForensicOptions enables high-detail outbound tracing.
// Use this profile during incident investigation due higher overhead/noise.
func ForensicOptions() Options {
	opts := DefaultOptions()
	opts.CaptureHeaders = true
	opts.CaptureQuery = true
	opts.CaptureRequestBody = true
	opts.CaptureResponseBody = true
	opts.MaxBodyBytes = 16 * 1024

	opts.LogRequestStart = true
	opts.LogRequestComplete = true
	opts.LogRequestError = true
	opts.LogSuccessHeaders = true
	opts.LogErrorHeaders = true
	opts.LogErrorBodies = true

	opts.SlowThreshold = 500 * time.Millisecond
	opts.SuccessSampleEvery = 5
	return opts
}
