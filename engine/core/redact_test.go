package core_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestRedactString(t *testing.T) {
	t.Run("Should trim and truncate long strings", func(t *testing.T) {
		longString := "   " + strings.Repeat("a", 300) + "   "
		result := core.RedactString(longString)
		// The string is trimmed first, then truncated to 256 bytes + "…" (which is 3 bytes in UTF-8)
		assert.LessOrEqual(t, len(result), 259) // Max 256 + 3 bytes for "…"
		assert.True(t, strings.HasSuffix(result, "…"))
		// Verify the actual content length before ellipsis
		assert.Equal(t, 256, len(result)-3)
	})
	t.Run("Should redact Bearer tokens", func(t *testing.T) {
		input := "Authorization: Bearer abc123def456ghi789"
		result := core.RedactString(input)
		assert.Equal(t, "Authorization: Bearer [REDACTED]", result)
	})
	t.Run("Should redact API keys in various formats", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"api_key=secret123", "api_key=[REDACTED]"},
			{"api-key: 'secret123'", "api-key=[REDACTED]"},
			{"API_KEY=\"secret123\"", "API_KEY=[REDACTED]"},
			{"token=abc123xyz", "token=[REDACTED]"},
			{"secret: mysecret", "secret=[REDACTED]"},
			{"password=mypass123", "password=[REDACTED]"},
			{"pwd: hunter2", "pwd=[REDACTED]"},
			{"access_token=xyz789", "access_token=[REDACTED]"},
		}
		for _, tc := range testCases {
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should redact generic keys", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"sk-1234567890123456", "[REDACTED]"},
			{"pk-abcdef1234567890", "[REDACTED]"},
			{"api_1234567890123456", "[REDACTED]"},
			{"key-xyz1234567890123", "[REDACTED]"},
		}
		for _, tc := range testCases {
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should redact JWT tokens", func(t *testing.T) {
		jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
		input := "token: " + jwt
		result := core.RedactString(input)
		assert.Equal(t, "token=[REDACTED]", result)
	})
	t.Run("Should redact AWS keys", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"AKIAIOSFODNN7EXAMPLE", "[AWS_KEY_REDACTED]"},
			{"aws_access_key_id: AKIAIOSFODNN7EXAMPLE", "[AWS_KEY_REDACTED]"},
		}
		for _, tc := range testCases {
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should redact GitHub tokens", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"ghp_" + string(make([]byte, 36)), "[GITHUB_TOKEN_REDACTED]"},
			{"gho_" + string(make([]byte, 36)), "[GITHUB_TOKEN_REDACTED]"},
			{"ghs_" + string(make([]byte, 36)), "[GITHUB_TOKEN_REDACTED]"},
			{"ghr_" + string(make([]byte, 36)), "[GITHUB_TOKEN_REDACTED]"},
		}
		for _, tc := range testCases {
			// Fill with valid characters
			tc.input = tc.input[:4] + strings.Repeat("a", 36)
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should redact Slack tokens", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"xoxb-123456789012", "[SLACK_TOKEN_REDACTED]"},
			{"xoxa-2-123456789012", "[SLACK_TOKEN_REDACTED]"},
			{"xoxp-123456789012", "[SLACK_TOKEN_REDACTED]"},
			{"xoxr-123456789012", "[SLACK_TOKEN_REDACTED]"},
			{"xoxs-123456789012", "[SLACK_TOKEN_REDACTED]"},
		}
		for _, tc := range testCases {
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should redact connection strings", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"postgres://user:pass@localhost/db", "postgres://[REDACTED]"},
			{"mysql://root:secret@127.0.0.1:3306/mydb", "mysql://[REDACTED]"},
			{"mongodb://admin:password@cluster.mongodb.net/test", "mongodb://[REDACTED]"},
			{"redis://user:pass@redis.example.com:6379", "redis://[REDACTED]"},
			{"DATABASE_URL=postgres://user:pass@host/db", "DATABASE_URL=postgres://[REDACTED]"},
		}
		for _, tc := range testCases {
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should redact email addresses", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected string
		}{
			{"user@example.com", "[EMAIL_REDACTED]"},
			{"contact: admin@company.org", "contact: [EMAIL_REDACTED]"},
			{"john.doe+tag@subdomain.example.co.uk", "[EMAIL_REDACTED]"},
		}
		for _, tc := range testCases {
			result := core.RedactString(tc.input)
			assert.Equal(t, tc.expected, result, "Failed for input: %s", tc.input)
		}
	})
	t.Run("Should handle multiple secrets in one string", func(t *testing.T) {
		input := "Bearer abc123 api_key=secret email@test.com sk-1234567890123456"
		result := core.RedactString(input)
		assert.Contains(t, result, "Bearer [REDACTED]")
		assert.Contains(t, result, "api_key=[REDACTED]")
		assert.Contains(t, result, "[EMAIL_REDACTED]")
		assert.NotContains(t, result, "sk-1234567890123456")
	})
	t.Run("Should preserve non-sensitive content", func(t *testing.T) {
		input := "This is a normal log message with no secrets"
		result := core.RedactString(input)
		assert.Equal(t, input, result)
	})
}

func TestRedactError(t *testing.T) {
	t.Run("Should return empty string for nil error", func(t *testing.T) {
		result := core.RedactError(nil)
		assert.Equal(t, "", result)
	})
	t.Run("Should redact error message with secrets", func(t *testing.T) {
		err := errors.New("connection failed: postgres://user:password@localhost/db")
		result := core.RedactError(err)
		assert.Equal(t, "connection failed: postgres://[REDACTED]", result)
	})
	t.Run("Should handle normal error messages", func(t *testing.T) {
		err := errors.New("file not found")
		result := core.RedactError(err)
		assert.Equal(t, "file not found", result)
	})
}

func TestRedactHeaders(t *testing.T) {
	t.Run("Should return empty map for empty input", func(t *testing.T) {
		result := core.RedactHeaders(map[string]string{})
		assert.Empty(t, result)
		result = core.RedactHeaders(nil)
		assert.Empty(t, result)
	})
	t.Run("Should preserve Authorization scheme", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "Bearer abc123xyz789",
		}
		result := core.RedactHeaders(headers)
		assert.Equal(t, "Bearer [REDACTED]", result["Authorization"])
	})
	t.Run("Should handle Proxy-Authorization", func(t *testing.T) {
		headers := map[string]string{
			"Proxy-Authorization": "Basic dXNlcjpwYXNz",
		}
		result := core.RedactHeaders(headers)
		assert.Equal(t, "Basic dXNlcjpwYXNz", result["Proxy-Authorization"])
	})
	t.Run("Should redact sensitive headers completely", func(t *testing.T) {
		headers := map[string]string{
			"X-Api-Key":    "secret123",
			"X-Auth-Token": "token456",
			"X-Access-Key": "access789",
			"Cookie":       "session=abc123",
			"Set-Cookie":   "token=xyz789",
			"X-Secret":     "mysecret",
			"Api-Password": "pass123",
			"Session-Id":   "sess456",
			"Credential":   "cred789",
		}
		result := core.RedactHeaders(headers)
		for k := range headers {
			assert.Equal(t, "[REDACTED]", result[k], "Failed for header: %s", k)
		}
	})
	t.Run("Should redact embedded secrets in non-sensitive headers", func(t *testing.T) {
		headers := map[string]string{
			"User-Agent":    "MyApp/1.0 token=abc123",
			"Referer":       "https://example.com?api_key=secret",
			"Content-Type":  "application/json",
			"Cache-Control": "max-age=3600",
		}
		result := core.RedactHeaders(headers)
		assert.Equal(t, "MyApp/1.0 token=[REDACTED]", result["User-Agent"])
		assert.Equal(t, "https://example.com?api_key=[REDACTED]", result["Referer"])
		assert.Equal(t, "application/json", result["Content-Type"])
		assert.Equal(t, "max-age=3600", result["Cache-Control"])
	})
	t.Run("Should handle case-insensitive header names", func(t *testing.T) {
		headers := map[string]string{
			"AUTHORIZATION": "Bearer token123",
			"x-api-key":     "key456",
			"X-API-KEY":     "key789",
		}
		result := core.RedactHeaders(headers)
		assert.Equal(t, "Bearer [REDACTED]", result["AUTHORIZATION"])
		assert.Equal(t, "[REDACTED]", result["x-api-key"])
		assert.Equal(t, "[REDACTED]", result["X-API-KEY"])
	})
	t.Run("Should handle headers with 'key' in the name", func(t *testing.T) {
		headers := map[string]string{
			"X-Key":         "secret",
			"API-Key":       "secret",
			"Public-Key-Id": "not-sensitive", // This will still be redacted due to 'key' pattern
		}
		result := core.RedactHeaders(headers)
		for k := range headers {
			assert.Equal(t, "[REDACTED]", result[k], "Failed for header: %s", k)
		}
	})
	t.Run("Should not false positive on common words containing 'key'", func(t *testing.T) {
		// These headers should NOT be redacted based on the header name alone
		// (though their values might still be redacted if they contain secrets)
		headers := map[string]string{
			"X-Monkey-Header":   "banana",
			"X-Hockey-League":   "NHL",
			"X-Turkey-Region":   "Istanbul",
			"X-Keyboard-Layout": "QWERTY",
			"X-Donkey-Mode":     "enabled",
		}
		result := core.RedactHeaders(headers)
		// These headers should pass through since they don't match sensitive patterns
		assert.Equal(t, "banana", result["X-Monkey-Header"])
		assert.Equal(t, "NHL", result["X-Hockey-League"])
		assert.Equal(t, "Istanbul", result["X-Turkey-Region"])
		assert.Equal(t, "QWERTY", result["X-Keyboard-Layout"])
		assert.Equal(t, "enabled", result["X-Donkey-Mode"])
	})
}
