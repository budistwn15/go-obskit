package main

import (
	"net/http"

	"github.com/budistwn15/go-obskit/adapters/nethttp"
	"github.com/budistwn15/go-obskit/logger"
)

func main() {
	log := logger.New(
		logger.Config{
			ServiceName: "nethttp-basic",
			Environment: "local",
			Level:       logger.LevelInfo,
		},
	)
	
	opts := nethttp.DefaultOptions()
	mux := http.NewServeMux()
	mux.HandleFunc(
		"/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		},
	)
	
	_ = http.ListenAndServe(":8080", nethttp.Middleware(log, opts)(mux))
}
