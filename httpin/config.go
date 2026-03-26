package httpin

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/budistwn15/go-obskit/logger"
)

const DefaultCorrelationHeader = "X-Correlation-ID"

type Config struct {
	Logger               *slog.Logger
	CorrelationHeader    string
	RequestHeaders       []string
	ResponseHeaders      []string
	CaptureRequestBody   bool
	CaptureResponseBody  bool
	LogRequestHeaders    bool
	LogResponseHeaders   bool
	LogRequestBody       bool
	LogResponseBody      bool
	MaxBodyCaptureBytes  int
	Redactor             *logger.Redactor
	RedactJSONBody       bool
	DisableJSONRedaction bool
	RouteExtractor       func(*http.Request) string
	ClientIPExtractor    func(*http.Request) string
	CorrelationIDFactory func() string
	ShouldLog            func(*http.Request, int, time.Duration) bool
	ShouldLogDetail      func(*http.Request, int, time.Duration) bool
}

func DefaultConfig() Config {
	return Config{
		CorrelationHeader:   DefaultCorrelationHeader,
		CaptureRequestBody:  false,
		CaptureResponseBody: false,
		LogRequestHeaders:   false,
		LogResponseHeaders:  false,
		LogRequestBody:      false,
		LogResponseBody:     false,
		MaxBodyCaptureBytes: 4 * 1024,
		Redactor:            logger.DefaultRedactor(),
		RedactJSONBody:      true,
		RequestHeaders: []string{
			"Content-Type",
			"Accept",
			"X-Request-ID",
			"X-Forwarded-For",
			"X-Real-IP",
		},
		ResponseHeaders: []string{
			"Content-Type",
			"Content-Length",
		},
	}
}

type runtimeConfig struct {
	logger               *slog.Logger
	correlationHeader    string
	requestHeaders       map[string]struct{}
	responseHeaders      map[string]struct{}
	captureRequestBody   bool
	captureResponseBody  bool
	logRequestHeaders    bool
	logResponseHeaders   bool
	logRequestBody       bool
	logResponseBody      bool
	maxBodyCaptureBytes  int
	redactor             *logger.Redactor
	redactJSONBody       bool
	routeExtractor       func(*http.Request) string
	clientIPExtractor    func(*http.Request) string
	correlationIDFactory func() string
	shouldLog            func(*http.Request, int, time.Duration) bool
	shouldLogDetail      func(*http.Request, int, time.Duration) bool
}

func normalizeConfig(cfg Config) runtimeConfig {
	def := DefaultConfig()
	
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.CorrelationHeader == "" {
		cfg.CorrelationHeader = def.CorrelationHeader
	}
	if cfg.MaxBodyCaptureBytes <= 0 {
		cfg.MaxBodyCaptureBytes = def.MaxBodyCaptureBytes
	}
	if cfg.Redactor == nil {
		cfg.Redactor = def.Redactor
	}
	if cfg.RequestHeaders == nil {
		cfg.RequestHeaders = def.RequestHeaders
	}
	if cfg.ResponseHeaders == nil {
		cfg.ResponseHeaders = def.ResponseHeaders
	}
	if cfg.CorrelationIDFactory == nil {
		cfg.CorrelationIDFactory = newCorrelationID
	}
	if cfg.ShouldLog == nil {
		cfg.ShouldLog = func(*http.Request, int, time.Duration) bool { return true }
	}
	if cfg.ShouldLogDetail == nil {
		cfg.ShouldLogDetail = func(*http.Request, int, time.Duration) bool { return false }
	}
	
	return runtimeConfig{
		logger:               cfg.Logger,
		correlationHeader:    cfg.CorrelationHeader,
		requestHeaders:       headerAllowlist(cfg.RequestHeaders),
		responseHeaders:      headerAllowlist(cfg.ResponseHeaders),
		captureRequestBody:   cfg.CaptureRequestBody,
		captureResponseBody:  cfg.CaptureResponseBody,
		logRequestHeaders:    cfg.LogRequestHeaders,
		logResponseHeaders:   cfg.LogResponseHeaders,
		logRequestBody:       cfg.LogRequestBody,
		logResponseBody:      cfg.LogResponseBody,
		maxBodyCaptureBytes:  cfg.MaxBodyCaptureBytes,
		redactor:             cfg.Redactor,
		redactJSONBody:       resolveRedactJSONBody(cfg),
		routeExtractor:       cfg.RouteExtractor,
		clientIPExtractor:    cfg.ClientIPExtractor,
		correlationIDFactory: cfg.CorrelationIDFactory,
		shouldLog:            cfg.ShouldLog,
		shouldLogDetail:      cfg.ShouldLogDetail,
	}
}

func resolveRedactJSONBody(cfg Config) bool {
	if cfg.DisableJSONRedaction {
		return false
	}
	if cfg.RedactJSONBody {
		return true
	}
	return true
}

func headerAllowlist(headers []string) map[string]struct{} {
	out := make(map[string]struct{}, len(headers))
	for _, h := range headers {
		if k := http.CanonicalHeaderKey(h); k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}
