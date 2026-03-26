package httplog

import "testing"

func TestNormalizeQueryValuesRedactsSensitiveKeys(t *testing.T) {
	in := map[string][]string{
		"token": {"abc"},
		"page":  {"1"},
	}
	out := NormalizeQueryValues(in, []string{"token"})
	if out["token"] != "***redacted***" {
		t.Fatalf("token should be redacted")
	}
	if out["page"] != "1" {
		t.Fatalf("page should remain intact")
	}
}
