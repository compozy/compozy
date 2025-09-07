package core

import (
	"regexp"
	"strings"
)

// Precompiled patterns for common secret shapes in error/log strings.
var (
	bearerTokenRe = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9\-\._~\+\/]+=*`)
	kvSecretRe    = regexp.MustCompile(
		`(?i)(api[_-]?key|token|secret|password|pass|pwd|credential|auth|authorization_token|access_token|refresh_token)\s*[:=]\s*["']?[^"'\s]+["']?`,
	)
	genericKeyRe = regexp.MustCompile(
		`\b(sk-[A-Za-z0-9_\-]{16,}|pk-[A-Za-z0-9_\-]{16,}|api_[A-Za-z0-9_\-]{16,}|key-[A-Za-z0-9_\-]{16,})\b`,
	)
	jwtRe         = regexp.MustCompile(`\b(eyJ[A-Za-z0-9_\-]+\.eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+)\b`)
	awsKeyRe      = regexp.MustCompile(`\b(AKIA[A-Z0-9]{16}|aws_[a-z]+_key_id\s*[:=]\s*[A-Z0-9]{20})\b`)
	githubTokenRe = regexp.MustCompile(
		`\b(ghp_[A-Za-z0-9]{36}|gho_[A-Za-z0-9]{36}|ghs_[A-Za-z0-9]{36}|ghr_[A-Za-z0-9]{36})\b`,
	)
	slackTokenRe = regexp.MustCompile(`\b(xox[baprs]-[A-Za-z0-9\-]{10,})\b`)
	connectionRe = regexp.MustCompile(
		`(?i)(postgres|mysql|mongodb|redis|amqp|database_url|connection_string|conn_str|dsn)://[^@\s]+@[^\s]+`,
	)
	emailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
)

// RedactString trims, truncates, and scrubs common secret patterns.
func RedactString(s string) string {
	const maxLen = 256
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	// Apply redaction patterns in order of specificity
	s = jwtRe.ReplaceAllString(s, "[JWT_REDACTED]")
	s = awsKeyRe.ReplaceAllString(s, "[AWS_KEY_REDACTED]")
	s = githubTokenRe.ReplaceAllString(s, "[GITHUB_TOKEN_REDACTED]")
	s = slackTokenRe.ReplaceAllString(s, "[SLACK_TOKEN_REDACTED]")
	s = connectionRe.ReplaceAllString(s, "$1://[REDACTED]")
	s = bearerTokenRe.ReplaceAllString(s, "$1[REDACTED]")
	s = kvSecretRe.ReplaceAllString(s, "$1=[REDACTED]")
	s = genericKeyRe.ReplaceAllString(s, "[REDACTED]")
	s = emailRe.ReplaceAllString(s, "[EMAIL_REDACTED]")
	return s
}

// RedactError applies RedactString to an error, returning an empty string when nil.
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	return RedactString(err.Error())
}

// sensitiveHeaderPatterns contains patterns to identify sensitive headers
var sensitiveHeaderPatterns = []string{
	"token", "secret", "api-key", "apikey", "key",
	"password", "passwd", "pwd", "auth",
	"credential", "cred", "session",
	"cookie", "jwt", "bearer",
	"x-api-", "x-auth-", "x-access-",
}

// RedactHeaders returns a copy of headers with sensitive values redacted for logging.
// Any header key containing sensitive patterns (case-insensitive),
// or the Authorization header, is considered sensitive. For Authorization, the scheme is preserved
// via RedactString (e.g., "Bearer [REDACTED]"). Other sensitive headers are replaced entirely
// with "[REDACTED]". Non-sensitive headers are passed through RedactString for safety.
func RedactHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		lk := strings.ToLower(k)
		isSensitive := false
		// Check if header name contains any sensitive pattern
		for _, pattern := range sensitiveHeaderPatterns {
			if strings.Contains(lk, pattern) {
				isSensitive = true
				break
			}
		}
		// Special handling for specific headers
		switch {
		case strings.EqualFold(k, "authorization") || strings.EqualFold(k, "proxy-authorization"):
			out[k] = RedactString(v)
		case isSensitive || strings.EqualFold(k, "set-cookie") || strings.EqualFold(k, "cookie"):
			out[k] = "[REDACTED]"
		default:
			// Still pass through RedactString for non-sensitive headers to catch embedded secrets
			out[k] = RedactString(v)
		}
	}
	return out
}
