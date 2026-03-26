package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type contextKey string

const key contextKey = "go-obskit/correlation-id"

func WithID(ctx context.Context, id string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, key, id)
}

func ID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(key).(string)
	return id
}

func GetOrGenerate(ctx context.Context) (context.Context, string) {
	if id := ID(ctx); id != "" {
		return ctx, id
	}
	id := Generate()
	return WithID(ctx, id), id
}

func Generate() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "corr-fallback"
	}
	return hex.EncodeToString(b[:])
}
