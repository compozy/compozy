package cliutils

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// HTTPError represents an HTTP-specific error
type HTTPError struct {
	StatusCode int    `json:"status_code"`
	Method     string `json:"method"`
	URL        string `json:"url"`
	Message    string `json:"message"`
	Body       string `json:"body,omitempty"`
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d %s %s: %s", e.StatusCode, e.Method, e.URL, e.Message)
}

// NewHTTPError creates a new HTTP error
func NewHTTPError(statusCode int, method, url, message string) *HTTPError {
	return &HTTPError{
		StatusCode: statusCode,
		Method:     method,
		URL:        url,
		Message:    message,
	}
}

// IsHTTPError checks if an error is an HTTP error
func IsHTTPError(err error) bool {
	_, ok := err.(*HTTPError)
	return ok
}

// GetHTTPStatusCategory returns the category of an HTTP status code
func GetHTTPStatusCategory(statusCode int) string {
	switch {
	case statusCode >= 100 && statusCode < 200:
		return "informational"
	case statusCode >= 200 && statusCode < 300:
		return "success"
	case statusCode >= 300 && statusCode < 400:
		return "redirection"
	case statusCode >= 400 && statusCode < 500:
		return "client_error"
	case statusCode >= 500 && statusCode < 600:
		return "server_error"
	default:
		return "unknown"
	}
}

// IsRetryableHTTPError checks if an HTTP error is retryable
func IsRetryableHTTPError(err error) bool {
	if httpErr, ok := err.(*HTTPError); ok {
		// Retry on server errors and specific client errors
		switch httpErr.StatusCode {
		case http.StatusTooManyRequests, // 429
			http.StatusRequestTimeout,      // 408
			http.StatusConflict,            // 409 (sometimes retryable)
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout:      // 504
			return true
		}
	}
	return false
}

// BuildURL constructs a URL from components
func BuildURL(baseURL, path string, params map[string]string) (string, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid base URL: %w", err)
	}

	// Join path
	if path != "" {
		base.Path = strings.TrimSuffix(base.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	}

	// Add query parameters
	if len(params) > 0 {
		query := base.Query()
		for key, value := range params {
			query.Set(key, value)
		}
		base.RawQuery = query.Encode()
	}

	return base.String(), nil
}

// ParsePaginationParams parses pagination parameters from strings
func ParsePaginationParams(limitStr, offsetStr string) (limit, offset int, err error) {
	// Default values
	limit = 50
	offset = 0

	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid limit: %w", err)
		}
		if limit < 1 || limit > 1000 {
			return 0, 0, fmt.Errorf("limit must be between 1 and 1000")
		}
	}

	if offsetStr != "" {
		offset, err = strconv.Atoi(offsetStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid offset: %w", err)
		}
		if offset < 0 {
			return 0, 0, fmt.Errorf("offset must be non-negative")
		}
	}

	return limit, offset, nil
}

// BuildFilters constructs query filters from various inputs
func BuildFilters(params map[string]string) map[string]string {
	filters := make(map[string]string)

	// Standard filter parameters
	filterKeys := []string{
		"status", "name", "type", "tag", "tags",
		"created_after", "created_before", "updated_after", "updated_before",
		"workflow_id", "execution_id", "user_id",
	}

	for _, key := range filterKeys {
		if value := params[key]; value != "" {
			filters[key] = value
		}
	}

	return filters
}

// ValidateURL validates that a string is a valid URL
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return NewCliError("INVALID_URL", "URL cannot be empty")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return NewCliError("INVALID_URL", "Invalid URL format", err.Error())
	}

	if parsedURL.Scheme == "" {
		return NewCliError("INVALID_URL", "URL must include a scheme (http:// or https://)")
	}

	if parsedURL.Host == "" {
		return NewCliError("INVALID_URL", "URL must include a host")
	}

	return nil
}

// ExtractHostFromURL extracts the host from a URL string
func ExtractHostFromURL(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	return parsedURL.Host, nil
}

// IsLocalhost checks if a URL points to localhost
func IsLocalhost(urlStr string) bool {
	host, err := ExtractHostFromURL(urlStr)
	if err != nil {
		return false
	}
	return host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "localhost:")
}

// HTTPClient represents a simplified HTTP client interface
type HTTPClient interface {
	Get(ctx context.Context, url string) (*http.Response, error)
	Post(ctx context.Context, url string, body any) (*http.Response, error)
	Put(ctx context.Context, url string, body any) (*http.Response, error)
	Delete(ctx context.Context, url string) (*http.Response, error)
}

// RetryConfig configures retry behavior for HTTP requests
type RetryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns a default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:  3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     5 * time.Second,
		Multiplier:  2.0,
	}
}

// CalculateRetryWait calculates the wait time for a retry attempt
func (rc *RetryConfig) CalculateRetryWait(attempt int) time.Duration {
	if attempt <= 0 {
		return rc.InitialWait
	}

	wait := time.Duration(float64(rc.InitialWait) * (rc.Multiplier * float64(attempt)))
	if wait > rc.MaxWait {
		wait = rc.MaxWait
	}

	return wait
}

// FormatHTTPError formats HTTP errors for display
func FormatHTTPError(err error) string {
	if httpErr, ok := err.(*HTTPError); ok {
		category := GetHTTPStatusCategory(httpErr.StatusCode)
		return fmt.Sprintf("HTTP %d (%s): %s", httpErr.StatusCode, category, httpErr.Message)
	}
	return err.Error()
}

// ParseIDFromPath extracts an ID from a URL path
func ParseIDFromPath(path string) (core.ID, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "", NewCliError("INVALID_PATH", "Path cannot be empty")
	}

	// Get the last part of the path as the ID
	idStr := parts[len(parts)-1]
	return ParseID(idStr)
}

// SanitizeURLForLog removes sensitive information from URLs for logging
func SanitizeURLForLog(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr // Return as-is if parsing fails
	}

	// Remove sensitive query parameters
	query := parsedURL.Query()
	sensitiveParams := []string{"api_key", "token", "password", "secret", "key"}
	for _, param := range sensitiveParams {
		if query.Has(param) {
			query.Set(param, "[REDACTED]")
		}
	}

	// Remove user info from URL
	parsedURL.User = nil
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String()
}
