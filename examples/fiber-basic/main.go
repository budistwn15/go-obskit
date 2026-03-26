package main

import (
	"github.com/budistwn15/go-obskit/adapters/fiberx"
	"github.com/budistwn15/go-obskit/logger"
	"github.com/gofiber/fiber/v2"
)

func main() {
	log := logger.New(
		logger.Config{
			ServiceName: "fiber-basic",
			Environment: "local",
			Level:       logger.LevelInfo,
		},
	)
	
	app := fiber.New()
	app.Use(fiberx.Middleware(log, fiberx.DefaultOptions()))
	app.Get(
		"/health", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{"ok": true})
		},
	)
	
	_ = app.Listen(":8082")
}
