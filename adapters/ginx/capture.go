package ginx

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"github.com/budistwn15/go-obskit/httplog"
	"github.com/gin-gonic/gin"
)

type responseWriter struct {
	gin.ResponseWriter
	captureBody   bool
	maxBodyBytes  int
	body          bytes.Buffer
	bodyTruncated bool
}

func newResponseWriter(w gin.ResponseWriter, capture bool, maxBytes int) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		captureBody:    capture,
		maxBodyBytes:   maxBytes,
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.captureBody {
		w.capture(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *responseWriter) capture(p []byte) {
	if w.maxBodyBytes <= 0 || w.bodyTruncated {
		return
	}
	remain := w.maxBodyBytes - w.body.Len()
	if remain <= 0 {
		w.bodyTruncated = true
		return
	}
	if len(p) > remain {
		_, _ = w.body.Write(p[:remain])
		w.bodyTruncated = true
		return
	}
	_, _ = w.body.Write(p)
}

func captureRequestBody(c *gin.Context, opts Options) (string, bool) {
	if !opts.CaptureRequestBody || c.Request == nil || c.Request.Body == nil {
		return "", false
	}
	contentType := c.GetHeader("Content-Type")
	if !httplog.IsSafeBodyContentType(contentType) {
		return "", false
	}
	max := opts.MaxBodyBytes
	buf, err := io.ReadAll(io.LimitReader(c.Request.Body, int64(max+1)))
	if err != nil {
		return "", false
	}
	c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), c.Request.Body))
	cap := httplog.CaptureBody(contentType, buf, max, opts.BodyJSONDenylist)
	return cap.Value, cap.Truncated
}

func captureRequestMeta(c *gin.Context, opts Options) httplog.RequestMeta {
	meta := httplog.RequestMeta{
		Method: c.Request.Method,
		Scheme: requestScheme(c.Request),
		Host:   c.Request.Host,
		Path:   c.Request.URL.Path,
		Route:  c.FullPath(),
		URL:    c.Request.URL.String(),
	}
	httplog.FillSourceFromRemoteAddr(&meta, c.Request.RemoteAddr)
	httplog.FillTargetFromRequest(&meta, c.Request)
	if opts.CaptureQuery {
		meta.Query = httplog.NormalizeQuery(c.Request.URL.Query(), opts.BodyJSONDenylist)
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHTTPHeaders(c.Request.Header, opts.HeaderAllowlist, opts.HeaderDenylist)
	}
	if opts.IncludeUserAgent {
		meta.UserAgent = c.Request.UserAgent()
	}
	if opts.IncludeReferer {
		meta.Referer = c.Request.Referer()
	}
	if opts.IncludeClientIP {
		meta.ClientIP = c.ClientIP()
		meta.XForwardedFor = c.GetHeader("X-Forwarded-For")
		meta.XRealIP = c.GetHeader("X-Real-IP")
	}
	return httplog.NormalizeRequestMeta(meta)
}

func captureResponseMeta(c *gin.Context, w *responseWriter, opts Options) httplog.ResponseMeta {
	meta := httplog.ResponseMeta{
		StatusCode: w.Status(),
		SizeBytes:  int64(w.Size()),
	}
	if opts.CaptureHeaders {
		meta.Headers = httplog.FilterHTTPHeaders(c.Writer.Header(), opts.HeaderAllowlist, opts.HeaderDenylist)
	}
	shouldLogRespBody := (meta.StatusCode >= 400 && opts.LogErrorBodies) || (meta.StatusCode < 400 && opts.LogSuccessBodies)
	if opts.CaptureResponseBody && shouldLogRespBody {
		contentType := c.Writer.Header().Get("Content-Type")
		if httplog.IsSafeBodyContentType(contentType) {
			bodyCap := httplog.CaptureBody(contentType, w.body.Bytes(), opts.MaxBodyBytes, opts.BodyJSONDenylist)
			meta.ResponseBody = bodyCap.Value
			meta.ResponseBodyTruncated = bodyCap.Truncated || w.bodyTruncated
		}
	}
	return httplog.NormalizeResponseMeta(meta)
}

func requestScheme(req *http.Request) string {
	if req == nil {
		return ""
	}
	if xf := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); xf != "" {
		return xf
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}
