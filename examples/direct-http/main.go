package main

import (
	"context"
	"os"

	obskit "github.com/budistwn15/go-obskit/v2"
	"github.com/budistwn15/go-obskit/v2/httpsink"
)

func main() {
	sink := httpsink.NewMiddleware(httpsink.Config{
		Enabled:  true,
		Endpoint: "https://collector.example.com/logs",
		APIKey:   os.Getenv("LOG_API_KEY"),
	})
	runtime := obskit.Open(obskit.Config{Service: "billing-api"}, sink)
	defer runtime.Close(context.Background())
	runtime.Logger().Info("ready")
}
