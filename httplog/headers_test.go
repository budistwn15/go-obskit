package httplog

import "testing"

func TestFilterHeaders(t *testing.T) {
	in := map[string][]string{
		"Authorization": {"Bearer token"},
		"X-Request-ID":  {"req-1"},
		"X-Extra":       {"v"},
	}
	out := FilterHeaders(in, []string{"X-Request-ID", "Authorization"}, nil)
	if out["Authorization"] != "***redacted***" {
		t.Fatalf("authorization should be redacted")
	}
	if out["X-Request-Id"] != "req-1" {
		t.Fatalf("x-request-id should be kept")
	}
	if _, ok := out["X-Extra"]; ok {
		t.Fatalf("x-extra should not pass allowlist")
	}
}
