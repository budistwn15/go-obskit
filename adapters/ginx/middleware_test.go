package ginx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/budistwn15/go-obskit/logger"
	"github.com/gin-gonic/gin"
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

func TestMiddleware_GinLifecycleRouteAndCorrelation(t *testing.T) {
	gin.SetMode(gin.TestMode)
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
	
	r := gin.New()
	r.Use(Middleware(log, opts))
	r.GET(
		"/users/:id", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)
	
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	req.Header.Set("X-Correlation-ID", "corr-gin")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	
	if rec.Code != http.StatusOK {
		t.Fatalf("request flow broken")
	}
	if rec.Header().Get("X-Correlation-ID") != "corr-gin" {
		t.Fatalf("correlation header not propagated")
	}
	
	logs := parseLines(t, buf.String())
	if len(logs) == 0 {
		t.Fatalf("expected logs")
	}
	last := logs[len(logs)-1]
	if last["http.route"] != "/users/:id" {
		t.Fatalf("expected route capture, got=%v", last["http.route"])
	}
}
