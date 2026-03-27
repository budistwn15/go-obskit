package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (m *Middleware) Bootstrap(ctx context.Context) error {
	if m == nil || !m.cfg.active() || !m.cfg.Bootstrap {
		return nil
	}
	if err := m.putPipeline(ctx); err != nil {
		return err
	}
	if err := m.putIndexTemplate(ctx); err != nil {
		return err
	}
	if m.cfg.ApplyPipelineToExisting {
		if err := m.applyPipelineToExisting(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (m *Middleware) putPipeline(ctx context.Context) error {
	pipeline := map[string]any{
		"description": "obskit pipeline: parse http body json and extract db.table",
		"processors": []map[string]any{
			{
				"json": map[string]any{
					"field":          "http.request.body",
					"target_field":   "http.request.body_json",
					"ignore_failure": true,
				},
			},
			{
				"json": map[string]any{
					"field":          "http.response.body",
					"target_field":   "http.response.body_json",
					"ignore_failure": true,
				},
			},
			{
				"grok": map[string]any{
					"field": "db.statement",
					"patterns": []string{
						"(?i)(?:from|update|into|join)\\s+`?%{WORD:db.table}`?",
					},
					"ignore_failure": true,
				},
			},
		},
	}
	return m.putJSON(ctx, http.MethodPut, "/_ingest/pipeline/"+m.cfg.PipelineName, pipeline)
}

func (m *Middleware) putIndexTemplate(ctx context.Context) error {
	tpl := map[string]any{
		"index_patterns": []string{m.cfg.IndexPattern},
		"template": map[string]any{
			"settings": map[string]any{
				"index.default_pipeline": m.cfg.PipelineName,
			},
			"mappings": map[string]any{
				"dynamic": true,
				"properties": map[string]any{
					"@timestamp":        map[string]any{"type": "date"},
					"event":             map[string]any{"type": "keyword"},
					"level":             map[string]any{"type": "keyword"},
					"schema.version":    map[string]any{"type": "keyword"},
					"duration_ms":       map[string]any{"type": "long"},
					"threshold_ms":      map[string]any{"type": "long"},
					"slow_threshold_ms": map[string]any{"type": "long"},
					"slow":              map[string]any{"type": "boolean"},
					"correlation_id":    map[string]any{"type": "keyword"},
					"request_id":        map[string]any{"type": "keyword"},
					"trace_id":          map[string]any{"type": "keyword"},
					"span_id":           map[string]any{"type": "keyword"},
					"status_code":       map[string]any{"type": "integer"},
					"http.status_code":  map[string]any{"type": "integer"},
					"source.port":       map[string]any{"type": "integer"},
					"target.port":       map[string]any{"type": "integer"},
					"db.table":          map[string]any{"type": "keyword"},
					"db.query_type":     map[string]any{"type": "keyword"},
					"db.fingerprint":    map[string]any{"type": "keyword"},
				},
			},
		},
		"priority": 200,
	}
	return m.putJSON(ctx, http.MethodPut, "/_index_template/"+m.cfg.TemplateName, tpl)
}

func (m *Middleware) applyPipelineToExisting(ctx context.Context) error {
	indices, err := m.listIndices(ctx)
	if err != nil {
		return err
	}
	for _, idx := range indices {
		payload := map[string]any{"index.default_pipeline": m.cfg.PipelineName}
		if err := m.putJSON(ctx, http.MethodPut, "/"+idx+"/_settings", payload); err != nil {
			return err
		}
	}
	return nil
}

func (m *Middleware) listIndices(ctx context.Context) ([]string, error) {
	resp, err := m.do(ctx, http.MethodGet, "/_cat/indices/"+m.cfg.IndexPattern+"?h=index&format=json", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("list indices status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var rows []map[string]string
	if err := json.Unmarshal(b, &rows); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		idx := strings.TrimSpace(r["index"])
		if idx != "" {
			out = append(out, idx)
		}
	}
	return out, nil
}

func (m *Middleware) putJSON(ctx context.Context, method, path string, payload map[string]any) error {
	return m.sendJSON(ctx, method, path, payload)
}

func (m *Middleware) sendJSON(ctx context.Context, method, path string, payload any) error {
	buf, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := m.do(ctx, method, path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (m *Middleware) do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, m.cfg.Endpoint+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.cfg.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+m.cfg.APIKey)
	}
	if m.cfg.Username != "" {
		req.SetBasicAuth(m.cfg.Username, m.cfg.Password)
	}
	return m.cfg.HTTPClient.Do(req)
}
