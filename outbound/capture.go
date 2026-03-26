package outbound

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/budistwn15/go-obskit/httplog"
)

type bodyCapture struct {
	value     string
	truncated bool
}

func captureRequest(req *http.Request, opts Options, captureBody bool) (httplog.RequestMeta, bodyCapture) {
	meta := httplog.RequestMeta{
		Method:    req.Method,
		Scheme:    req.URL.Scheme,
		Host:      req.URL.Host,
		Path:      req.URL.Path,
		URL:       req.URL.String(),
		UserAgent: req.Header.Get("User-Agent"),
	}
	httplog.FillTargetFromURL(&meta, req.URL)
	if opts.CaptureQuery {
		meta.Query = httplog.NormalizeQuery(req.URL.Query(), opts.BodyJSONDenylist)
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHTTPHeaders(req.Header, opts.HeaderAllowlist, opts.HeaderDenylist)
	}

	captured := bodyCapture{}
	if captureBody && req.Body != nil {
		contentType := req.Header.Get("Content-Type")
		if httplog.IsSafeBodyContentType(contentType) {
			readLimit := int64(opts.MaxBodyBytes + 1)
			buf, err := io.ReadAll(io.LimitReader(req.Body, readLimit))
			if err == nil {
				req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), req.Body))
				body := httplog.CaptureBody(contentType, buf, opts.MaxBodyBytes, opts.BodyJSONDenylist)
				captured.value = body.Value
				captured.truncated = body.Truncated
				meta.RequestBody = body.Value
				meta.RequestBodyTruncated = body.Truncated
			}
		}
	}
	return httplog.NormalizeRequestMeta(meta), captured
}

func captureResponse(resp *http.Response, opts Options, captureBody bool) (httplog.ResponseMeta, bodyCapture) {
	meta := httplog.ResponseMeta{}
	captured := bodyCapture{}
	if resp == nil {
		return httplog.NormalizeResponseMeta(meta), captured
	}

	meta.StatusCode = resp.StatusCode
	meta.SizeBytes = resp.ContentLength
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHTTPHeaders(resp.Header, opts.HeaderAllowlist, opts.HeaderDenylist)
	}

	if captureBody && resp.Body != nil {
		contentType := resp.Header.Get("Content-Type")
		if httplog.IsSafeBodyContentType(contentType) {
			readLimit := int64(opts.MaxBodyBytes + 1)
			buf, err := io.ReadAll(io.LimitReader(resp.Body, readLimit))
			if err == nil {
				resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), resp.Body))
				body := httplog.CaptureBody(contentType, buf, opts.MaxBodyBytes, opts.BodyJSONDenylist)
				captured.value = body.Value
				captured.truncated = body.Truncated
				meta.ResponseBody = body.Value
				meta.ResponseBodyTruncated = body.Truncated
			}
		}
	}
	return httplog.NormalizeResponseMeta(meta), captured
}

func traceIDs(req *http.Request) (traceID, spanID string) {
	traceID = strings.TrimSpace(req.Header.Get("X-Trace-ID"))
	spanID = strings.TrimSpace(req.Header.Get("X-Span-ID"))
	return traceID, spanID
}
