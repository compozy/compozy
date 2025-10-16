package llmadapter

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrorParser handles the extraction and classification of errors from various LLM providers
type ErrorParser struct {
	provider string
}

var retryAfterMessagePattern = regexp.MustCompile(
	`(?i)(?:retry(?:\s|-)?after|try(?:\s|-)?again(?:\s|-)?in)[^0-9]*(\d+(?:\.\d+)?)(?:\s*(milliseconds?|ms|seconds?|secs?|s|minutes?|mins?|m|hours?|hrs?|h))?`,
)

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
	retryHint := p.extractRetryAfter(err)
	// Try to extract HTTP status code from error message
	if statusCode := p.extractHTTPStatusCode(errMsgLower); statusCode > 0 {
		llmErr := NewError(statusCode, errMsg, p.provider, err)
		if retryHint != nil {
			llmErr.RetryAfter = retryHint
		} else if retry := parseRetryAfterFromMessage(errMsg); retry != nil {
			llmErr.RetryAfter = retry
		}
		return llmErr
	}
	// Provider-specific error pattern matching
	if llmErr := p.matchProviderPatterns(errMsgLower, errMsg, err); llmErr != nil {
		if llmErr.RetryAfter == nil {
			if retryHint != nil {
				llmErr.RetryAfter = retryHint
			} else {
				llmErr.RetryAfter = parseRetryAfterFromMessage(errMsg)
			}
		}
		return llmErr
	}
	// Network-level error patterns
	if llmErr := p.matchNetworkPatterns(errMsgLower, errMsg, err); llmErr != nil {
		if llmErr.RetryAfter == nil {
			if retryHint != nil {
				llmErr.RetryAfter = retryHint
			} else {
				llmErr.RetryAfter = parseRetryAfterFromMessage(errMsg)
			}
		}
		return llmErr
	}
	return nil // Return nil to fallback to generic wrapping
}

// ParseErrorWithHeaders attempts to parse structured error information plus retry hints from headers.
func (p *ErrorParser) ParseErrorWithHeaders(err error, headers map[string]string) *Error {
	llmErr := p.ParseError(err)
	if llmErr == nil {
		return nil
	}
	if llmErr.RetryAfter == nil {
		if retry := p.parseRetryAfter(headers); retry != nil {
			llmErr.RetryAfter = retry
		}
	}
	return llmErr
}

// extractHTTPStatusCode attempts to extract HTTP status codes from error messages
func (p *ErrorParser) extractHTTPStatusCode(errMsg string) int {
	// Match "HTTP 503", "status code: 429", "error 500", "code 404", or standalone 3-digit tokens
	pats := []string{
		`(?i)\bhttp\s+(\d{3})\b`,
		`(?i)\bstatus(?:\s+code)?:\s*(\d{3})\b`,
		`(?i)\berror\s+(\d{3})\b`,
		`(?i)\bcode\s+(\d{3})\b`,
		`(?i)\b(\d{3})\b`,
	}
	for _, ptn := range pats {
		if m := regexp.MustCompile(ptn).FindStringSubmatch(errMsg); len(m) == 2 {
			if code, err := strconv.Atoi(m[1]); err == nil && code >= 100 && code < 600 {
				return code
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
	// Authorization patterns (use specific phrases to avoid overmatching e.g., "author")
	authPatterns := []string{
		"unauthorized", "invalid api key", "invalid_api_key", "api key",
		"authentication failed", "invalid token", "expired token",
		"invalid credentials", "forbidden", "permission denied",
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

func (p *ErrorParser) extractRetryAfter(err error) *time.Duration {
	if err == nil {
		return nil
	}
	type retryAfterProvider interface {
		RetryAfter() time.Duration
	}
	var rap retryAfterProvider
	if errors.As(err, &rap) {
		delay := rap.RetryAfter()
		if delay > 0 {
			return &delay
		}
	}
	return nil
}

func (p *ErrorParser) parseRetryAfter(headers map[string]string) *time.Duration {
	if len(headers) == 0 {
		return nil
	}
	for key, value := range headers {
		if strings.EqualFold(key, "Retry-After") {
			if delay := parseRetryAfterHeaderValue(value); delay != nil {
				return delay
			}
		}
	}
	return nil
}

func parseRetryAfterHeaderValue(value string) *time.Duration {
	if value == "" {
		return nil
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		delay := time.Duration(seconds) * time.Second
		if delay < 0 {
			delay = 0
		}
		return &delay
	}
	if when, err := http.ParseTime(trimmed); err == nil {
		delay := time.Until(when)
		if delay < 0 {
			delay = 0
		}
		return &delay
	}
	return nil
}

func parseRetryAfterFromMessage(message string) *time.Duration {
	if message == "" {
		return nil
	}
	matches := retryAfterMessagePattern.FindStringSubmatch(message)
	if len(matches) < 2 {
		return nil
	}
	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil || value < 0 {
		return nil
	}
	unit := "s"
	if len(matches) >= 3 && matches[2] != "" {
		unit = strings.ToLower(matches[2])
	}
	var delay time.Duration
	switch unit {
	case "millisecond", "milliseconds", "ms":
		delay = time.Duration(value * float64(time.Millisecond))
	case "minute", "minutes", "min", "mins", "m":
		delay = time.Duration(value * float64(time.Minute))
	case "hour", "hours", "hr", "hrs", "h":
		delay = time.Duration(value * float64(time.Hour))
	default:
		delay = time.Duration(value * float64(time.Second))
	}
	if delay < 0 {
		delay = 0
	}
	return &delay
}
