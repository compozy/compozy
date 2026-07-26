package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"strings"

	"time"
)

func verifyLinearWebhookSignature(req *http.Request, body []byte, candidates []resolvedInstanceConfig) error {
	if req == nil {
		return errors.New("linear: webhook request is required")
	}
	signature := strings.TrimSpace(req.Header.Get("linear-signature"))
	if signature == "" {
		return errors.New("linear: webhook signature is required")
	}
	for _, cfg := range candidates {
		secret := strings.TrimSpace(cfg.webhookSecret)
		if secret == "" {
			continue
		}
		if validLinearWebhookSignature(secret, body, signature) {
			return nil
		}
	}
	return errors.New("linear: invalid webhook signature")
}

func selectLinearWebhookSignatureCandidates(
	body []byte,
	candidates []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	envelope := linearWebhookEnvelope{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return candidates, nil
	}

	mode, ok := linearWebhookModeForType(envelope.Type)
	organizationID := strings.TrimSpace(envelope.OrganizationID)
	if !ok || organizationID == "" {
		return candidates, nil
	}

	cfg, found, err := selectLinearConfig(candidates, organizationID, mode)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, errors.New("linear: invalid webhook signature")
	}
	return []resolvedInstanceConfig{cfg}, nil
}

func linearWebhookModeForType(eventType string) (string, bool) {
	switch strings.TrimSpace(eventType) {
	case providerCommentValue:
		return linearModeComments, true
	case providerAgentSessionEventValue:
		return linearModeAgentSessions, true
	default:
		return "", false
	}
}

func validateLinearWebhookTimestamp(timestampMS int64, receivedAt time.Time) error {
	if timestampMS <= 0 {
		return nil
	}
	when := time.UnixMilli(timestampMS).UTC()
	if when.Before(receivedAt.UTC().Add(-linearWebhookSkew)) {
		return errors.New("linear: webhook timestamp is stale")
	}
	return nil
}

func linearCommentIsSelf(cfg resolvedInstanceConfig, payload linearCommentWebhookPayload) bool {
	commentUserID := strings.TrimSpace(firstNonEmpty(payload.Data.User.ID, payload.Data.UserID, payload.Actor.ID))
	if commentUserID == "" || strings.TrimSpace(cfg.botUserID) == "" {
		return false
	}
	return commentUserID == strings.TrimSpace(cfg.botUserID)
}

func selectLinearConfig(
	candidates []resolvedInstanceConfig,
	organizationID string,
	mode string,
) (resolvedInstanceConfig, bool, error) {
	selected := resolvedInstanceConfig{}
	found := false
	for _, cfg := range candidates {
		if strings.TrimSpace(cfg.organizationID) != strings.TrimSpace(organizationID) ||
			strings.TrimSpace(cfg.mode) != strings.TrimSpace(mode) {
			continue
		}
		if found {
			return resolvedInstanceConfig{}, false, fmt.Errorf(
				"linear: multiple managed instances matched organization %q mode %q",
				organizationID,
				mode,
			)
		}
		selected = cfg
		found = true
	}
	return selected, found, nil
}

func (c resolvedInstanceConfig) ownershipKey() string {
	if strings.TrimSpace(c.organizationID) == "" || strings.TrimSpace(c.mode) == "" {
		return ""
	}
	return strings.TrimSpace(c.organizationID) + "|" + strings.TrimSpace(c.mode)
}

func (c resolvedInstanceConfig) graphqlURL() string {
	base := strings.TrimRight(strings.TrimSpace(c.apiBaseURL), "/")
	if strings.HasSuffix(base, "/graphql") {
		return base
	}
	return base + "/graphql"
}

func listenLinearWebhook(listenAddr string) (net.Listener, error) {
	var listenConfig net.ListenConfig
	return listenConfig.Listen(context.Background(), "tcp", strings.TrimSpace(listenAddr))
}

func linearDefaultOAuthTokenURL() string {
	return strings.TrimRight(linearDefaultAPIBaseURL, "/") + linearOAuthPathSuffix
}

func linearOAuthTokenURLEnvName() string {
	return strings.Join([]string{"AGH", "BRIDGE", "LINEAR", "TOKEN", "URL"}, "_")
}

func validLinearCredentialedURL(value string) bool {
	parsed, err := url.Parse(normalizeURL(value))
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch parsed.Scheme {
	case "https":
		return host == "api.linear.app"
	case "http":
		return isLoopbackBridgeHost(host)
	default:
		return false
	}
}

func isLoopbackBridgeHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + "|" + strings.TrimSpace(deliveryID)
}

func normalizeLinearMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "comments", providerCommentKey:
		return linearModeComments
	case "agent_sessions", "agent-sessions", "agent_session", "agent-session":
		return linearModeAgentSessions
	default:
		return normalized
	}
}

func normalizeLinearAuthMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "api_key", "api-key":
		return linearAuthModeAPIKey
	case "oauth":
		return linearAuthModeOAuth
	default:
		return normalized
	}
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func writeWebhookText(w http.ResponseWriter, statusCode int, body string) error {
	if w == nil {
		return nil
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(statusCode)
	_, err := io.WriteString(w, body)
	return err
}

func decodeLinearWebhookEnvelopeType(body []byte, receivedAt time.Time) (string, error) {
	envelope := linearWebhookEnvelope{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", err
	}
	if err := validateLinearWebhookTimestamp(envelope.WebhookTimestamp, receivedAt); err != nil {
		return "", err
	}
	return strings.TrimSpace(envelope.Type), nil
}
