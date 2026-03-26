package httplog

import "testing"

func TestBuildRequestComplete(t *testing.T) {
	req := RequestMeta{
		Method: "GET",
		Path:   "/health",
		URL:    "http://example.com/health",
	}
	res := ResponseMeta{
		StatusCode: 200,
	}
	ev := EventMeta{
		DurationMS: 12,
		Layer:      "handler",
	}
	event := BuildRequestComplete(req, res, ev)
	if event.Message == "" {
		t.Fatalf("event message should not be empty")
	}
	if len(event.Attrs) == 0 {
		t.Fatalf("event attrs should not be empty")
	}
}
