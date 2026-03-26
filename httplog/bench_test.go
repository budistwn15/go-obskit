package httplog

import "testing"

func BenchmarkBuildRequestCompleteMinimal(b *testing.B) {
	req := RequestMeta{
		Method: "GET",
		Scheme: "https",
		Host:   "example.com",
		Path:   "/health",
		URL:    "https://example.com/health",
	}
	res := ResponseMeta{
		StatusCode: 200,
		SizeBytes:  2,
	}
	ev := EventMeta{DurationMS: 3}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildRequestComplete(req, res, ev)
	}
}

func BenchmarkCaptureBodyJSONBounded(b *testing.B) {
	body := []byte(`{"username":"john","password":"secret","token":"abc","status":"ok"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CaptureBody("application/json", body, 64, nil)
	}
}
