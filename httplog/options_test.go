package httplog

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.CorrelationHeader != "X-Correlation-ID" {
		t.Fatalf("unexpected correlation header: %s", opts.CorrelationHeader)
	}
	if opts.MaxBodyBytes <= 0 {
		t.Fatalf("max body bytes should be > 0")
	}
	if opts.CaptureRequestBody || opts.CaptureResponseBody {
		t.Fatalf("body capture must be disabled by default")
	}
	if !opts.LogRequestComplete || !opts.LogRequestError || opts.LogRequestStart {
		t.Fatalf("unexpected default request event toggles")
	}
	if opts.SlowRequestThreshold != time.Second {
		t.Fatalf("unexpected slow threshold: %v", opts.SlowRequestThreshold)
	}
}

func TestForensicOptions(t *testing.T) {
	opts := ForensicOptions()
	if !opts.CaptureHeaders || !opts.CaptureRequestBody || !opts.CaptureResponseBody {
		t.Fatalf("forensic options should enable detailed capture")
	}
	if !opts.LogRequestStart || !opts.LogErrorBodies || !opts.LogSuccessHeaders || !opts.LogSuccessBodies {
		t.Fatalf("forensic options should enable detailed lifecycle logs")
	}
	if opts.SuccessSampleEvery != 1 {
		t.Fatalf("forensic options should keep full success visibility")
	}
	if opts.MaxBodyBytes < 4096 {
		t.Fatalf("forensic options should keep bounded body capture with practical size")
	}
}
