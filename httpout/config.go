package httpout

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
	Layer                logger.Layer
	Component            string
	Operation            string
	TargetResolver       func(*http.Request) string
	ShouldLog            func(*http.Request, *http.Response, error, time.Duration) bool
	ShouldLogDetail      func(*http.Request, *http.Response, error, time.Duration) bool
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
		Layer:               logger.LayerContract,
		Component:           "http_client",
		Operation:           "outbound_request",
		RequestHeaders: []string{
			"Content-Type",
			"Accept",
			"X-Request-ID",
			"X-Correlation-ID",
		},
		ResponseHeaders: []string{
			"Content-Type",
			"Content-Length",
		},
	}
}

type runtimeConfig struct {
	logger              *slog.Logger
	correlationHeader   string
	requestHeaders      map[string]struct{}
	responseHeaders     map[string]struct{}
	captureRequestBody  bool
	captureResponseBody bool
	logRequestHeaders   bool
	logResponseHeaders  bool
	logRequestBody      bool
	logResponseBody     bool
	maxBodyCaptureBytes int
	redactor            *logger.Redactor
	redactJSONBody      bool
	layer               logger.Layer
	component           string
	operation           string
	targetResolver      func(*http.Request) string
	shouldLog           func(*http.Request, *http.Response, error, time.Duration) bool
	shouldLogDetail     func(*http.Request, *http.Response, error, time.Duration) bool
}

func normalizeConfig(cfg Config) runtimeConfig {
	def := DefaultConfig()
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.CorrelationHeader == "" {
		cfg.CorrelationHeader = def.CorrelationHeader
	}
	if cfg.RequestHeaders == nil {
		cfg.RequestHeaders = def.RequestHeaders
	}
	if cfg.ResponseHeaders == nil {
		cfg.ResponseHeaders = def.ResponseHeaders
	}
	if cfg.MaxBodyCaptureBytes <= 0 {
		cfg.MaxBodyCaptureBytes = def.MaxBodyCaptureBytes
	}
	if cfg.Redactor == nil {
		cfg.Redactor = def.Redactor
	}
	if cfg.Layer == "" {
		cfg.Layer = def.Layer
	}
	if cfg.Component == "" {
		cfg.Component = def.Component
	}
	if cfg.Operation == "" {
		cfg.Operation = def.Operation
	}
	if cfg.ShouldLog == nil {
		cfg.ShouldLog = func(*http.Request, *http.Response, error, time.Duration) bool { return true }
	}
	if cfg.ShouldLogDetail == nil {
		cfg.ShouldLogDetail = func(*http.Request, *http.Response, error, time.Duration) bool { return false }
	}
	
	return runtimeConfig{
		logger:              cfg.Logger,
		correlationHeader:   cfg.CorrelationHeader,
		requestHeaders:      headerAllowlist(cfg.RequestHeaders),
		responseHeaders:     headerAllowlist(cfg.ResponseHeaders),
		captureRequestBody:  cfg.CaptureRequestBody,
		captureResponseBody: cfg.CaptureResponseBody,
		logRequestHeaders:   cfg.LogRequestHeaders,
		logResponseHeaders:  cfg.LogResponseHeaders,
		logRequestBody:      cfg.LogRequestBody,
		logResponseBody:     cfg.LogResponseBody,
		maxBodyCaptureBytes: cfg.MaxBodyCaptureBytes,
		redactor:            cfg.Redactor,
		redactJSONBody:      resolveRedactJSONBody(cfg),
		layer:               cfg.Layer,
		component:           cfg.Component,
		operation:           cfg.Operation,
		targetResolver:      cfg.TargetResolver,
		shouldLog:           cfg.ShouldLog,
		shouldLogDetail:     cfg.ShouldLogDetail,
	}
}

func resolveRedactJSONBody(cfg Config) bool {
	if cfg.DisableJSONRedaction {
		return false
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
