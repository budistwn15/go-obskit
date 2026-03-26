package nethttp

import "testing"

func TestForensicOptions(t *testing.T) {
	opts := ForensicOptions()
	if !opts.CaptureHeaders || !opts.CaptureRequestBody || !opts.CaptureResponseBody {
		t.Fatalf("forensic options should enable detailed capture")
	}
	if !opts.LogRequestStart || !opts.LogErrorBodies || !opts.LogSuccessHeaders || !opts.LogSuccessBodies {
		t.Fatalf("forensic options should enable detailed lifecycle logs")
	}
}
