package automation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"strings"

	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
)

// SessionObserver exposes the existing session notifier shape for internal lifecycle ingress.
func (e *TriggerEngine) SessionObserver() session.Notifier {
	return &triggerSessionObserver{engine: e}
}

// HookTelemetrySink exposes the existing hook telemetry sink shape for hook-completion ingress.
func (e *TriggerEngine) HookTelemetrySink() hookspkg.TelemetrySink {
	return &triggerHookTelemetrySink{engine: e}
}

// MemoryObserver exposes the observer-facing dream-consolidation completion adapter.
func (e *TriggerEngine) MemoryObserver() MemoryConsolidationObserver {
	return &triggerMemoryObserver{engine: e}
}

// ParseWebhookEndpoint resolves the human slug and stable webhook id from an endpoint path segment.
func ParseWebhookEndpoint(endpoint string) (ParsedWebhookEndpoint, error) {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return ParsedWebhookEndpoint{}, ErrWebhookEndpointInvalid
	}

	separator := strings.LastIndex(trimmed, "--")
	if separator <= 0 || separator+2 >= len(trimmed) {
		return ParsedWebhookEndpoint{}, fmt.Errorf("%w: expected <slug>--<webhook_id>", ErrWebhookEndpointInvalid)
	}

	parsed := ParsedWebhookEndpoint{
		EndpointSlug: strings.TrimSpace(trimmed[:separator]),
		WebhookID:    strings.TrimSpace(trimmed[separator+2:]),
	}
	if parsed.EndpointSlug == "" || parsed.WebhookID == "" {
		return ParsedWebhookEndpoint{}, fmt.Errorf(
			"%w: expected non-empty slug and webhook id",
			ErrWebhookEndpointInvalid,
		)
	}
	if !strings.HasPrefix(parsed.WebhookID, "wbh_") {
		return ParsedWebhookEndpoint{}, fmt.Errorf(
			"%w: webhook id %q must start with \"wbh_\"",
			ErrWebhookEndpointInvalid,
			parsed.WebhookID,
		)
	}

	return parsed, nil
}

// FormatWebhookEndpoint returns the stable public endpoint segment for one webhook registration.
func FormatWebhookEndpoint(endpointSlug string, webhookID string) (string, error) {
	trimmedSlug := strings.TrimSpace(endpointSlug)
	trimmedWebhookID := strings.TrimSpace(webhookID)
	if trimmedSlug == "" || trimmedWebhookID == "" {
		return "", ErrWebhookEndpointInvalid
	}
	return trimmedSlug + "--" + trimmedWebhookID, nil
}

// SignWebhookPayload calculates the expected HMAC signature for a webhook request payload.
func SignWebhookPayload(secret string, timestamp time.Time, payload []byte) (string, error) {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return "", ErrWebhookSecretRequired
	}
	if timestamp.IsZero() {
		return "", errors.New("automation: webhook timestamp is required")
	}

	mac := hmac.New(sha256.New, []byte(trimmedSecret))
	if _, err := mac.Write(webhookSignaturePayload(timestamp, payload)); err != nil {
		return "", fmt.Errorf("automation: sign webhook payload: %w", err)
	}
	return webhookSignaturePrefix + hex.EncodeToString(mac.Sum(nil)), nil
}

// ValidateWebhookSignature verifies the provided signature before any trigger dispatch occurs.
func ValidateWebhookSignature(secret string, timestamp time.Time, payload []byte, signature string) error {
	expected, err := SignWebhookPayload(secret, timestamp, payload)
	if err != nil {
		return err
	}

	expectedMAC, err := decodeWebhookSignature(expected)
	if err != nil {
		return err
	}
	providedMAC, err := decodeWebhookSignature(signature)
	if err != nil {
		return err
	}
	if !hmac.Equal(providedMAC, expectedMAC) {
		return ErrWebhookSignatureInvalid
	}
	return nil
}

// ValidateWebhookTimestamp rejects stale or far-future webhook timestamps.
func ValidateWebhookTimestamp(timestamp time.Time, now time.Time, window time.Duration) error {
	if timestamp.IsZero() {
		return errors.New("automation: webhook timestamp is required")
	}
	if now.IsZero() {
		return errors.New("automation: current time is required")
	}
	if window <= 0 {
		return errors.New("automation: webhook freshness window must be positive")
	}

	delta := now.UTC().Sub(timestamp.UTC())
	if delta < 0 {
		delta = -delta
	}
	if delta > window {
		return ErrWebhookTimestampInvalid
	}
	return nil
}
