package presets

import (
	"context"
	"errors"
	"fmt"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/notifications"
)

func (s *Service) dispatchPreset(
	ctx context.Context,
	preset Preset,
	event Event,
) (DispatchResult, error) {
	compiled, err := CompileFilter(preset.Filter)
	if err != nil {
		key, keyErr := cursorKeyForTarget(preset, Target{}, 0, event)
		if keyErr != nil {
			return DispatchResult{Failed: 1}, keyErr
		}
		return DispatchResult{Failed: 1}, s.recordDispatchError(ctx, key, preset, event, err)
	}
	if !compiled.Eval(event) {
		return s.skipPresetTargets(ctx, preset, event, "filter")
	}
	if len(preset.Targets) == 0 {
		return s.skipPresetTargets(ctx, preset, event, "no_targets")
	}

	result, joined := s.dispatchPresetTargets(ctx, preset, event)
	if joined != nil {
		return result, joined
	}
	return result, nil
}

func (s *Service) dispatchPresetTargets(
	ctx context.Context,
	preset Preset,
	event Event,
) (DispatchResult, error) {
	result := DispatchResult{}
	var joined error
	for index, target := range preset.Targets {
		cursorKey, keyErr := cursorKeyForTarget(preset, target, index, event)
		if keyErr != nil {
			result.Failed++
			joined = errors.Join(joined, keyErr)
			continue
		}
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
		deliveryID, deliveryIDErr := deliveryIDForTarget(
			cursorKey,
			preset,
			target,
			index,
			event.Sequence,
		)
		if deliveryIDErr != nil {
			result.Failed++
			joined = errors.Join(joined, deliveryIDErr)
			continue
		}
		err := s.deliverTarget(ctx, preset, event, target, deliveryID, cursorKey)
		switch {
		case err == nil:
			result.Delivered++
			if _, advanceErr := s.advance(ctx, cursorKey, event, deliveryID); advanceErr != nil {
				result.Failed++
				joined = errors.Join(joined, advanceErr)
			}
		case errors.Is(err, bridgepkg.ErrBridgeNotificationSuppressed):
			result.Suppressed++
			skippedID, skippedIDErr := skippedDeliveryID(
				cursorKey,
				preset,
				target,
				index,
				event.Sequence,
				"suppressed",
			)
			if skippedIDErr != nil {
				result.Failed++
				joined = errors.Join(joined, skippedIDErr)
				continue
			}
			if _, advanceErr := s.advance(
				ctx,
				cursorKey,
				event,
				skippedID,
			); advanceErr != nil {
				result.Failed++
				joined = errors.Join(joined, advanceErr)
			}
		default:
			result.Failed++
			joined = errors.Join(joined, s.recordDispatchError(ctx, cursorKey, preset, event, err))
		}
	}
	return result, joined
}

func (s *Service) skipPresetTargets(
	ctx context.Context,
	preset Preset,
	event Event,
	reason string,
) (DispatchResult, error) {
	if len(preset.Targets) == 0 {
		key, keyErr := cursorKeyForTarget(preset, Target{}, 0, event)
		if keyErr != nil {
			return DispatchResult{Failed: 1}, keyErr
		}
		deliveryID, deliveryIDErr := skippedDeliveryID(
			key,
			preset,
			Target{},
			0,
			event.Sequence,
			reason,
		)
		if deliveryIDErr != nil {
			return DispatchResult{Failed: 1}, deliveryIDErr
		}
		_, err := s.advance(ctx, key, event, deliveryID)
		return DispatchResult{Skipped: 1}, err
	}
	result := DispatchResult{}
	var joined error
	for index, target := range preset.Targets {
		key, keyErr := cursorKeyForTarget(preset, target, index, event)
		if keyErr != nil {
			result.Failed++
			joined = errors.Join(joined, keyErr)
			continue
		}
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
		deliveryID, deliveryIDErr := skippedDeliveryID(
			key,
			preset,
			target,
			index,
			event.Sequence,
			reason,
		)
		if deliveryIDErr != nil {
			result.Failed++
			joined = errors.Join(joined, deliveryIDErr)
			continue
		}
		if _, err := s.advance(ctx, key, event, deliveryID); err != nil {
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
	cursorKey notifications.CursorKey,
) error {
	normalizedTarget, instance, err := s.deliverableBridge(ctx, preset, target)
	if err != nil {
		return err
	}
	delivery, err := s.deliveryForTarget(ctx, preset, event, normalizedTarget, instance, deliveryID)
	if err != nil {
		return err
	}
	permit := notifications.DeliveryPermit{
		Key: cursorKey, DeliveryID: deliveryID, AcquiredAt: s.now(),
	}
	if err := s.cursors.AcquireDeliveryPermit(ctx, permit); err != nil {
		return err
	}
	if err := s.deliverBridgeEvent(ctx, preset, event, instance, delivery); err != nil {
		return err
	}
	return nil
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
