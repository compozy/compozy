package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

const teamsTextFormatMarkdown = "markdown"

func referenceRemoteMessageID(reference *bridgepkg.DeliveryMessageReference) string {
	if reference == nil {
		return ""
	}
	return strings.TrimSpace(reference.RemoteMessageID)
}

func referenceDeliveryID(reference *bridgepkg.DeliveryMessageReference) string {
	if reference == nil {
		return ""
	}
	return strings.TrimSpace(reference.DeliveryID)
}

func classifyTeamsHTTPError(statusCode int, retryAfterHeader string, raw string) error {
	message := strings.TrimSpace(raw)
	if message == "" {
		message = fmt.Sprintf("teams: http %d", statusCode)
	}
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &bridgesdk.AuthError{Err: errors.New(message)}
	case http.StatusTooManyRequests:
		return &bridgesdk.RateLimitError{
			Err:        errors.New(message),
			RetryAfter: parseRetryAfter(retryAfterHeader),
		}
	default:
		return &bridgesdk.HTTPError{
			StatusCode: statusCode,
			Message:    message,
			RetryAfter: parseRetryAfter(retryAfterHeader),
		}
	}
}

func stringHeader(header map[string]any, key string) string {
	value, ok := header[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func defaultTeamsTokenURL(tenantID string) string {
	authority := "botframework.com"
	if trimmed := strings.TrimSpace(tenantID); trimmed != "" {
		authority = trimmed
	}
	return "https://login.microsoftonline.com/" + authority + "/oauth2/v2.0/token"
}

func teamsOAuthTokenURLEnvName() string {
	return strings.Join([]string{"AGH", "BRIDGE", "TEAMS", "TOKEN", "URL"}, "_")
}

func listenTeamsWebhook(listenAddr string) (net.Listener, error) {
	var listenConfig net.ListenConfig
	return listenConfig.Listen(context.Background(), "tcp", strings.TrimSpace(listenAddr))
}

func looksLikeTenantID(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.EqualFold(trimmed, "botframework.com") {
		return true
	}
	parts := strings.Split(trimmed, "-")
	if len(parts) != 5 {
		return false
	}
	return !slices.Contains(parts, "")
}

func validTeamsServiceURL(value string) bool {
	parsed, err := url.Parse(normalizeURL(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validatedTeamsCredentialedURL(value string, label string) (string, error) {
	normalized := normalizeURL(value)
	if !validTeamsCredentialedURL(normalized) {
		return "", fmt.Errorf("teams: %s %q is invalid", label, normalized)
	}
	return normalized, nil
}

func validTeamsCredentialedURL(value string) bool {
	parsed, err := url.Parse(normalizeURL(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch parsed.Scheme {
	case "https":
		return host == "login.botframework.com" || host == "login.microsoftonline.com"
	case "http":
		return teamsLoopbackCredentialedURLsEnabledForTesting() && isLoopbackTeamsHost(host)
	default:
		return false
	}
}

func teamsLoopbackCredentialedURLsEnabledForTesting() bool {
	return strings.TrimSpace(os.Getenv(teamsTestLoopbackAuthEnv)) == "1"
}

func isLoopbackTeamsHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func looksLikeTeamsUserID(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "29:") || strings.HasPrefix(trimmed, "8:orgid:") ||
		strings.HasPrefix(trimmed, "8:teamsvisitor:") {
		return true
	}
	if strings.HasPrefix(trimmed, "19:") || strings.Contains(trimmed, "@thread") {
		return false
	}
	return strings.Contains(trimmed, ":")
}

func parseTeamsTimestamp(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, trimmed)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func buildTeamsIDSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := normalizeTeamsID(value); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildTeamsUsernameSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := normalizeTeamsUsername(value); normalized != "" {
			out[normalized] = struct{}{}
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
}

func userContextKey(instanceID string, userID string) string {
	return strings.TrimSpace(instanceID) + ":" + normalizeTeamsID(userID)
}

func normalizeWebhookPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func normalizeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func writeWebhookOK(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("ok"))
	return err
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
