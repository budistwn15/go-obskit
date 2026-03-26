package fiberx

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
	"github.com/gofiber/fiber/v2"
)

func parseLines(t *testing.T, raw string) []map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid line: %v", err)
		}
		out = append(out, m)
	}
	return out
}

func TestMiddleware_FiberLifecyclePathAndCorrelation(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(
		logger.Config{
			ServiceName: "svc",
			Environment: "production",
			Level:       logger.LevelInfo,
			Output:      &buf,
		},
	)
	opts := DefaultOptions()
	opts.CorrelationHeader = "X-Correlation-ID"
	
	app := fiber.New()
	app.Use(Middleware(log, opts))
	app.Get(
		"/orders/:id", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"ok": true})
		},
	)
	
	req := httptest.NewRequest("GET", "/orders/42", nil)
	req.Header.Set("X-Correlation-ID", "corr-fiber")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("fiber request failed: %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("request flow broken")
	}
	if resp.Header.Get("X-Correlation-ID") != "corr-fiber" {
		t.Fatalf("correlation header not propagated")
	}
	
	logs := parseLines(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("expected logs")
	}
	last := logs[len(logs)-1]
	if last["http.route"] == "" {
		t.Fatalf("expected route/path capture")
	}
}
