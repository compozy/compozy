package model

import (
	"errors"
	"net/url"
	"strings"
)

// ErrWebhookEndpointInvalid reports that a webhook endpoint value cannot be normalized.
var ErrWebhookEndpointInvalid = errors.New("automation: invalid webhook endpoint")

// FormatWebhookEndpoint returns the stable public endpoint segment for one webhook registration.
func FormatWebhookEndpoint(endpointSlug string, webhookID string) (string, error) {
	trimmedSlug := strings.TrimSpace(endpointSlug)
	trimmedWebhookID := strings.TrimSpace(webhookID)
	if trimmedSlug == "" || trimmedWebhookID == "" {
		return "", ErrWebhookEndpointInvalid
	}
	if !isWebhookEndpointSlug(trimmedSlug) {
		return "", ErrWebhookEndpointInvalid
	}
	return trimmedSlug + "--" + trimmedWebhookID, nil
}

func isWebhookEndpointSlug(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && trimmed != "." && trimmed != ".." && url.PathEscape(trimmed) == trimmed
}
