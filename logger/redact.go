package logger

import (
	"log/slog"
	"strings"
)

const DefaultMask = "[REDACTED]"

var defaultSensitiveKeys = []string{
	"authorization",
	"proxy-authorization",
	"cookie",
	"set-cookie",
	"x-api-key",
	"password",
	"passwd",
	"token",
	"secret",
	"apikey",
	"api_key",
	"client_secret",
	"otp",
	"pin",
}

type Redactor struct {
	mask string
	keys map[string]struct{}
}

func NewRedactor(mask string, sensitiveKeys ...string) *Redactor {
	if mask == "" {
		mask = DefaultMask
	}
	keys := make(map[string]struct{}, len(defaultSensitiveKeys)+len(sensitiveKeys))
	for _, k := range defaultSensitiveKeys {
		keys[strings.ToLower(strings.TrimSpace(k))] = struct{}{}
	}
	for _, k := range sensitiveKeys {
		if t := strings.ToLower(strings.TrimSpace(k)); t != "" {
			keys[t] = struct{}{}
		}
	}
	return &Redactor{
		mask: mask,
		keys: keys,
	}
}

func DefaultRedactor() *Redactor {
	return NewRedactor(DefaultMask)
}

func (r *Redactor) RedactMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for k, v := range input {
		if r.IsSensitive(k) {
			out[k] = r.mask
			continue
		}
		out[k] = v
	}
	return out
}

func (r *Redactor) RedactAttr(attr slog.Attr) slog.Attr {
	if r.IsSensitive(attr.Key) {
		return slog.String(attr.Key, r.mask)
	}
	return attr
}

func (r *Redactor) IsSensitive(key string) bool {
	if r == nil {
		return false
	}
	_, ok := r.keys[strings.ToLower(strings.TrimSpace(key))]
	return ok
}

func (r *Redactor) Mask() string {
	if r == nil || r.mask == "" {
		return DefaultMask
	}
	return r.mask
}
