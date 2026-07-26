package presets

import (
	"context"

	"errors"
	"fmt"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/notifications"
)

func (s *Service) dispatchPreset(
	ctx context.Context,
	preset Preset,
	event Event,
) (DispatchResult, error) {
	compiled, err := CompileFilter(preset.Filter)
	if err != nil {
		key := cursorKeyForTarget(preset, Target{}, event)
		return DispatchResult{Failed: 1}, s.recordDispatchError(ctx, key, preset, event, err)
	}
	if !compiled.Eval(event) {
		return s.skipPresetTargets(ctx, preset, event, "filter")
	}
	if len(preset.Targets) == 0 {
		key := cursorKeyForTarget(preset, Target{}, event)
		_, advanceErr := s.advance(ctx, key, event, skipDeliveryID(preset, event, "no_targets"))
		return DispatchResult{Skipped: 1}, advanceErr
	}

	result := DispatchResult{}
	var joined error
	for index, target := range preset.Targets {
		cursorKey := cursorKeyForTarget(preset, target, event)
		cursor, cursorErr := s.cursors.Get(ctx, cursorKey)
		if cursorErr != nil && !errors.Is(cursorErr, notifications.ErrCursorNotFound) {
			result.Failed++
			joined = errors.Join(joined, cursorErr)
			continue
		}
		if cursor.LastSequence >= event.Sequence {
			result.Skipped++
			continue
		}
		deliveryID := deliveryIDForTarget(preset, event, index)
		err := s.deliverTarget(ctx, preset, event, target, deliveryID)
		switch {
		case err == nil:
			result.Delivered++
			if _, advanceErr := s.advance(ctx, cursorKey, event, deliveryID); advanceErr != nil {
				result.Failed++
				joined = errors.Join(joined, advanceErr)
			}
		case errors.Is(err, bridgepkg.ErrBridgeNotificationSuppressed):
			result.Suppressed++
			if _, advanceErr := s.advance(
				ctx,
				cursorKey,
				event,
				skipDeliveryID(preset, event, "suppressed"),
			); advanceErr != nil {
				result.Failed++
				joined = errors.Join(joined, advanceErr)
			}
		default:
			result.Failed++
			joined = errors.Join(joined, s.recordDispatchError(ctx, cursorKey, preset, event, err))
		}
	}
	if joined != nil {
		return result, joined
	}
	return result, nil
}

func (s *Service) skipPresetTargets(
	ctx context.Context,
	preset Preset,
	event Event,
	reason string,
) (DispatchResult, error) {
	if len(preset.Targets) == 0 {
		key := cursorKeyForTarget(preset, Target{}, event)
		_, err := s.advance(ctx, key, event, skipDeliveryID(preset, event, reason))
		return DispatchResult{Skipped: 1}, err
	}
	result := DispatchResult{}
	var joined error
	for _, target := range preset.Targets {
		key := cursorKeyForTarget(preset, target, event)
		cursor, cursorErr := s.cursors.Get(ctx, key)
		if cursorErr != nil && !errors.Is(cursorErr, notifications.ErrCursorNotFound) {
			result.Failed++
			joined = errors.Join(joined, cursorErr)
			continue
		}
		if cursor.LastSequence >= event.Sequence {
			result.Skipped++
			continue
		}
		if _, err := s.advance(ctx, key, event, skipDeliveryID(preset, event, reason)); err != nil {
			result.Failed++
			joined = errors.Join(joined, err)
			continue
		}
		result.Skipped++
	}
	return result, joined
}

func (s *Service) deliverTarget(
	ctx context.Context,
	preset Preset,
	event Event,
	target Target,
	deliveryID string,
) error {
	normalizedTarget, instance, err := s.deliverableBridge(ctx, preset, target)
	if err != nil {
		return err
	}
	delivery, err := s.deliveryForTarget(ctx, preset, event, normalizedTarget, instance, deliveryID)
	if err != nil {
		return err
	}
	return s.deliverBridgeEvent(ctx, preset, event, instance, delivery)
}

func (s *Service) deliverableBridge(
	ctx context.Context,
	preset Preset,
	target Target,
) (Target, bridgepkg.BridgeInstance, error) {
	normalizedTarget := target.Normalize()
	if err := normalizedTarget.Validate(); err != nil {
		return Target{}, bridgepkg.BridgeInstance{}, err
	}
	instance, err := s.bridges.GetBridgeInstance(ctx, normalizedTarget.BridgeID)
	if err != nil {
		return Target{}, bridgepkg.BridgeInstance{}, fmt.Errorf(
			"notifications: load bridge %q for preset %q: %w",
			normalizedTarget.BridgeID,
			preset.Name,
			err,
		)
	}
	if instance.NotificationSuppress {
		return Target{}, bridgepkg.BridgeInstance{}, fmt.Errorf(
			"%w: bridge instance %q",
			bridgepkg.ErrBridgeNotificationSuppressed,
			instance.ID,
		)
	}
	if !instance.Enabled || instance.Status.Normalize() != bridgepkg.BridgeStatusReady {
		return Target{}, bridgepkg.BridgeInstance{}, fmt.Errorf(
			"%w: bridge instance %q status %q enabled=%t",
			bridgepkg.ErrBridgeInstanceUnavailable,
			instance.ID,
			instance.Status.Normalize(),
			instance.Enabled,
		)
	}
	return normalizedTarget, instance, nil
}
