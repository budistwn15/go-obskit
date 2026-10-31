package redact

import "testing"

func BenchmarkRedactMalformedJSON(b *testing.B) {
	input := []byte(`{"password":"secret","token":`)
	rules := DefaultRules()
	for b.Loop() {
		_, _ = RedactJSONBytes(input, 4*1024, rules)
	}
}
