package main

import (
	"net/http"

	"github.com/budistwn15/go-obskit/adapters/ginx"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/gin-gonic/gin"
)

func main() {
	log := logger.New(
		logger.Config{
			ServiceName: "gin-basic",
			Environment: "local",
			Level:       logger.LevelInfo,
		},
	)
	
	r := gin.New()
	r.Use(ginx.Middleware(log, ginx.DefaultOptions()))
	r.GET(
		"/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		},
	)
	
	_ = r.Run(":8081")
}
