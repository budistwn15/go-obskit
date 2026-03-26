package main

import (
	"net/http"
	"time"

	"github.com/budistwn15/go-obskit/logger"
	"github.com/budistwn15/go-obskit/outbound"
)

func main() {
	log := logger.New(
		logger.Config{
			ServiceName: "outbound-basic",
			Environment: "local",
			Level:       logger.LevelInfo,
		},
	)
	
	client := &http.Client{Timeout: 5 * time.Second}
	client = outbound.WrapClient(client, log, outbound.DefaultOptions())
	
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = client.Do(req)
}
