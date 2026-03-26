package elastic

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type ConnectionStatus struct {
	Up                  bool
	LastCheckedAt       time.Time
	LastError           string
	ConsecutiveFailures int64
	Endpoint            string
}

func (m *Middleware) MonitorStatus() ConnectionStatus {
	if m == nil {
		return ConnectionStatus{}
	}
	m.monitorMu.RLock()
	defer m.monitorMu.RUnlock()
	return m.monitor
}

func (m *Middleware) HealthCheck(ctx context.Context) ConnectionStatus {
	if m == nil || !m.cfg.active() {
		return ConnectionStatus{}
	}
	st := m.checkConnection(ctx)
	m.updateMonitor(st)
	return st
}

func (m *Middleware) runMonitor() {
	defer m.wg.Done()
	ticker := time.NewTicker(m.cfg.MonitorInterval)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
	st := m.checkConnection(ctx)
	cancel()
	m.updateMonitor(st)

	for {
		select {
		case <-m.stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
			st := m.checkConnection(ctx)
			cancel()
			m.updateMonitor(st)
		}
	}
}

func (m *Middleware) checkConnection(ctx context.Context) ConnectionStatus {
	st := ConnectionStatus{Endpoint: m.cfg.Endpoint}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.Endpoint+m.cfg.MonitorPath, nil)
	if err != nil {
		st.LastCheckedAt = time.Now().UTC()
		st.LastError = err.Error()
		return st
	}
	if m.cfg.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+m.cfg.APIKey)
	}
	if m.cfg.Username != "" {
		req.SetBasicAuth(m.cfg.Username, m.cfg.Password)
	}

	resp, err := m.cfg.HTTPClient.Do(req)
	if err != nil {
		st.LastCheckedAt = time.Now().UTC()
		st.LastError = err.Error()
		return st
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))

	st.LastCheckedAt = time.Now().UTC()
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		st.Up = true
		return st
	}
	st.LastError = fmt.Sprintf("monitor status=%d", resp.StatusCode)
	return st
}

func (m *Middleware) updateMonitor(st ConnectionStatus) {
	if m == nil {
		return
	}
	var cb func(ConnectionStatus)
	var shouldLog bool
	m.monitorMu.Lock()
	if st.Up {
		st.ConsecutiveFailures = 0
	} else {
		st.ConsecutiveFailures = m.monitor.ConsecutiveFailures + 1
	}
	changed := monitorChanged(m.monitor, st)
	shouldLog = changed || m.cfg.ConnectionLogAllChecks
	m.monitor = st
	cb = m.cfg.OnMonitor
	m.monitorMu.Unlock()

	if shouldLog {
		level := slog.LevelInfo
		if !st.Up {
			level = slog.LevelWarn
		}
		m.emitStatus(
			level, "elastic connection check",
			slog.Bool("elastic.up", st.Up),
			slog.String("elastic.endpoint", st.Endpoint),
			slog.String("elastic.last_error", st.LastError),
			slog.Int64("elastic.consecutive_failures", st.ConsecutiveFailures),
			slog.String("elastic.checked_at", st.LastCheckedAt.Format(time.RFC3339Nano)),
		)
	}

	if cb != nil && changed {
		safeCall(m.cfg.RecoverInternally, func() { cb(st) })
	}
}

func monitorChanged(prev, now ConnectionStatus) bool {
	if prev.Up != now.Up {
		return true
	}
	if prev.ConsecutiveFailures != now.ConsecutiveFailures {
		return true
	}
	return strings.TrimSpace(prev.LastError) != strings.TrimSpace(now.LastError)
}
