package obskit_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/budistwn15/go-obskit/v2/elastic"
	"github.com/budistwn15/go-obskit/v2/httpsink"
	"github.com/budistwn15/go-obskit/v2/logger"
	"github.com/budistwn15/go-obskit/v2/lokisink"
)

type deliveryFailureHarness struct {
	middleware logger.HandlerMiddleware
	close      func(context.Context) error
	stats      func() (failed, retried uint64)
}

func TestDirectProvidersIsolatePermanentAuthFailure(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"api_key":"must-not-leak"}`))
			}))
			defer server.Close()
			for _, provider := range failureProviders(server.URL, nil) {
				assertDeliveryFailure(t, provider.name, provider.harness, 0)
			}
		})
	}
}

func TestDirectProvidersIsolateTransportFailures(t *testing.T) {
	errorsToTest := []struct {
		name string
		err  error
	}{
		{"dns", &net.DNSError{Err: "no such host", Name: "invalid.example"}},
		{"refused", errors.New("connection refused")},
		{"tls", errors.New("tls handshake failure")},
		{"timeout", context.DeadlineExceeded},
	}
	for _, failure := range errorsToTest {
		t.Run(failure.name, func(t *testing.T) {
			client := &http.Client{Transport: failureRoundTripper{err: failure.err}}
			for _, provider := range failureProviders("https://invalid.example", client) {
				assertDeliveryFailure(t, provider.name, provider.harness, 1)
			}
		})
	}
}

func TestDirectProvidersIsolateMalformedEndpoint(t *testing.T) {
	for _, provider := range failureProviders("://bad endpoint", nil) {
		assertDeliveryFailure(t, provider.name, provider.harness, 1)
	}
}

func assertDeliveryFailure(t *testing.T, name string, harness deliveryFailureHarness, wantRetries uint64) {
	t.Helper()
	var stdout bytes.Buffer
	log := logger.New(logger.Config{Output: &stdout, Middlewares: []logger.HandlerMiddleware{harness.middleware}})
	log.LogAttrs(context.Background(), slog.LevelInfo, "application continues", slog.String("safe", "value"))
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		failed, _ := harness.stats()
		if failed == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := harness.close(ctx); err != nil {
		t.Fatalf("%s close: %v", name, err)
	}
	failed, retried := harness.stats()
	if failed != 1 || retried != wantRetries {
		t.Fatalf("%s failed=%d retried=%d", name, failed, retried)
	}
	if !strings.Contains(stdout.String(), "application continues") {
		t.Fatalf("%s changed application logging", name)
	}
}

type namedFailureProvider struct {
	name    string
	harness deliveryFailureHarness
}

func failureProviders(endpoint string, client *http.Client) []namedFailureProvider {
	httpSink := httpsink.NewMiddleware(httpsink.Config{
		Enabled: true, Endpoint: endpoint, HTTPClient: client, BatchSize: 1, MaxRetries: 1,
		RetryBackoff: time.Millisecond, MaxBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})
	elasticSink := elastic.NewMiddleware(elastic.Config{
		Enabled: true, Endpoint: endpoint, Index: "logs", HTTPClient: client, BatchSize: 1, MaxRetries: 1,
		RetryBackoff: time.Millisecond, MaxBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})
	lokiSink := lokisink.NewMiddleware(lokisink.Config{
		Enabled: true, Endpoint: endpoint, HTTPClient: client, BatchSize: 1, MaxRetries: 1,
		RetryBackoff: time.Millisecond, MaxBackoff: time.Millisecond, ConnectionLogToStdout: false,
	})
	return []namedFailureProvider{
		{"http", deliveryFailureHarness{httpSink.LoggerMiddleware(), httpSink.Close, func() (uint64, uint64) {
			s := httpSink.Stats()
			return s.Failed, s.Retried
		}}},
		{"elastic", deliveryFailureHarness{elasticSink.LoggerMiddleware(), elasticSink.Close, func() (uint64, uint64) {
			s := elasticSink.Stats()
			return s.Failed, s.Retried
		}}},
		{"loki", deliveryFailureHarness{lokiSink.LoggerMiddleware(), lokiSink.Close, func() (uint64, uint64) {
			s := lokiSink.Stats()
			return s.Failed, s.Retried
		}}},
	}
}

type failureRoundTripper struct{ err error }

func (f failureRoundTripper) RoundTrip(*http.Request) (*http.Response, error) { return nil, f.err }
