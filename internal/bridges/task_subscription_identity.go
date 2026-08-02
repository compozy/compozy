package bridges

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/notifications"
)

const bridgeTaskSubscriptionIdentityVersion = 1

type bridgeTaskSubscriptionTargetIdentity struct {
	Version          int          `json:"version"`
	Kind             string       `json:"kind"`
	SubscriptionID   string       `json:"subscription_id"`
	BridgeInstanceID string       `json:"bridge_instance_id"`
	PeerID           string       `json:"peer_id,omitempty"`
	ThreadID         string       `json:"thread_id,omitempty"`
	GroupID          string       `json:"group_id,omitempty"`
	DeliveryMode     DeliveryMode `json:"delivery_mode"`
}

func terminalTaskNotificationDeliveryID(
	subscription BridgeTaskSubscription,
	sequence int64,
) (string, error) {
	return terminalTaskNotificationOutcomeID(
		subscription,
		sequence,
		notifications.DeliveryKindDeliver,
		"",
	)
}

func terminalTaskNotificationSkippedDeliveryID(
	subscription BridgeTaskSubscription,
	sequence int64,
	reason string,
) (string, error) {
	return terminalTaskNotificationOutcomeID(
		subscription,
		sequence,
		notifications.DeliveryKindSkipped,
		reason,
	)
}

func terminalTaskNotificationOutcomeID(
	subscription BridgeTaskSubscription,
	sequence int64,
	kind notifications.DeliveryKind,
	reason string,
) (string, error) {
	if err := subscription.Validate(); err != nil {
		return "", err
	}
	normalized := subscription.Normalize()
	targetIdentity, err := encodeBridgeTaskSubscriptionTargetIdentity(normalized)
	if err != nil {
		return "", err
	}
	return notifications.EncodeDeliveryID(notifications.DeliveryIdentity{
		Cursor:         normalized.CursorKey(),
		TargetIdentity: targetIdentity,
		TargetIndex:    0,
		Sequence:       sequence,
		Kind:           kind,
		Reason:         reason,
	})
}

func encodeBridgeTaskSubscriptionTargetIdentity(
	subscription BridgeTaskSubscription,
) (string, error) {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "subscription id", value: subscription.SubscriptionID},
		{name: "bridge instance id", value: subscription.BridgeInstanceID},
		{name: "peer id", value: subscription.PeerID},
		{name: "thread id", value: subscription.ThreadID},
		{name: "group id", value: subscription.GroupID},
		{name: "delivery mode", value: string(subscription.DeliveryMode)},
	} {
		if !utf8.ValidString(field.value) {
			return "", fmt.Errorf(
				"%w: %s must be valid UTF-8",
				ErrInvalidBridgeTaskSubscription,
				field.name,
			)
		}
	}
	encoded, err := json.Marshal(bridgeTaskSubscriptionTargetIdentity{
		Version:          bridgeTaskSubscriptionIdentityVersion,
		Kind:             "bridge_task_subscription_target",
		SubscriptionID:   subscription.SubscriptionID,
		BridgeInstanceID: subscription.BridgeInstanceID,
		PeerID:           subscription.PeerID,
		ThreadID:         subscription.ThreadID,
		GroupID:          subscription.GroupID,
		DeliveryMode:     subscription.DeliveryMode,
	})
	if err != nil {
		return "", fmt.Errorf("bridges: encode task subscription target identity: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}
