package core

import (
	"regexp"
	"slices"
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
	// Scheme-based URIs with credentials (e.g., postgres://user:pass@host/db)
	connectionRe = regexp.MustCompile(
		`(?i)((postgres|postgresql|mysql|mongodb(\+srv)?|redis|rediss|amqp|amqps|https?)://)[^@\s]+@[^\s]+`,
	)
	// Env-var style key=value connection strings (e.g., DATABASE_URL=...)
	envConnRe = regexp.MustCompile(
		`(?i)\b((?:database_url|connection_string|conn_str|dsn)\s*[:=]\s*)([^"'\s:]+)(\s|$)`,
	)
	emailRe = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
)

// RedactString trims, truncates, and scrubs common secret patterns.
func RedactString(s string) string {
	const maxLen = 256
	s = strings.TrimSpace(s)
	// NOTE: Apply redaction patterns from most to least specific to avoid leaking tokens.
	s = jwtRe.ReplaceAllString(s, "[JWT_REDACTED]")
	s = awsKeyRe.ReplaceAllString(s, "[AWS_KEY_REDACTED]")
	s = githubTokenRe.ReplaceAllString(s, "[GITHUB_TOKEN_REDACTED]")
	s = slackTokenRe.ReplaceAllString(s, "[SLACK_TOKEN_REDACTED]")
	s = connectionRe.ReplaceAllString(s, "$1[REDACTED]")
	s = envConnRe.ReplaceAllString(s, "$1[REDACTED]")
	s = bearerTokenRe.ReplaceAllString(s, "$1[REDACTED]")
	s = kvSecretRe.ReplaceAllString(s, "$1=[REDACTED]")
	s = genericKeyRe.ReplaceAllString(s, "[REDACTED]")
	s = emailRe.ReplaceAllString(s, "[EMAIL_REDACTED]")
	if len(s) > maxLen {
		s = s[:maxLen] + "…"
	}
	return s
}

// RedactError applies RedactString to an error, returning an empty string when nil.
func RedactError(err error) string {
	if err == nil {
		return ""
	}
	return RedactString(err.Error())
}

// sensitiveSubstrings contains words that identify a sensitive header if they appear in any segment.
// These are typically nouns that strongly imply confidentiality.
var sensitiveSubstrings = []string{
	"password", "secret", "passwd", "pwd", "apikey", "api-key", "api_key",
	"private-key", "public-key", "secret-key", "access-key",
	"session", "credential", "cred",
}

// sensitiveSuffixes contains words that identify a sensitive header ONLY if they are the last segment.
// These are often standard header names or common suffixes for tokens and identifiers.
var sensitiveSuffixes = []string{
	"authorization", "token", "cookie", "auth", "key", "bearer", "jwt", "id",
}

// isSensitiveHeader checks if a header name contains sensitive information using segment-based matching.
// It splits the header by common delimiters and checks:
// 1. If any segment matches sensitive substrings (always sensitive)
// 2. If the last segment matches sensitive suffixes (contextually sensitive)
// 3. Also checks for compound patterns (e.g., "public-key")
func isSensitiveHeader(headerName string) bool {
	lowerName := strings.ToLower(headerName)
	compoundPatterns := []string{
		"api-key", "api_key", "apikey",
		"private-key", "private_key", "privatekey",
		"public-key", "public_key", "publickey",
		"secret-key", "secret_key", "secretkey",
		"access-key", "access_key", "accesskey",
	}
	for _, pattern := range compoundPatterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}
	segments := strings.FieldsFunc(lowerName, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	for _, segment := range segments {
		if slices.Contains(sensitiveSubstrings, segment) {
			return true
		}
	}
	if len(segments) > 0 {
		lastSegment := segments[len(segments)-1]
		if slices.Contains(sensitiveSuffixes, lastSegment) {
			return true
		}
	}
	return false
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
		switch {
		case strings.EqualFold(k, "authorization") || strings.EqualFold(k, "proxy-authorization"):
			out[k] = RedactString(v)
		case isSensitiveHeader(k) || strings.EqualFold(k, "set-cookie") || strings.EqualFold(k, "cookie"):
			out[k] = "[REDACTED]"
		default:
			// NOTE: Run non-sensitive headers through RedactString to catch embedded secrets.
			out[k] = RedactString(v)
		}
	}
	return out
}
