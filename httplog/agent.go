package httplog

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

func EnrichRequestMeta(meta RequestMeta) RequestMeta {
	meta.AgentName, meta.AgentType, meta.AgentDevice = parseUserAgent(meta.UserAgent)

	if meta.SourceIP == "" && meta.ClientIP != "" {
		meta.SourceIP = meta.ClientIP
	}
	if meta.TargetHost == "" {
		meta.TargetHost = meta.Host
	}
	if meta.TargetPort == 0 {
		meta.TargetPort = parsePort(meta.TargetHost, meta.Scheme)
	}
	return meta
}

func FillSourceFromRemoteAddr(meta *RequestMeta, remoteAddr string) {
	if meta == nil || strings.TrimSpace(remoteAddr) == "" {
		return
	}
	meta.SourceAddr = remoteAddr
	if host, port, err := net.SplitHostPort(remoteAddr); err == nil {
		meta.SourceIP = host
		meta.SourcePort = atoi(port)
		return
	}
	meta.SourceIP = remoteAddr
}

func FillTargetFromURL(meta *RequestMeta, u *url.URL) {
	if meta == nil || u == nil {
		return
	}
	host := u.Host
	if host == "" {
		return
	}
	meta.TargetHost = host
	meta.TargetPort = parsePort(host, u.Scheme)
}

func FillTargetFromRequest(meta *RequestMeta, req *http.Request) {
	if meta == nil || req == nil {
		return
	}
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if host == "" {
		return
	}
	meta.TargetHost = host
	meta.TargetPort = parsePort(host, requestScheme(req))
}

func parseUserAgent(ua string) (name, kind, device string) {
	u := strings.ToLower(strings.TrimSpace(ua))
	if u == "" {
		return "", "unknown", "unknown"
	}
	device = "desktop"
	if strings.Contains(u, "mobile") || strings.Contains(u, "android") || strings.Contains(u, "iphone") {
		device = "mobile"
	}
	switch {
	case strings.Contains(u, "postmanruntime"):
		return "postman", "api_client", device
	case strings.Contains(u, "insomnia"):
		return "insomnia", "api_client", device
	case strings.Contains(u, "curl/"):
		return "curl", "cli", "server"
	case strings.Contains(u, "wget/"):
		return "wget", "cli", "server"
	case strings.Contains(u, "httpie"):
		return "httpie", "cli", "server"
	case strings.Contains(u, "mozilla/"):
		return "browser", "browser", device
	case strings.Contains(u, "go-http-client"):
		return "go-http-client", "service", "server"
	case strings.Contains(u, "python-requests"):
		return "python-requests", "service", "server"
	default:
		return "unknown", "unknown", device
	}
}

func parsePort(hostOrHostPort, scheme string) int {
	if _, p, err := net.SplitHostPort(hostOrHostPort); err == nil {
		return atoi(p)
	}
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "https":
		return 443
	case "http":
		return 80
	default:
		return 0
	}
}

func atoi(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

func requestScheme(req *http.Request) string {
	if req == nil {
		return ""
	}
	if xf := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); xf != "" {
		return xf
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}
