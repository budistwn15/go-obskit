package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func (m *Middleware) run() {
	defer m.workerDone()
	ticker := time.NewTicker(m.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]slog.Record, 0, m.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		err := m.ship(m.shipContext(), batch)
		if errors.Is(err, context.Canceled) && m.closed.Load() {
			err = m.ship(m.closeCtx, batch)
		}
		if err != nil {
			m.failed.Add(uint64(len(batch)))
			m.handleErr(err)
		} else {
			m.sent.Add(uint64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-m.stop:
			<-m.enqueuesDone
			for {
				select {
				case rec := <-m.queue:
					batch = append(batch, rec)
					if len(batch) >= m.cfg.BatchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}
		case rec := <-m.queue:
			batch = append(batch, rec)
			if len(batch) >= m.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

func (m *Middleware) shipContext() context.Context {
	if m.closed.Load() {
		return m.closeCtx
	}
	return m.ctx
}

func (m *Middleware) ship(ctx context.Context, batch []slog.Record) error {
	var body bytes.Buffer
	indexName := m.currentIndexName(time.Now().UTC())
	for _, rec := range batch {
		meta := map[string]any{"index": map[string]any{"_index": indexName}}
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		doc := recordToDocumentWithJSONExpansion(rec, m.cfg.StaticFields, m.cfg.ExpandJSONFields)
		docJSON, err := json.Marshal(doc)
		if err != nil {
			return err
		}
		body.Write(metaJSON)
		body.WriteByte('\n')
		body.Write(docJSON)
		body.WriteByte('\n')
	}

	endpoint := m.cfg.Endpoint + "/_bulk"
	var allErr error
	for attempt := 0; attempt <= m.cfg.MaxRetries; attempt++ {
		err := m.sendBulk(ctx, endpoint, body.Bytes())
		if err == nil {
			return nil
		}
		allErr = joinErr(allErr, err)
		if attempt == m.cfg.MaxRetries {
			break
		}
		m.retryErr(attempt+1, err)
		backoff := m.cfg.RetryBackoff * time.Duration(1<<attempt)
		if backoff > m.cfg.MaxBackoff {
			backoff = m.cfg.MaxBackoff
		}
		if err := waitRetry(ctx, backoff); err != nil {
			return errors.Join(allErr, err)
		}
	}
	return allErr
}

func (m *Middleware) currentIndexName(now time.Time) string {
	if m == nil {
		return ""
	}
	if !m.cfg.IndexTimestampSuffix {
		return m.cfg.Index
	}
	layout := m.cfg.IndexTimestampLayout
	if layout == "" {
		layout = "2006.01.02"
	}
	return m.cfg.Index + "-" + now.Format(layout)
}

func (m *Middleware) sendBulk(ctx context.Context, endpoint string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	if m.cfg.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+m.cfg.APIKey)
	}
	if m.cfg.Username != "" {
		req.SetBasicAuth(m.cfg.Username, m.cfg.Password)
	}

	resp, err := m.cfg.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 || resp.StatusCode == 429 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("bulk server status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("bulk rejected status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
