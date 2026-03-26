package gormx

import (
	"errors"

	"gorm.io/gorm"
)

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "record_not_found"
	}
	return "db_error"
}
