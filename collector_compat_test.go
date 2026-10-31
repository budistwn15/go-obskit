package obskit_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	obskit "github.com/budistwn15/go-obskit"
)

func TestJSONLineCollectorCompatibility(t *testing.T) {
	var output bytes.Buffer
	log := obskit.New(obskit.Config{
		Service:     "billing-api",
		Environment: "production",
		Output:      &output,
	})
	log.Info("request complete", "http.response.status_code", 200)

	line := output.String()
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("Filebeat and Logstash require one newline-delimited JSON event, got %q", line)
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event); err != nil {
		t.Fatalf("collector cannot decode JSON line: %v", err)
	}
	for _, key := range []string{"time", "level", "msg", "service.name", "service.environment"} {
		if _, ok := event[key].(string); !ok {
			t.Fatalf("%s must be a string, got %T", key, event[key])
		}
	}
	if _, ok := event["http.response.status_code"].(float64); !ok {
		t.Fatalf("http.response.status_code must remain numeric, got %T", event["http.response.status_code"])
	}
}
