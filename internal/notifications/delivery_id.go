package notifications

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// DeliveryKind classifies whether a cursor advanced after an outbound send or a skip.
type DeliveryKind string

const (
	// DeliveryKindDeliver identifies an outbound delivery attempt acknowledged by the bridge.
	DeliveryKindDeliver DeliveryKind = "deliver"
	// DeliveryKindSkipped identifies a deliberate cursor advance without an outbound delivery.
	DeliveryKindSkipped DeliveryKind = "skipped"
)

const deliveryIDVersion = 1

const deliveryIDPrefix = "nd1_"

// DeliveryIdentity is the complete deterministic identity of one notification outcome.
type DeliveryIdentity struct {
	Cursor         CursorKey    `json:"cursor"`
	TargetIdentity string       `json:"target_identity"`
	TargetIndex    int          `json:"target_index"`
	Sequence       int64        `json:"sequence"`
	Kind           DeliveryKind `json:"kind"`
	Reason         string       `json:"reason,omitempty"`
}

type encodedDeliveryIdentity struct {
	Version        int          `json:"version"`
	Cursor         CursorKey    `json:"cursor"`
	TargetIdentity string       `json:"target_identity"`
	TargetIndex    int          `json:"target_index"`
	Sequence       int64        `json:"sequence"`
	Kind           DeliveryKind `json:"kind"`
	Reason         string       `json:"reason,omitempty"`
}

// EncodeDeliveryID returns an opaque, versioned, deterministic delivery identity.
func EncodeDeliveryID(identity DeliveryIdentity) (string, error) {
	cursor, err := identity.Cursor.Normalize()
	if err != nil {
		return "", err
	}
	normalized := DeliveryIdentity{
		Cursor:         cursor,
		TargetIdentity: identity.TargetIdentity,
		TargetIndex:    identity.TargetIndex,
		Sequence:       identity.Sequence,
		Kind:           identity.Kind,
		Reason:         identity.Reason,
	}
	if normalized.TargetIdentity == "" {
		return "", fmt.Errorf("%w: delivery target identity is required", ErrInvalidCursor)
	}
	if err := validateNotificationIdentityUTF8(normalized.TargetIdentity, "delivery target identity"); err != nil {
		return "", err
	}
	if err := validateNotificationIdentityUTF8(normalized.Reason, "delivery reason"); err != nil {
		return "", err
	}
	if normalized.TargetIndex < 0 {
		return "", fmt.Errorf("%w: delivery target index must be zero or greater", ErrInvalidCursor)
	}
	if normalized.Sequence <= 0 {
		return "", fmt.Errorf("%w: delivery sequence must be greater than zero", ErrInvalidCursor)
	}
	switch normalized.Kind {
	case DeliveryKindDeliver:
		if normalized.Reason != "" {
			return "", fmt.Errorf("%w: delivered notification must not have a skip reason", ErrInvalidCursor)
		}
	case DeliveryKindSkipped:
		if normalized.Reason == "" {
			return "", fmt.Errorf("%w: skipped notification reason is required", ErrInvalidCursor)
		}
	default:
		return "", fmt.Errorf(
			"%w: delivery kind must be %q or %q",
			ErrInvalidCursor,
			DeliveryKindDeliver,
			DeliveryKindSkipped,
		)
	}
	encoded, err := json.Marshal(encodedDeliveryIdentity{
		Version:        deliveryIDVersion,
		Cursor:         normalized.Cursor,
		TargetIdentity: normalized.TargetIdentity,
		TargetIndex:    normalized.TargetIndex,
		Sequence:       normalized.Sequence,
		Kind:           normalized.Kind,
		Reason:         normalized.Reason,
	})
	if err != nil {
		return "", fmt.Errorf("notifications: encode delivery identity: %w", err)
	}
	return deliveryIDPrefix + base64.RawURLEncoding.EncodeToString(encoded), nil
}

// DecodeDeliveryID validates and returns an opaque delivery identity.
func DecodeDeliveryID(value string) (DeliveryIdentity, error) {
	encoded := value
	if !strings.HasPrefix(encoded, deliveryIDPrefix) {
		return DeliveryIdentity{}, fmt.Errorf("%w: delivery id has an unsupported version", ErrInvalidCursor)
	}
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(encoded, deliveryIDPrefix))
	if err != nil {
		return DeliveryIdentity{}, fmt.Errorf("%w: decode delivery id: %v", ErrInvalidCursor, err)
	}
	var decoded encodedDeliveryIdentity
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return DeliveryIdentity{}, fmt.Errorf("%w: decode delivery id payload: %v", ErrInvalidCursor, err)
	}
	if decoded.Version != deliveryIDVersion {
		return DeliveryIdentity{}, fmt.Errorf("%w: delivery id has an unsupported version", ErrInvalidCursor)
	}
	identity := DeliveryIdentity{
		Cursor:         decoded.Cursor,
		TargetIdentity: decoded.TargetIdentity,
		TargetIndex:    decoded.TargetIndex,
		Sequence:       decoded.Sequence,
		Kind:           decoded.Kind,
		Reason:         decoded.Reason,
	}
	canonical, err := EncodeDeliveryID(identity)
	if err != nil {
		return DeliveryIdentity{}, err
	}
	if canonical != encoded {
		return DeliveryIdentity{}, fmt.Errorf("%w: delivery id is not canonical", ErrInvalidCursor)
	}
	return identity, nil
}
