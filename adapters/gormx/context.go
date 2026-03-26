package gormx

import "context"

type queryMetaKey string

const (
	queryNameKey queryMetaKey = "obskit/gormx/query-name"
	expectedKey  queryMetaKey = "obskit/gormx/expected"
)

func WithQueryName(ctx context.Context, queryName string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if queryName == "" {
		return ctx
	}
	return context.WithValue(ctx, queryNameKey, queryName)
}

func QueryName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(queryNameKey).(string)
	return v
}

func WithExpected(ctx context.Context, expected string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if expected == "" {
		return ctx
	}
	return context.WithValue(ctx, expectedKey, expected)
}

func Expected(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(expectedKey).(string)
	return v
}
