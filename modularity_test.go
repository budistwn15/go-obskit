package obskit_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/budistwn15/go-obskit/correlation"
	"github.com/budistwn15/go-obskit/errorsx"
	"github.com/budistwn15/go-obskit/joblog"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/budistwn15/go-obskit/outbound"
)

func TestCoreModularAdoptionWithoutAdapters(t *testing.T) {
	log := logger.New(
		logger.Config{
			ServiceName: "modular-test",
			Environment: "production",
			Output:      io.Discard,
		},
	)

	ctx := correlation.WithID(context.Background(), "corr-1")
	ctx, run := joblog.Start(
		ctx, log, joblog.Meta{
			JobName:   "job",
			Component: "worker",
			Operation: "run",
		},
	)
	run.AddProcessed(1)
	run.End(nil)

	wrapped := errorsx.Wrap(errors.New("x"), errorsx.Meta{Code: "E_X", Layer: "usecase"})
	if wrapped == nil {
		t.Fatalf("expected wrapped error")
	}

	client := outbound.WrapClient(&http.Client{}, log, outbound.DefaultOptions())
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/health", nil)
	req = req.WithContext(ctx)

	// stub transport at call-site level so test does not perform real network calls
	client.Transport = outbound.NewTransport(
		roundTripFunc(
			func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				}, nil
			},
		), log, outbound.DefaultOptions(),
	)
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected client result err=%v status=%v", err, resp.StatusCode)
	}
	_ = resp.Body.Close()
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
