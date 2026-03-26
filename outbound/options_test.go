package outbound

import "testing"

func TestForensicOptions(t *testing.T) {
	opts := ForensicOptions()
	if !opts.CaptureHeaders || !opts.CaptureRequestBody || !opts.CaptureResponseBody {
		t.Fatalf("forensic options should enable detailed capture")
	}
	if !opts.LogRequestStart || !opts.LogErrorBodies || !opts.LogSuccessHeaders {
		t.Fatalf("forensic options should enable detailed lifecycle logs")
	}
	if opts.SuccessSampleEvery <= 1 {
		t.Fatalf("forensic options should enable conservative success sampling")
	}
}
