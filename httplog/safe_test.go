package httplog

import "testing"

func TestSafeExecute(t *testing.T) {
	panicked := SafeExecute(true, func() { panic("boom") })
	if !panicked {
		t.Fatalf("expected panic to be recovered")
	}
}

func TestSafeValue(t *testing.T) {
	got, panicked := SafeValue(true, "fallback", func() string { panic("boom") })
	if !panicked {
		t.Fatalf("expected panic to be recovered")
	}
	if got != "fallback" {
		t.Fatalf("unexpected fallback value: %s", got)
	}
}
