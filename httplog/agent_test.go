package httplog

import (
	"net/http"
	"net/url"
	"testing"
)

func TestEnrichRequestMetaAgentAndTarget(t *testing.T) {
	meta := EnrichRequestMeta(RequestMeta{
		Scheme:    "https",
		Host:      "api.internal.local",
		UserAgent: "PostmanRuntime/7.43.0",
		ClientIP:  "10.10.1.54",
	})
	if meta.AgentName != "postman" || meta.AgentType != "api_client" {
		t.Fatalf("expected postman agent parsing, got=%s/%s", meta.AgentName, meta.AgentType)
	}
	if meta.TargetPort != 443 {
		t.Fatalf("expected target port 443")
	}
	if meta.SourceIP != "10.10.1.54" {
		t.Fatalf("expected source ip fallback from client ip")
	}
}

func TestFillSourceFromRemoteAddr(t *testing.T) {
	meta := RequestMeta{}
	FillSourceFromRemoteAddr(&meta, "10.10.1.54:53718")
	if meta.SourceIP != "10.10.1.54" || meta.SourcePort != 53718 {
		t.Fatalf("unexpected source parsing: %+v", meta)
	}
}

func TestFillTargetFromURL(t *testing.T) {
	u, _ := url.Parse("https://payments.internal/v1/charge")
	meta := RequestMeta{}
	FillTargetFromURL(&meta, u)
	if meta.TargetPort != 443 {
		t.Fatalf("expected 443 target port")
	}
}

func TestFillTargetFromRequest(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/a", nil)
	req.Host = "example.com:8080"
	meta := RequestMeta{}
	FillTargetFromRequest(&meta, req)
	if meta.TargetPort != 8080 {
		t.Fatalf("expected 8080 target port")
	}
}
