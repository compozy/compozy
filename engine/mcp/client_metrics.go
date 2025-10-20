package mcp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"

	mcpmetrics "github.com/compozy/compozy/engine/mcp/metrics"
)

func categorizeToolError(err error) mcpmetrics.ErrorKind {
	if err == nil {
		// No error should be classified; let callers skip recording.
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return mcpmetrics.ErrorKindTimeout
	}
	if kind := proxyErrorKind(err); kind != "" {
		return kind
	}
	if kind := networkErrorKind(err); kind != "" {
		return kind
	}
	if kind := inferredErrorKind(err); kind != "" {
		return kind
	}
	return mcpmetrics.ErrorKindExecution
}

func proxyErrorKind(err error) mcpmetrics.ErrorKind {
	var proxyErr *ProxyRequestError
	if !errors.As(err, &proxyErr) {
		return ""
	}
	switch proxyErr.StatusCode {
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return mcpmetrics.ErrorKindTimeout
	case http.StatusNotFound:
		return mcpmetrics.ErrorKindConnection
	}
	if proxyErr.StatusCode >= 500 {
		return mcpmetrics.ErrorKindExecution
	}
	if proxyErr.StatusCode >= 400 {
		return mcpmetrics.ErrorKindValidation
	}
	return ""
}

func networkErrorKind(err error) mcpmetrics.ErrorKind {
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return mcpmetrics.ErrorKindTimeout
		}
		return mcpmetrics.ErrorKindConnection
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return ""
	}
	if nested, ok := urlErr.Err.(net.Error); ok {
		if nested.Timeout() {
			return mcpmetrics.ErrorKindTimeout
		}
		return mcpmetrics.ErrorKindConnection
	}
	if containsConnectionPhrase(strings.ToLower(urlErr.Err.Error())) {
		return mcpmetrics.ErrorKindConnection
	}
	return ""
}

func inferredErrorKind(err error) mcpmetrics.ErrorKind {
	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "timeout"):
		return mcpmetrics.ErrorKindTimeout
	case containsConnectionPhrase(lower):
		return mcpmetrics.ErrorKindConnection
	case strings.Contains(lower, "invalid"),
		strings.Contains(lower, "validation"),
		strings.Contains(lower, "bad request"):
		return mcpmetrics.ErrorKindValidation
	default:
		return ""
	}
}

var connectionErrorPhrases = []string{
	"connection refused",
	"connection reset",
	"broken pipe",
	"network unreachable",
	"no route to host",
	"no such host",
	"tls:",
	"eof",
}

func containsConnectionPhrase(msg string) bool {
	for _, phrase := range connectionErrorPhrases {
		if strings.Contains(msg, phrase) {
			return true
		}
	}
	return false
}
