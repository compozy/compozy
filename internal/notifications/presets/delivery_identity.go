package presets

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/notifications"
)

const presetIdentityVersion = 1

type presetTargetIdentity struct {
	Version     int    `json:"version"`
	Kind        string `json:"kind"`
	Preset      string `json:"preset"`
	Target      Target `json:"target"`
	TargetIndex int    `json:"target_index"`
}

func cursorKeyForTarget(
	preset Preset,
	target Target,
	index int,
	event Event,
) (notifications.CursorKey, error) {
	consumerID, err := encodePresetTargetIdentity(preset, target, index)
	if err != nil {
		return notifications.CursorKey{}, err
	}
	return notifications.CursorKey{
		ProfileID:  event.ProfileID,
		Scope:      event.Scope,
		ConsumerID: consumerID,
		StreamName: event.Type,
		SubjectID:  event.ID,
	}.Normalize()
}

func deliveryIDForTarget(
	key notifications.CursorKey,
	preset Preset,
	target Target,
	index int,
	sequence int64,
) (string, error) {
	targetID, err := encodePresetTargetIdentity(preset, target, index)
	if err != nil {
		return "", err
	}
	return notifications.EncodeDeliveryID(notifications.DeliveryIdentity{
		Cursor:         key,
		TargetIdentity: targetID,
		TargetIndex:    index,
		Sequence:       sequence,
		Kind:           notifications.DeliveryKindDeliver,
	})
}

func skippedDeliveryID(
	key notifications.CursorKey,
	preset Preset,
	target Target,
	index int,
	sequence int64,
	reason string,
) (string, error) {
	targetID, err := encodePresetTargetIdentity(preset, target, index)
	if err != nil {
		return "", err
	}
	return notifications.EncodeDeliveryID(notifications.DeliveryIdentity{
		Cursor:         key,
		TargetIdentity: targetID,
		TargetIndex:    index,
		Sequence:       sequence,
		Kind:           notifications.DeliveryKindSkipped,
		Reason:         reason,
	})
}

func encodePresetTargetIdentity(preset Preset, target Target, index int) (string, error) {
	if index < 0 {
		return "", fmt.Errorf("%w: target index cannot be negative", ErrInvalidPreset)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "preset name", value: preset.Name},
		{name: "target bridge id", value: target.BridgeID},
		{name: "target canonical route", value: target.CanonicalRoute},
		{name: "target display name", value: target.DisplayName},
		{name: "target delivery mode", value: string(target.DeliveryMode)},
	} {
		if !utf8.ValidString(field.value) {
			return "", fmt.Errorf("%w: %s must be valid UTF-8", ErrInvalidPreset, field.name)
		}
	}
	normalizedPreset := preset.Normalize()
	normalizedTarget := target.Normalize()
	encoded, err := json.Marshal(presetTargetIdentity{
		Version:     presetIdentityVersion,
		Kind:        "preset_target",
		Preset:      normalizedPreset.Name,
		Target:      normalizedTarget,
		TargetIndex: index,
	})
	if err != nil {
		return "", fmt.Errorf("notifications: encode preset target identity: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}
