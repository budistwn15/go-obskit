package redact

import (
	"strings"
	"testing"
)

func TestRedactJSONBytes(t *testing.T) {
	rules := DefaultRules()
	in := []byte(`{"username":"u","password":"p"}`)
	out, _ := RedactJSONBytes(in, 1024, rules)
	if strings.Contains(string(out), `"password":"p"`) {
		t.Fatalf("password should be redacted: %s", out)
	}
	if !strings.Contains(string(out), RedactedValue) {
		t.Fatalf("redacted marker not found: %s", out)
	}
}

func TestRedactJSONBytesFallbackOnInvalidJSON(t *testing.T) {
	rules := DefaultRules()
	out, _ := RedactJSONBytes([]byte(`{"password":`), 20, rules)
	if len(out) == 0 {
		t.Fatalf("fallback output should not be empty")
	}
}

func TestRedactJSONBytes_PIIRegexMode(t *testing.T) {
	rules := DefaultPIIRules()
	in := []byte(`{"message":"email john.doe@example.com phone +6281234567890 nik 3175090901010001"}`)
	out, _ := RedactJSONBytes(in, 2048, rules)
	got := string(out)
	if strings.Contains(got, "john.doe@example.com") ||
		strings.Contains(got, "+6281234567890") ||
		strings.Contains(got, "3175090901010001") {
		t.Fatalf("pii regex mode should redact free-text values, got=%s", got)
	}
	if !strings.Contains(got, RedactedValue) {
		t.Fatalf("expected redacted marker in pii regex mode")
	}
}

func TestRedactJSONBytes_PIIRegexDisabledByDefault(t *testing.T) {
	rules := DefaultRules()
	in := []byte(`{"message":"email john.doe@example.com"}`)
	out, _ := RedactJSONBytes(in, 2048, rules)
	got := string(out)
	if !strings.Contains(got, "john.doe@example.com") {
		t.Fatalf("default rules should keep free-text value when regex mode disabled")
	}
}
