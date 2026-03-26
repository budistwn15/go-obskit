package httplog

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

func NormalizeHeaderMap(input map[string][]string) map[string][]string {
	if len(input) == 0 {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(input))
	for k, vals := range input {
		ck := http.CanonicalHeaderKey(k)
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[ck] = cp
	}
	return out
}

func NormalizeQueryValues(values map[string][]string, denylist []string) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	var deny map[string]struct{}
	if len(denylist) > 0 {
		deny = toSet(denylist)
	}
	out := make(map[string]any, len(values))
	for key, vals := range values {
		if len(deny) > 0 {
			if _, ok := deny[strings.ToLower(strings.TrimSpace(key))]; ok {
				out[key] = "***redacted***"
				continue
			}
		}
		if len(vals) == 1 {
			out[key] = vals[0]
			continue
		}
		cp := make([]string, len(vals))
		copy(cp, vals)
		out[key] = cp
	}
	return out
}

func NormalizeQuery(values url.Values, denylist []string) map[string]any {
	return NormalizeQueryValues(values, denylist)
}

func Duration(start time.Time, end time.Time) time.Duration {
	if end.Before(start) {
		return 0
	}
	return end.Sub(start)
}

func NormalizeRequestMeta(in RequestMeta) RequestMeta {
	in.Method = strings.TrimSpace(in.Method)
	in.Scheme = strings.TrimSpace(in.Scheme)
	in.Host = strings.TrimSpace(in.Host)
	in.Path = strings.TrimSpace(in.Path)
	in.Route = strings.TrimSpace(in.Route)
	in.URL = strings.TrimSpace(in.URL)
	in.UserAgent = strings.TrimSpace(in.UserAgent)
	in.Referer = strings.TrimSpace(in.Referer)
	in.ClientIP = strings.TrimSpace(in.ClientIP)
	in.SourceIP = strings.TrimSpace(in.SourceIP)
	in.SourceAddr = strings.TrimSpace(in.SourceAddr)
	in.XForwardedFor = strings.TrimSpace(in.XForwardedFor)
	in.XRealIP = strings.TrimSpace(in.XRealIP)
	in.TargetHost = strings.TrimSpace(in.TargetHost)
	in.AgentName = strings.TrimSpace(in.AgentName)
	in.AgentType = strings.TrimSpace(in.AgentType)
	in.AgentDevice = strings.TrimSpace(in.AgentDevice)
	return EnrichRequestMeta(in)
}

func NormalizeResponseMeta(in ResponseMeta) ResponseMeta {
	if in.StatusCode < 0 {
		in.StatusCode = 0
	}
	if in.SizeBytes < 0 {
		in.SizeBytes = 0
	}
	return in
}

func toSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		out[strings.ToLower(strings.TrimSpace(v))] = struct{}{}
	}
	return out
}
