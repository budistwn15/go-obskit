package lokisink

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

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

func (m *Middleware) run() {
	defer close(m.done)
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
	payload, err := m.buildPayload(batch)
	if err != nil {
		return err
	}
	if m.cfg.MaxBatchBytes > 0 && len(payload) > m.cfg.MaxBatchBytes {
		return fmt.Errorf("loki batch exceeds %d bytes", m.cfg.MaxBatchBytes)
	}
	var allErr error
	for attempt := 0; attempt <= m.cfg.MaxRetries; attempt++ {
		err := m.send(ctx, payload)
		if err == nil {
			return nil
		}
		allErr = joinErr(allErr, err)
		if attempt == m.cfg.MaxRetries || !isRetryable(err) {
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

func (m *Middleware) buildPayload(batch []slog.Record) ([]byte, error) {
	stream := map[string]string{}
	for k, v := range m.cfg.Labels {
		if strings.TrimSpace(k) == "" || strings.TrimSpace(v) == "" {
			continue
		}
		stream[k] = v
	}
	values := make([][]string, 0, len(batch))
	for _, rec := range batch {
		line, err := recordLine(rec, m.cfg.StaticFields)
		if err != nil {
			return nil, err
		}
		ts := rec.Time.UTC()
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		values = append(values, []string{fmt.Sprintf("%d", ts.UnixNano()), line})
	}
	push := lokiPush{
		Streams: []lokiStream{{Stream: stream, Values: values}},
	}
	return json.Marshal(push)
}

func (m *Middleware) send(ctx context.Context, payload []byte) error {
	endpoint := m.cfg.Endpoint
	if !strings.Contains(endpoint, "/loki/api/v1/push") {
		endpoint = endpoint + "/loki/api/v1/push"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.cfg.APIKey)
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
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		return fmt.Errorf("loki server status=%d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
		return permanentError{fmt.Errorf("loki rejected status=%d", resp.StatusCode)}
	}
	return nil
}

type permanentError struct{ error }

func isRetryable(err error) bool {
	var permanent permanentError
	return !errors.As(err, &permanent)
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
