package httpapi

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/gin-gonic/gin"
)

const (
	ipv4WildcardBind = "0.0.0.0"
)

var errLoopbackMutationRequired = errors.New(
	"remote HTTP settings and extension mutations are disabled in v1 unless the daemon is bound to a loopback host",
)

var errLoopbackAPIRequired = errors.New(
	"remote HTTP API access is disabled unless the daemon is bound to a loopback host",
)

var errRequestBodyTooLarge = core.ErrRequestBodyTooLarge

const maxAPIRequestBodyBytes int64 = 4 << 20

func requestLoggingMiddleware(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		logger.Info(
			"httpapi: request",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", c.Writer.Status(),
			"latency_ms", time.Since(started).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}

func corsMiddleware(boundHost string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(c.GetHeader("Origin"))
		headers := c.Writer.Header()
		headers.Set("Access-Control-Allow-Headers", "Content-Type, Last-Event-ID, Accept")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		headers.Set("Access-Control-Expose-Headers", "Content-Type, Last-Event-ID, x-vercel-ai-ui-message-stream")
		headers.Set("Vary", "Origin")
		if origin != "" {
			allowedOrigin, ok := resolveAllowedOrigin(origin, requestScheme(c.Request), c.Request.Host, boundHost)
			if !ok {
				if isOpenAICompatiblePath(c) {
					core.RespondOpenAIError(c, http.StatusForbidden, errors.New("origin not allowed"), false)
					c.Abort()
				} else {
					c.AbortWithStatusJSON(http.StatusForbidden, contract.ErrorPayload{Error: "origin not allowed"})
				}
				return
			}
			headers.Set("Access-Control-Allow-Origin", allowedOrigin)
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func resolveAllowedOrigin(origin string, requestScheme string, requestHost string, boundHost string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", false
	}

	originSpec, ok := canonicalOriginFromURL(parsed)
	if !ok {
		return "", false
	}

	requestSpec, ok := canonicalOriginFromHost(requestHost, requestScheme, "")
	if !ok {
		return "", false
	}

	boundSpec, ok := canonicalOriginFromHost(boundHost, requestSpec.scheme, requestSpec.port)
	switch {
	case originSpec.canonical == requestSpec.canonical:
		return origin, true
	case ok && !boundSpec.wildcard && originSpec.canonical == boundSpec.canonical:
		return origin, true
	default:
		return "", false
	}
}

type canonicalOrigin struct {
	scheme    string
	hostname  string
	port      string
	canonical string
	loopback  bool
	wildcard  bool
}

const defaultRequestScheme = "http"

func requestScheme(r *http.Request) string {
	if r == nil {
		return defaultRequestScheme
	}
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		return strings.ToLower(forwarded)
	}
	if r.TLS != nil {
		return "https"
	}
	if scheme := strings.TrimSpace(r.URL.Scheme); scheme != "" {
		return strings.ToLower(scheme)
	}
	return defaultRequestScheme
}

func canonicalOriginFromURL(parsed *url.URL) (canonicalOrigin, bool) {
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	hostname := canonicalHost(parsed.Hostname())
	port := normalizePort(scheme, parsed.Port())
	if scheme == "" || hostname == "" || port == "" {
		return canonicalOrigin{}, false
	}

	return canonicalOrigin{
		scheme:    scheme,
		hostname:  hostname,
		port:      port,
		canonical: scheme + "://" + net.JoinHostPort(hostname, port),
		loopback:  isLoopbackHost(hostname),
		wildcard:  isWildcardHost(hostname),
	}, true
}

func canonicalOriginFromHost(host string, scheme string, fallbackPort string) (canonicalOrigin, bool) {
	trimmedHost := strings.TrimSpace(host)
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if trimmedHost == "" || scheme == "" {
		return canonicalOrigin{}, false
	}

	hostname := canonicalHost(trimmedHost)
	port := ""
	if parsedHost, parsedPort, err := net.SplitHostPort(trimmedHost); err == nil {
		hostname = canonicalHost(parsedHost)
		port = parsedPort
	}
	if hostname == "" {
		return canonicalOrigin{}, false
	}

	port = normalizePort(scheme, firstNonEmptyString(port, fallbackPort))
	if port == "" {
		return canonicalOrigin{}, false
	}

	return canonicalOrigin{
		scheme:    scheme,
		hostname:  hostname,
		port:      port,
		canonical: scheme + "://" + net.JoinHostPort(hostname, port),
		loopback:  isLoopbackHost(hostname),
		wildcard:  isWildcardHost(hostname),
	}, true
}

func normalizePort(scheme string, port string) string {
	trimmed := strings.TrimSpace(port)
	if trimmed != "" {
		return trimmed
	}

	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func canonicalHost(value string) string {
	host := strings.TrimSpace(value)
	if host == "" {
		return ""
	}
	if strings.Contains(host, "://") {
		if parsed, err := url.Parse(host); err == nil {
			host = parsed.Hostname()
		}
	} else if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return strings.Trim(strings.TrimSpace(host), "[]")
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isWildcardHost(host string) bool {
	switch host {
	case "", ipv4WildcardBind, "::":
		return true
	default:
		return false
	}
}

func errorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 || c.Writer.Written() {
			return
		}
		core.RespondError(c, http.StatusInternalServerError, c.Errors.Last(), true)
	}
}

func requestBodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 || c.Request == nil || c.Request.Body == nil || c.Request.Body == http.NoBody {
			c.Next()
			return
		}
		if !strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			core.RespondError(c, http.StatusRequestEntityTooLarge, errRequestBodyTooLarge, false)
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

func loopbackAPIGuard(boundHost string) gin.HandlerFunc {
	return loopbackGuard(boundHost, errLoopbackAPIRequired, true)
}

func loopbackMutationGuard(boundHost string) gin.HandlerFunc {
	return loopbackGuard(boundHost, errLoopbackMutationRequired, false)
}

func loopbackGuard(boundHost string, guardErr error, openAICompatible bool) gin.HandlerFunc {
	allowed := isLoopbackHost(canonicalHost(boundHost))
	return func(c *gin.Context) {
		if allowed {
			c.Next()
			return
		}
		if openAICompatible && isOpenAICompatiblePath(c) {
			core.RespondOpenAIError(c, http.StatusForbidden, guardErr, false)
		} else {
			core.RespondError(c, http.StatusForbidden, guardErr, false)
		}
		c.Abort()
	}
}

func isOpenAICompatiblePath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	return strings.HasPrefix(c.Request.URL.Path, "/api/openai/")
}
