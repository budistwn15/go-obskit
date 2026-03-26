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
