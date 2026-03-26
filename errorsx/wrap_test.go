package errorsx

import (
	"errors"
	"testing"
)

func TestWrapExtractAndUnwrap(t *testing.T) {
	base := errors.New("db timeout")
	err := Wrap(base, Meta{
		Code:      "DB_TIMEOUT",
		Type:      "timeout",
		Layer:     LayerRepository,
		Component: "repo",
		Operation: "save",
		Fields: map[string]any{
			"retry": true,
		},
	})

	if !errors.Is(err, base) {
		t.Fatalf("errors.Is must match wrapped error")
	}

	extracted, ok := Extract(err)
	if !ok {
		t.Fatalf("expected extract success")
	}
	if extracted.Meta.Code != "DB_TIMEOUT" {
		t.Fatalf("unexpected code: %s", extracted.Meta.Code)
	}
}

func TestClassify(t *testing.T) {
	err := Wrap(errors.New("x"), Meta{Layer: LayerContract})
	meta := Classify(err)
	if meta.Layer != LayerContract {
		t.Fatalf("unexpected layer: %s", meta.Layer)
	}
}
