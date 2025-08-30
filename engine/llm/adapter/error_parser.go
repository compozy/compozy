package llmadapter

import (
	"net/http"
	"strconv"
	"strings"
)

// ErrorParser handles the extraction and classification of errors from various LLM providers
type ErrorParser struct {
	provider string
}

// NewErrorParser creates a new error parser for the given provider
func NewErrorParser(provider string) *ErrorParser {
	return &ErrorParser{
		provider: provider,
	}
}

// ParseError attempts to extract structured error information from raw errors
func (p *ErrorParser) ParseError(err error) *Error {
	if err == nil {
		return nil
	}
	errMsg := err.Error()
	errMsgLower := strings.ToLower(errMsg)
	// Try to extract HTTP status code from error message
	if statusCode := p.extractHTTPStatusCode(errMsgLower); statusCode > 0 {
		return NewError(statusCode, errMsg, p.provider, err)
	}
	// Provider-specific error pattern matching
	if llmErr := p.matchProviderPatterns(errMsgLower, errMsg, err); llmErr != nil {
		return llmErr
	}
	// Network-level error patterns
	if llmErr := p.matchNetworkPatterns(errMsgLower, errMsg, err); llmErr != nil {
		return llmErr
	}
	return nil // Return nil to fallback to generic wrapping
}

// extractHTTPStatusCode attempts to extract HTTP status codes from error messages
func (p *ErrorParser) extractHTTPStatusCode(errMsg string) int {
	// Common HTTP status code patterns in error messages
	patterns := map[string]int{
		"400": http.StatusBadRequest,
		"401": http.StatusUnauthorized,
		"403": http.StatusForbidden,
		"404": http.StatusNotFound,
		"429": http.StatusTooManyRequests,
		"500": http.StatusInternalServerError,
		"502": http.StatusBadGateway,
		"503": http.StatusServiceUnavailable,
		"504": http.StatusGatewayTimeout,
	}
	for pattern, status := range patterns {
		if strings.Contains(errMsg, pattern) {
			return status
		}
	}
	// Try to extract numeric status codes from common patterns
	// e.g., "status code: 429", "HTTP 503", "error 500"
	statusPatterns := []string{
		"status code: ",
		"status code ",
		"http ",
		"error ",
		"code ",
	}
	for _, prefix := range statusPatterns {
		if idx := strings.Index(errMsg, prefix); idx >= 0 {
			start := idx + len(prefix)
			if start < len(errMsg) {
				// Extract next 3 digits
				var numStr strings.Builder
				for i := start; i < len(errMsg) && i < start+3; i++ {
					if errMsg[i] >= '0' && errMsg[i] <= '9' {
						numStr.WriteByte(errMsg[i])
					} else {
						break
					}
				}
				if numStr.Len() == 3 {
					if code, err := strconv.Atoi(numStr.String()); err == nil && code >= 100 && code < 600 {
						return code
					}
				}
			}
		}
	}
	return 0
}

// matchProviderPatterns matches provider-specific error patterns
func (p *ErrorParser) matchProviderPatterns(errMsgLower, errMsg string, originalErr error) *Error {
	// Rate limiting patterns
	rateLimitPatterns := []string{
		"rate limit", "rate-limit", "ratelimit", "too many requests",
		"throttled", "throttling", "quota exceeded", "quota_exceeded",
		"requests per minute", "requests per second", "rpm", "rps",
	}
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(errMsgLower, pattern) {
			return NewError(http.StatusTooManyRequests, errMsg, p.provider, originalErr)
		}
	}
	// Service unavailable patterns
	unavailablePatterns := []string{
		"service unavailable", "service_unavailable", "temporarily unavailable",
		"overloaded", "capacity", "busy", "try again later",
	}
	for _, pattern := range unavailablePatterns {
		if strings.Contains(errMsgLower, pattern) {
			return NewError(http.StatusServiceUnavailable, errMsg, p.provider, originalErr)
		}
	}
	// Authorization patterns
	authPatterns := []string{
		"unauthorized", "invalid api key", "invalid_api_key", "api key",
		"authentication", "auth", "token", "credential",
	}
	for _, pattern := range authPatterns {
		if strings.Contains(errMsgLower, pattern) {
			return NewError(http.StatusUnauthorized, errMsg, p.provider, originalErr)
		}
	}
	// Model/content policy patterns
	if strings.Contains(errMsgLower, "invalid model") || strings.Contains(errMsgLower, "model not found") {
		return NewErrorWithCode(ErrCodeInvalidModel, errMsg, p.provider, originalErr)
	}
	if strings.Contains(errMsgLower, "content policy") || strings.Contains(errMsgLower, "safety") {
		return NewErrorWithCode(ErrCodeContentPolicy, errMsg, p.provider, originalErr)
	}
	// Provider-specific patterns
	providerLower := strings.ToLower(p.provider)
	switch providerLower {
	case "openai":
		return p.matchOpenAIPatterns(errMsgLower, errMsg, originalErr)
	case "anthropic":
		return p.matchAnthropicPatterns(errMsgLower, errMsg, originalErr)
	case "google":
		return p.matchGooglePatterns(errMsgLower, errMsg, originalErr)
	}
	return nil
}

// matchNetworkPatterns matches network-level error patterns
func (p *ErrorParser) matchNetworkPatterns(errMsgLower, errMsg string, originalErr error) *Error {
	// Timeout patterns
	timeoutPatterns := []string{
		"timeout", "timed out", "deadline exceeded", "context deadline exceeded",
	}
	for _, pattern := range timeoutPatterns {
		if strings.Contains(errMsgLower, pattern) {
			return NewErrorWithCode(ErrCodeTimeout, errMsg, p.provider, originalErr)
		}
	}
	// Connection patterns
	connectionPatterns := []string{
		"connection reset", "connection refused", "connection failed",
		"network error", "dns", "host not found",
	}
	for _, pattern := range connectionPatterns {
		if strings.Contains(errMsgLower, pattern) {
			if strings.Contains(errMsgLower, "reset") {
				return NewErrorWithCode(ErrCodeConnectionReset, errMsg, p.provider, originalErr)
			}
			return NewErrorWithCode(ErrCodeConnectionRefused, errMsg, p.provider, originalErr)
		}
	}
	return nil
}

// Provider-specific pattern matching methods
func (p *ErrorParser) matchOpenAIPatterns(errMsgLower, errMsg string, originalErr error) *Error {
	// OpenAI-specific patterns
	if strings.Contains(errMsgLower, "insufficient_quota") {
		return NewErrorWithCode(ErrCodeQuotaExceeded, errMsg, p.provider, originalErr)
	}
	return nil
}

func (p *ErrorParser) matchAnthropicPatterns(errMsgLower, errMsg string, originalErr error) *Error {
	// Anthropic-specific patterns
	if strings.Contains(errMsgLower, "rate_limit_error") {
		return NewError(http.StatusTooManyRequests, errMsg, p.provider, originalErr)
	}
	return nil
}

func (p *ErrorParser) matchGooglePatterns(errMsgLower, errMsg string, originalErr error) *Error {
	// Google AI-specific patterns
	if strings.Contains(errMsgLower, "quota exceeded") {
		return NewErrorWithCode(ErrCodeQuotaExceeded, errMsg, p.provider, originalErr)
	}
	return nil
}
