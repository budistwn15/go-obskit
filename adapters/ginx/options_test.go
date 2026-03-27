package ginx

import "testing"

func TestForensicOptionsStablePreset(t *testing.T) {
	opts := ForensicOptions()

	if opts.CorrelationHeader != "X-Correlation-ID" {
		t.Fatalf("unexpected correlation header: %q", opts.CorrelationHeader)
	}
	if !opts.CaptureHeaders || !opts.CaptureQuery || !opts.CaptureRequestBody || !opts.CaptureResponseBody {
		t.Fatalf("forensic preset should capture headers/query/bodies")
	}
	if opts.MaxBodyBytes != 16*1024 {
		t.Fatalf("unexpected MaxBodyBytes: %d", opts.MaxBodyBytes)
	}
	if opts.HeaderAllowlist != nil {
		t.Fatalf("forensic preset should capture all headers (allowlist=nil)")
	}

	if !opts.LogRequestStart || !opts.LogRequestComplete || !opts.LogRequestError {
		t.Fatalf("forensic preset should log full request lifecycle")
	}
	if !opts.LogSuccessHeaders || !opts.LogSuccessBodies || !opts.LogErrorHeaders || !opts.LogErrorBodies {
		t.Fatalf("forensic preset should log success+error headers/bodies")
	}

	if !opts.IncludeClientIP || !opts.IncludeUserAgent || !opts.IncludeReferer {
		t.Fatalf("forensic preset should include client ip/user-agent/referer")
	}
	if opts.SuccessSampleEvery != 1 {
		t.Fatalf("forensic preset should keep full success visibility")
	}
}
