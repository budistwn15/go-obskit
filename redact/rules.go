package redact

import "strings"

const RedactedValue = "***redacted***"

type Rules struct {
	HeaderKeys map[string]struct{}
	JSONKeys   map[string]struct{}
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
	}
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
