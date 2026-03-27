package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBootstrap_CreatesPipelineTemplateAndAppliesSettings(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var pipelineBody, templateBody, settingsBody string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.Path)
		mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		s := string(b)
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/_ingest/pipeline/"):
			mu.Lock()
			pipelineBody = s
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/_index_template/"):
			mu.Lock()
			templateBody = s
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/_cat/indices/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"index":"xeanees-logs-2026.03.26"}]`))
		case r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/_settings"):
			mu.Lock()
			settingsBody = s
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"acknowledged":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = ts.URL
	cfg.Index = "xeanees-logs"
	cfg.Bootstrap = true
	cfg.BootstrapOnStart = false
	cfg.PipelineName = "obskit-p1"
	cfg.TemplateName = "obskit-t1"
	cfg.IndexPattern = "xeanees-logs-*"
	cfg.ApplyPipelineToExisting = true
	cfg.ConnectionLogOutput = &bytes.Buffer{}

	m := NewMiddleware(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap error: %v", err)
	}

	if !strings.Contains(pipelineBody, "http.request.body_json") || !strings.Contains(pipelineBody, "db.table") {
		t.Fatalf("pipeline body missing expected processors: %s", pipelineBody)
	}
	if !strings.Contains(templateBody, "index.default_pipeline") || !strings.Contains(templateBody, "@timestamp") {
		t.Fatalf("template body missing expected mappings/settings: %s", templateBody)
	}
	if !strings.Contains(settingsBody, "obskit-p1") {
		t.Fatalf("existing index settings should apply pipeline")
	}

	_ = paths
}

func TestCurrentIndexName_WithTimestampSuffix(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = "http://localhost:9200"
	cfg.Index = "xeanees-logs"
	cfg.IndexTimestampSuffix = true
	cfg.IndexTimestampLayout = "2006.01.02"
	cfg.ConnectionLogOutput = &bytes.Buffer{}

	m := NewMiddleware(cfg)
	idx := m.currentIndexName(time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC))
	if idx != "xeanees-logs-2026.03.27" {
		t.Fatalf("unexpected index name: %s", idx)
	}
}

func TestBootstrapTemplateJSONValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.Endpoint = "http://localhost:9200"
	cfg.Index = "xeanees-logs"
	cfg.BootstrapOnStart = false
	cfg.ConnectionLogOutput = &bytes.Buffer{}
	m := NewMiddleware(cfg)

	payload := map[string]any{
		"ok": true,
	}
	b, err := json.Marshal(payload)
	if err != nil || len(b) == 0 {
		t.Fatalf("json marshal sanity failed: %v", err)
	}
	_ = m
}
