package fiberx

import (
	"github.com/budistwn15/go-obskit/httplog"
	"github.com/gofiber/fiber/v2"
)

func captureRequestMeta(c *fiber.Ctx, opts Options) httplog.RequestMeta {
	meta := httplog.RequestMeta{
		Method: c.Method(),
		Scheme: c.Protocol(),
		Host:   c.Hostname(),
		Path:   c.Path(),
		Route:  routePath(c),
		URL:    c.OriginalURL(),
	}
	if ra := c.Context().RemoteAddr(); ra != nil {
		httplog.FillSourceFromRemoteAddr(&meta, ra.String())
	}
	if opts.CaptureQuery {
		query := make(map[string][]string, len(c.Queries()))
		for k, v := range c.Queries() {
			query[k] = []string{v}
		}
		meta.Query = httplog.NormalizeQueryValues(query, opts.BodyJSONDenylist)
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHeaders(requestHeaders(c), opts.HeaderAllowlist, opts.HeaderDenylist)
	}
	if opts.IncludeUserAgent {
		meta.UserAgent = c.Get("User-Agent")
	}
	if opts.IncludeReferer {
		meta.Referer = c.Get("Referer")
	}
	if opts.IncludeClientIP {
		meta.ClientIP = c.IP()
		meta.XForwardedFor = c.Get("X-Forwarded-For")
		meta.XRealIP = c.Get("X-Real-IP")
	}
	return httplog.NormalizeRequestMeta(meta)
}

func captureResponseMeta(c *fiber.Ctx, opts Options) httplog.ResponseMeta {
	meta := httplog.ResponseMeta{
		StatusCode: c.Response().StatusCode(),
		SizeBytes:  int64(len(c.Response().Body())),
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHeaders(responseHeaders(c), opts.HeaderAllowlist, opts.HeaderDenylist)
	}
	shouldLogRespBody := (meta.StatusCode >= 400 && opts.LogErrorBodies) || (meta.StatusCode < 400 && opts.LogSuccessBodies)
	if opts.CaptureResponseBody && shouldLogRespBody {
		contentType := c.GetRespHeader("Content-Type")
		bodyCap := httplog.CaptureBody(contentType, c.Response().Body(), opts.MaxBodyBytes, opts.BodyJSONDenylist)
		meta.ResponseBody = bodyCap.Value
		meta.ResponseBodyTruncated = bodyCap.Truncated
	}
	return httplog.NormalizeResponseMeta(meta)
}

func captureRequestBody(c *fiber.Ctx, opts Options) (string, bool) {
	if !opts.CaptureRequestBody {
		return "", false
	}
	contentType := c.Get("Content-Type")
	bodyCap := httplog.CaptureBody(contentType, c.Body(), opts.MaxBodyBytes, opts.BodyJSONDenylist)
	return bodyCap.Value, bodyCap.Truncated
}

func routePath(c *fiber.Ctx) string {
	route := c.Route()
	if route == nil {
		return ""
	}
	return route.Path
}

func requestHeaders(c *fiber.Ctx) map[string][]string {
	out := make(map[string][]string)
	c.Request().Header.VisitAll(
		func(key []byte, value []byte) {
			k := string(key)
			out[k] = append(out[k], string(value))
		},
	)
	return out
}

func responseHeaders(c *fiber.Ctx) map[string][]string {
	out := make(map[string][]string)
	c.Response().Header.VisitAll(
		func(key []byte, value []byte) {
			k := string(key)
			out[k] = append(out[k], string(value))
		},
	)
	return out
}
