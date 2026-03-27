package redact

import "testing"

func FuzzRedactJSONBytes(f *testing.F) {
	rules := DefaultPIIRules()
	f.Add([]byte(`{"email":"john@example.com","password":"secret"}`))
	f.Add([]byte(`{"nested":{"phone":"+6281234567890","nik":"3175090901010001"}}`))
	f.Add([]byte(`{"bad":`))
	f.Add([]byte(`[]`))

	f.Fuzz(func(t *testing.T, in []byte) {
		out, _ := RedactJSONBytes(in, 4096, rules)
		if out == nil {
			t.Fatalf("output should never be nil")
		}
		_ = len(out)
	})
}
