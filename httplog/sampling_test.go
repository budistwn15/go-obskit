package httplog

import "testing"

func TestSuccessSamplerDisabled(t *testing.T) {
	s := NewSuccessSampler(1)
	if s != nil {
		t.Fatalf("expected nil sampler when disabled")
	}
}

func TestSuccessSamplerSamplesCompleteSuccess(t *testing.T) {
	s := NewSuccessSampler(3)
	meta := DecisionMeta{
		Event: "complete",
		Response: ResponseMeta{
			StatusCode: 200,
		},
	}
	if s.ShouldLog(meta) {
		t.Fatalf("1st should be sampled out")
	}
	if s.ShouldLog(meta) {
		t.Fatalf("2nd should be sampled out")
	}
	if !s.ShouldLog(meta) {
		t.Fatalf("3rd should pass sample")
	}
}

func TestSuccessSamplerAlwaysKeepsImportantSignals(t *testing.T) {
	s := NewSuccessSampler(1000)

	if !s.ShouldLog(DecisionMeta{
		Event:    "error",
		Response: ResponseMeta{StatusCode: 500},
	}) {
		t.Fatalf("error event must not be sampled out")
	}

	if !s.ShouldLog(DecisionMeta{
		Event: "complete",
		Response: ResponseMeta{
			StatusCode: 200,
		},
		EventMeta: EventMeta{Slow: true},
	}) {
		t.Fatalf("slow complete event must not be sampled out")
	}

	if !s.ShouldLog(DecisionMeta{
		Event: "complete",
		Response: ResponseMeta{
			StatusCode: 503,
		},
	}) {
		t.Fatalf("5xx complete event must not be sampled out")
	}
}
