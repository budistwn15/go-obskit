package redact

import (
	"net/http"
	"testing"
)

func TestRedactHeaders(t *testing.T) {
	rules := DefaultRules()
	h := http.Header{}
	h.Set("Authorization", "Bearer abc")
	h.Set("X-Request-ID", "r1")
	out := RedactHeaders(h, rules)
	if out.Get("Authorization") != RedactedValue {
		t.Fatalf("authorization must be redacted")
	}
	if out.Get("X-Request-ID") != "r1" {
		t.Fatalf("non-sensitive header should remain")
	}
}

func TestTruncateBytes(t *testing.T) {
	out, truncated := TruncateBytes([]byte("abcdef"), 3)
	if string(out) != "abc" || !truncated {
		t.Fatalf("unexpected truncate result: %q %v", out, truncated)
	}
}
