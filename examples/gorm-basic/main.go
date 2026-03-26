package main

import (
	"github.com/budistwn15/go-obskit/adapters/gormx"
	"github.com/budistwn15/go-obskit/logger"
	"gorm.io/gorm"
)

func main() {
	log := logger.New(
		logger.Config{
			ServiceName: "gorm-basic",
			Environment: "local",
			Level:       logger.LevelInfo,
		},
	)
	
	gormLogger := gormx.New(log, gormx.DefaultOptions())
	
	_ = &gorm.Config{
		Logger: gormLogger,
	}
	
	// Example only:
	// db, err := gorm.Open(driver, &gorm.Config{Logger: gormLogger})
	// _ = db
	// _ = err
}
