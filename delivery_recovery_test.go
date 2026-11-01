package obskit_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/v2/elastic"
	"github.com/budistwn15/go-obskit/v2/httpsink"
	"github.com/budistwn15/go-obskit/v2/logger"
	"github.com/budistwn15/go-obskit/v2/lokisink"
)

func TestDirectProvidersRecoverAfterTransientFailure(t *testing.T) {
	tests := []struct {
		name string
		new  func(string) recoveryDelivery
	}{
		{"http", newHTTPRecoveryDelivery},
		{"elastic", newElasticRecoveryDelivery},
		{"loki", newLokiRecoveryDelivery},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				_ = r.Body.Close()
				if calls.Add(1) == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				time.Sleep(time.Millisecond)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()

			delivery := tt.new(server.URL)
			log := logger.New(logger.Config{Output: io.Discard, Middlewares: []logger.HandlerMiddleware{delivery.LoggerMiddleware()}})
			log.LogAttrs(context.Background(), slog.LevelInfo, "recover", slog.Int("attempt", 1))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := delivery.Close(ctx); err != nil {
				t.Fatal(err)
			}
			if delivery.sent() != 1 || delivery.retried() == 0 {
				t.Fatalf("sent=%d retried=%d calls=%d", delivery.sent(), delivery.retried(), calls.Load())
			}
		})
	}
}

type recoveryDelivery interface {
	LoggerMiddleware() logger.HandlerMiddleware
	Close(context.Context) error
	sent() uint64
	retried() uint64
}

type httpRecovery struct{ *httpsink.Middleware }
type elasticRecovery struct{ *elastic.Middleware }
type lokiRecovery struct{ *lokisink.Middleware }

func newHTTPRecoveryDelivery(endpoint string) recoveryDelivery {
	return httpRecovery{httpsink.NewMiddleware(httpsink.Config{
		Enabled: true, Endpoint: endpoint, BatchSize: 1, MaxRetries: 2,
		RetryBackoff: time.Millisecond, MaxBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})}
}

func newElasticRecoveryDelivery(endpoint string) recoveryDelivery {
	return elasticRecovery{elastic.NewMiddleware(elastic.Config{
		Enabled: true, Endpoint: endpoint, Index: "logs", BatchSize: 1, MaxRetries: 2,
		RetryBackoff: time.Millisecond, MaxBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})}
}

func newLokiRecoveryDelivery(endpoint string) recoveryDelivery {
	return lokiRecovery{lokisink.NewMiddleware(lokisink.Config{
		Enabled: true, Endpoint: endpoint, BatchSize: 1, MaxRetries: 2,
		RetryBackoff: time.Millisecond, MaxBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})}
}

func (d httpRecovery) sent() uint64       { return d.Stats().Sent }
func (d httpRecovery) retried() uint64    { return d.Stats().Retried }
func (d elasticRecovery) sent() uint64    { return d.Stats().Sent }
func (d elasticRecovery) retried() uint64 { return d.Stats().Retried }
func (d lokiRecovery) sent() uint64       { return d.Stats().Sent }
func (d lokiRecovery) retried() uint64    { return d.Stats().Retried }
