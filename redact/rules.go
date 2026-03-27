package redact

import (
	"regexp"
	"strings"
)

const RedactedValue = "***redacted***"

type Rules struct {
	HeaderKeys map[string]struct{}
	JSONKeys   map[string]struct{}
	// ValuePatterns redacts free-text value content when EnabledPatternRedaction is true.
	ValuePatterns []Pattern
	// EnabledPatternRedaction enables regex-based redaction on string values.
	EnabledPatternRedaction bool
}

type Pattern struct {
	Name string
	Expr *regexp.Regexp
}

func DefaultRules() Rules {
	return Rules{
		HeaderKeys: toSet([]string{
			"authorization",
			"cookie",
			"set-cookie",
			"x-api-key",
			"proxy-authorization",
		}),
		JSONKeys: toSet([]string{
			"password",
			"passcode",
			"pin",
			"otp",
			"token",
			"access_token",
			"refresh_token",
			"secret",
			"client_secret",
			"api_key",
			"private_key",
		}),
		ValuePatterns: []Pattern{
			{Name: "email", Expr: regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)},
			{Name: "phone", Expr: regexp.MustCompile(`(?i)\b(?:\+62|62|0)[0-9\-\s]{8,16}\b`)},
			{Name: "nik", Expr: regexp.MustCompile(`\b[0-9]{16}\b`)},
		},
		EnabledPatternRedaction: false,
	}
}

// DefaultPIIRules enables regex-based free-text PII redaction (email/phone/nik).
func DefaultPIIRules() Rules {
	r := DefaultRules()
	r.EnabledPatternRedaction = true
	return r
}

func toSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[strings.ToLower(strings.TrimSpace(v))] = struct{}{}
	}
	return out
}

func isSensitive(set map[string]struct{}, key string) bool {
	_, ok := set[strings.ToLower(strings.TrimSpace(key))]
	return ok
}
