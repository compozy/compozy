package main

import (
	"bytes"

	"encoding/json"

	"fmt"

	"net/http"

	"strconv"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func classifyWhatsAppHTTPError(statusCode int, retryAfterHeader string, raw []byte) error {
	retryAfter := parseRetryAfter(retryAfterHeader)
	envelope := whatsappGraphAPIErrorEnvelope{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &envelope); err != nil {
			envelope.Error.Message = strings.TrimSpace(string(raw))
		}
	}

	message := strings.TrimSpace(envelope.Error.Message)
	if message == "" {
		message = fmt.Sprintf("whatsapp graph api request failed with status %d", statusCode)
	}
	code := envelope.Error.Code
	subcode := envelope.Error.ErrorSubcode

	switch {
	case statusCode == http.StatusUnauthorized, statusCode == http.StatusForbidden, code == 190:
		return &bridgesdk.AuthError{
			Err: &bridgesdk.HTTPError{
				StatusCode: statusCode,
				Message:    message,
				RetryAfter: retryAfter,
			},
		}
	case statusCode == http.StatusTooManyRequests,
		code == 4,
		code == 80007,
		code == 130429,
		subcode == 2494010:
		return &bridgesdk.RateLimitError{
			Err: &bridgesdk.HTTPError{
				StatusCode: maxInt(statusCode, http.StatusTooManyRequests),
				Message:    message,
				RetryAfter: retryAfter,
			},
			RetryAfter: retryAfter,
		}
	case statusCode == http.StatusRequestTimeout, statusCode == http.StatusGatewayTimeout:
		return &bridgesdk.HTTPError{
			StatusCode: statusCode,
			Message:    message,
			RetryAfter: retryAfter,
		}
	case statusCode >= http.StatusInternalServerError:
		return &bridgesdk.HTTPError{
			StatusCode: statusCode,
			Message:    message,
			RetryAfter: retryAfter,
		}
	default:
		return &bridgesdk.HTTPError{
			StatusCode: statusCode,
			Message:    message,
			RetryAfter: retryAfter,
		}
	}
}

func matchesPhoneNumberID(cfg resolvedInstanceConfig, incoming string) bool {
	return strings.TrimSpace(incoming) == "" ||
		strings.TrimSpace(incoming) == strings.TrimSpace(cfg.phoneNumberID)
}

func contactsByWaID(items []whatsappContact) map[string]*whatsappContact {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]*whatsappContact, len(items))
	for idx := range items {
		item := items[idx]
		waID := strings.TrimSpace(item.WaID)
		if waID == "" {
			continue
		}
		copyItem := item
		result[waID] = &copyItem
	}
	return result
}

func parseUnixTimestamp(value string) time.Time {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
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
	return strings.TrimRight(trimmed, "/")
}

func buildIdentitySet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := normalizeUsername(value)
		if trimmed == "" {
			trimmed = strings.TrimSpace(value)
		}
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
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(value, "@")))
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func maxInt(values ...int) int {
	result := 0
	for _, value := range values {
		if value > result {
			result = value
		}
	}
	return result
}
