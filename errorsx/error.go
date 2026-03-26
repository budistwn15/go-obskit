package errorsx

import "fmt"

type Meta struct {
	Code string
	Type string
	// Layer is free-form. Shared constants are optional conventions only.
	Layer     string
	Component string
	Operation string
	Fields    map[string]any
}

type Error struct {
	err  error
	Meta Meta
}

func (e *Error) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	if e.Meta.Code != "" {
		return fmt.Sprintf("%s: %s", e.Meta.Code, e.err.Error())
	}
	return e.err.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}
