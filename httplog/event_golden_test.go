package httplog

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestBuildRequestComplete_GoldenSchema(t *testing.T) {
	event := BuildRequestComplete(
		RequestMeta{Method: "GET", Path: "/health"},
		ResponseMeta{StatusCode: 200},
		EventMeta{CorrelationID: "corr-1", Duration: 12 * time.Millisecond, DurationMS: 12},
	)
	got := make(map[string]any, len(event.Attrs))
	for _, attr := range event.Attrs {
		got[attr.Key] = attr.Value.Any()
	}
	gotBytes, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(gotBytes, &got); err != nil {
		t.Fatal(err)
	}
	wantBytes, err := os.ReadFile("testdata/http_complete.json")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal(wantBytes, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shared HTTP schema mismatch: got=%v want=%v", got, want)
	}
}
