package httplog

import (
	"testing"
	"time"
)

func TestDurationHelpers(t *testing.T) {
	if got := DurationMS(1500 * time.Millisecond); got != 1500 {
		t.Fatalf("unexpected duration ms: %d", got)
	}
	if !IsSlowRequest(2*time.Second, 1*time.Second) {
		t.Fatalf("expected slow request")
	}
	if IsSlowRequest(500*time.Millisecond, 1*time.Second) {
		t.Fatalf("unexpected slow detection")
	}
}
