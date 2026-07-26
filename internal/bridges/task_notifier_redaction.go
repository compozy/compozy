package bridges

import (
	"bytes"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"

	taskpkg "github.com/compozy/agh/internal/task"
)

func terminalTaskNotificationEnvelope(
	subscription BridgeTaskSubscription,
	record taskpkg.EventRecord,
	eventStatus taskpkg.Status,
	run taskpkg.Run,
) (TerminalTaskNotification, error) {
	payload, err := redactTerminalTaskNotificationPayload(cloneRawJSON(record.Event.Payload))
	if err != nil {
		return TerminalTaskNotification{}, err
	}
	return TerminalTaskNotification{
		DeliveryID:     terminalTaskNotificationDeliveryID(subscription.SubscriptionID, record.Sequence),
		EventType:      record.Event.EventType,
		Final:          true,
		Seq:            record.Sequence,
		TaskID:         subscription.TaskID,
		RunID:          strings.TrimSpace(record.Event.RunID),
		Status:         eventStatus,
		Error:          redactTerminalTaskNotificationText(strings.TrimSpace(run.Error)),
		Payload:        payload,
		SubscriptionID: subscription.SubscriptionID,
	}, nil
}

func redactTerminalTaskNotificationPayload(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("bridges: decode terminal task notification payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("bridges: decode terminal task notification payload: trailing data")
		}
		return nil, fmt.Errorf("bridges: decode terminal task notification payload trailing data: %w", err)
	}
	encoded, err := json.Marshal(redactTerminalTaskNotificationValue(payload))
	if err != nil {
		return nil, fmt.Errorf("bridges: encode terminal task notification payload: %w", err)
	}
	return json.RawMessage(encoded), nil
}

func redactTerminalTaskNotificationValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			if terminalTaskNotificationKeyCarriesSecret(key) {
				redacted[key] = "[REDACTED]"
				continue
			}
			redacted[key] = redactTerminalTaskNotificationValue(nested)
		}
		return redacted
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, redactTerminalTaskNotificationValue(nested))
		}
		return redacted
	case string:
		return redactTerminalTaskNotificationText(typed)
	case json.Number:
		return typed
	default:
		return typed
	}
}

func redactTerminalTaskNotificationText(value string) string {
	return diagnostics.Redact(taskpkg.RedactClaimTokens(value))
}

func terminalTaskNotificationKeyCarriesSecret(key string) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	if normalized == "" {
		return false
	}
	for _, marker := range []string{
		"claimtoken",
		"apikey",
		"accesstoken",
		"refreshtoken",
		"mcpauthtoken",
		"oauthcode",
		"authorizationcode",
		"codeverifier",
		"pkceverifier",
		"secretbinding",
		"authorization",
		"password",
		"secret",
		"token",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
