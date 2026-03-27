package httpsink

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func (m *Middleware) run() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]slog.Record, 0, m.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := m.ship(batch); err != nil {
			m.handleErr(err)
		} else {
			m.sent.Add(uint64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-m.stop:
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

func (m *Middleware) ship(batch []slog.Record) error {
	payload, err := marshalBatch(m.cfg.Format, batch, m.cfg.StaticFields)
	if err != nil {
		return err
	}
	var allErr error
	for attempt := 0; attempt <= m.cfg.MaxRetries; attempt++ {
		err := m.send(payload)
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
		time.Sleep(backoff)
	}
	return allErr
}

func (m *Middleware) send(payload []byte) error {
	req, err := http.NewRequest(http.MethodPost, m.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	if m.cfg.Format == FormatJSONArray {
		req.Header.Set("Content-Type", "application/json")
	} else {
		req.Header.Set("Content-Type", "application/x-ndjson")
	}
	for k, v := range m.cfg.Headers {
		if strings.TrimSpace(k) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
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
		return fmt.Errorf("sink server status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("sink rejected status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
