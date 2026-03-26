package httpin

import "context"

type contextKey string

const routePatternKey contextKey = "go-obskit/httpin/route"

func WithRoute(ctx context.Context, route string) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithValue(ctx, routePatternKey, route)
}

func RouteFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	route, ok := ctx.Value(routePatternKey).(string)
	return route, ok
}
