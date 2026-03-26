package logger

import "testing"

func TestRedactMap(t *testing.T) {
	r := DefaultRedactor()
	input := map[string]any{
		"username":      "john",
		"Authorization": "Bearer token",
		"X-API-Key":     "abc",
		"password":      "secret",
	}

	out := r.RedactMap(input)

	if out["username"] != "john" {
		t.Fatalf("non-sensitive value should stay unchanged")
	}
	if out["Authorization"] != DefaultMask {
		t.Fatalf("authorization should be redacted")
	}
	if out["X-API-Key"] != DefaultMask {
		t.Fatalf("x-api-key should be redacted")
	}
	if out["password"] != DefaultMask {
		t.Fatalf("password should be redacted")
	}
}
