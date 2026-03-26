package sampling

import "testing"

func TestDeterministicSamplingSuccessOnly(t *testing.T) {
	s := NewDeterministic(3)
	d := Decision{EventType: "complete"}
	if s.ShouldKeep(d) {
		t.Fatalf("expected first to be dropped")
	}
	if s.ShouldKeep(d) {
		t.Fatalf("expected second to be dropped")
	}
	if !s.ShouldKeep(d) {
		t.Fatalf("expected third to pass")
	}
}

func TestDeterministicSamplingKeepsImportant(t *testing.T) {
	s := NewDeterministic(1000)
	if !s.ShouldKeep(Decision{EventType: "error", HasError: true}) {
		t.Fatalf("error must be kept")
	}
	if !s.ShouldKeep(Decision{EventType: "complete", Slow: true}) {
		t.Fatalf("slow event must be kept")
	}
	if !s.ShouldKeep(Decision{EventType: "complete", StatusCode: 500}) {
		t.Fatalf(">=400 must be kept")
	}
}

func TestDisabledSamplerKeepsAll(t *testing.T) {
	var s *Deterministic
	if !s.ShouldKeep(Decision{EventType: "complete"}) {
		t.Fatalf("nil sampler should keep all")
	}
}
