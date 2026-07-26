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

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func writeWebhookJSON(w http.ResponseWriter, body any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(body)
}

func normalizeWebhookPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return trimmed
}

func normalizeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	return strings.TrimRight(parsed.String(), "/")
}

func joinGChatURL(base string, path string) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return "", err
	}
	ref, err := url.Parse(strings.TrimSpace(path))
	if err != nil {
		return "", err
	}
	return baseURL.ResolveReference(ref).String(), nil
}

func newValidatedGChatURL(value string) (validatedGChatURL, error) {
	normalized := normalizeURL(value)
	if err := validateGChatEndpointURL(normalized); err != nil {
		return "", err
	}
	return validatedGChatURL(normalized), nil
}

func newGChatRequest(
	ctx context.Context,
	method string,
	endpoint validatedGChatURL,
	body io.Reader,
) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parsed, err := url.ParseRequestURI(string(endpoint))
	if err != nil {
		return nil, err
	}
	req := &http.Request{
		Method: method,
		URL:    parsed,
		Header: make(http.Header),
	}
	if body != nil {
		req.Body = io.NopCloser(body)
	}
	req = req.WithContext(ctx)
	if err := validateGChatRequestURL(req); err != nil {
		return nil, err
	}
	return req, nil
}

func validateGChatRequestURL(req *http.Request) error {
	if req == nil || req.URL == nil {
		return errors.New("gchat: request url is required")
	}
	return validateGChatEndpointURL(req.URL.String())
}

func validateGChatEndpointURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("gchat: parse api url: %w", err)
	}
	if parsed == nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return errors.New("gchat: api url host is required")
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return nil
	}
	if !strings.EqualFold(parsed.Scheme, "http") {
		return fmt.Errorf("gchat: api url scheme %q is not allowed", parsed.Scheme)
	}

	host := strings.TrimSpace(parsed.Hostname())
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("gchat: insecure api host %q is not allowed", host)
}

func buildIdentitySet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := normalizeUsername(value)
		if trimmed == "" {
			continue
		}
		result[trimmed] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeUsername(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "@")
	return strings.ToLower(trimmed)
}

func referenceRemoteMessageID(reference *bridgepkg.DeliveryMessageReference) string {
	if reference == nil {
		return ""
	}
	return strings.TrimSpace(reference.RemoteMessageID)
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
}

func normalizeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeGChatMode(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validGChatMode(value string) bool {
	switch normalizeGChatMode(value) {
	case gchatModeDirect, gchatModePubSub, gchatModeHybrid:
		return true
	default:
		return false
	}
}

func modeUsesDirectIngress(mode string) bool {
	switch normalizeGChatMode(mode) {
	case gchatModeDirect, gchatModeHybrid:
		return true
	default:
		return false
	}
}

func modeUsesPubSubIngress(mode string) bool {
	switch normalizeGChatMode(mode) {
	case gchatModePubSub, gchatModeHybrid:
		return true
	default:
		return false
	}
}
