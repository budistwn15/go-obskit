package main

import (
	"log/slog"

	obskit "github.com/budistwn15/go-obskit"
)

func main() {
	log := obskit.New(obskit.Config{Service: "billing-api", Environment: "production"})
	log.Info("ready", slog.String("region", "ap-southeast-1"))
}
