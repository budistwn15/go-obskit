package httplog

import (
	"strings"
	"testing"
)

func TestCaptureBodyTruncation(t *testing.T) {
	res := CaptureBody("text/plain", []byte("abcdefghijklmnopqrstuvwxyz"), 5, nil)
	if res.Value != "abcde" {
		t.Fatalf("unexpected truncated value: %s", res.Value)
	}
	if !res.Truncated {
		t.Fatalf("expected truncated=true")
	}
}

func TestCaptureBodyJSONRedactionFallback(t *testing.T) {
	res := CaptureBody("application/json", []byte(`{"password":`), 64, nil)
	if res.Value == "" {
		t.Fatalf("expected safe fallback value")
	}
	if !strings.Contains(res.Value, "redacted") {
		t.Fatalf("expected redacted fallback, got=%s", res.Value)
	}
}

func TestCaptureBodySkipUnsafeType(t *testing.T) {
	res := CaptureBody("application/octet-stream", []byte{0x00, 0x01}, 64, nil)
	if !res.Skipped {
		t.Fatalf("expected body capture skipped")
	}
}
