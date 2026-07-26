package hooks

import (
	"context"
	"fmt"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

func guardNetworkParticipationPreResolvePatch(
	_ context.Context,
	hook RegisteredHook,
	payload NetworkParticipationPreResolvePayload,
	patch NetworkParticipationPreResolvePatch,
) error {
	if patch.Request == nil {
		return nil
	}
	if err := validateParticipationNarrowing(payload.Request, patch.Request); err != nil {
		return fmt.Errorf(
			"%w: hook %q network participation patch rejected: %w",
			ErrHookPatchRejected,
			hook.Name,
			err,
		)
	}
	return nil
}

func validateParticipationNarrowing(
	current *participation.Request,
	patched *participation.Request,
) error {
	patchedRequest, err := participation.NormalizeIntent(*patched)
	if err != nil {
		return fmt.Errorf("invalid patched request: %w", err)
	}
	patchedMode := participationRequestMode(patchedRequest)

	if current == nil {
		if patchedMode != participation.ModeLocal {
			return fmt.Errorf("a missing request can only be narrowed to local mode")
		}
		return nil
	}

	currentRequest, err := participation.NormalizeIntent(*current)
	if err != nil {
		return fmt.Errorf("invalid current request: %w", err)
	}
	currentMode := participationRequestMode(currentRequest)
	if patchedMode == participation.ModeLocal {
		return nil
	}
	if currentMode != participation.ModeLive {
		return fmt.Errorf("cannot widen %q participation to live mode", currentMode)
	}
	if !sameParticipationChannelIntent(currentRequest, patchedRequest) {
		return fmt.Errorf("cannot change live channel strategy or identity")
	}
	if err := validateNarrowerBounds(currentRequest.Bounds, patchedRequest.Bounds); err != nil {
		return err
	}
	return nil
}

func participationRequestMode(request participation.Request) participation.Mode {
	if request.Mode == nil {
		return participation.ModeLocal
	}
	return *request.Mode
}

func sameParticipationChannelIntent(current, patched participation.Request) bool {
	if current.ChannelStrategy == nil || patched.ChannelStrategy == nil ||
		*current.ChannelStrategy != *patched.ChannelStrategy {
		return false
	}
	return optionalStringEqual(current.ChannelID, patched.ChannelID)
}

func optionalStringEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateNarrowerBounds(current, patched *participation.BoundsRequest) error {
	if current == nil || patched == nil {
		if current == nil && patched == nil {
			return nil
		}
		return fmt.Errorf("cannot add or remove live bounds without their resolved defaults")
	}
	checks := []error{
		compareOptionalInt("max_wakes", current.MaxWakes, patched.MaxWakes),
		compareOptionalDurationMaximum(
			"max_wake_wall_time",
			current.MaxWakeWallTime,
			patched.MaxWakeWallTime,
		),
		compareOptionalDurationMaximum(
			"max_total_wall_time",
			current.MaxTotalWallTime,
			patched.MaxTotalWallTime,
		),
		compareOptionalInt64("max_input_tokens", current.MaxInputTokens, patched.MaxInputTokens),
		compareOptionalInt64("max_output_tokens", current.MaxOutputTokens, patched.MaxOutputTokens),
		compareOptionalInt("max_wake_depth", current.MaxWakeDepth, patched.MaxWakeDepth),
		compareOptionalDurationMinimum(
			"coalesce_window",
			current.CoalesceWindow,
			patched.CoalesceWindow,
		),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	return nil
}

func compareOptionalInt(field string, current, patched *int) error {
	if current == nil || patched == nil {
		if current == nil && patched == nil {
			return nil
		}
		return fmt.Errorf("cannot add or remove bound %s", field)
	}
	if *patched > *current {
		return fmt.Errorf("bound %s cannot increase from %d to %d", field, *current, *patched)
	}
	return nil
}

func compareOptionalInt64(field string, current, patched *int64) error {
	if current == nil || patched == nil {
		if current == nil && patched == nil {
			return nil
		}
		return fmt.Errorf("cannot add or remove bound %s", field)
	}
	if *patched > *current {
		return fmt.Errorf("bound %s cannot increase from %d to %d", field, *current, *patched)
	}
	return nil
}

func compareOptionalDurationMaximum(field string, current, patched *string) error {
	currentDuration, patchedDuration, err := comparableDurations(field, current, patched)
	if err != nil {
		return err
	}
	if patchedDuration > currentDuration {
		return fmt.Errorf("bound %s cannot increase from %s to %s", field, currentDuration, patchedDuration)
	}
	return nil
}

func compareOptionalDurationMinimum(field string, current, patched *string) error {
	currentDuration, patchedDuration, err := comparableDurations(field, current, patched)
	if err != nil {
		return err
	}
	if patchedDuration < currentDuration {
		return fmt.Errorf("bound %s cannot decrease from %s to %s", field, currentDuration, patchedDuration)
	}
	return nil
}

func comparableDurations(field string, current, patched *string) (time.Duration, time.Duration, error) {
	if current == nil || patched == nil {
		if current == nil && patched == nil {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("cannot add or remove bound %s", field)
	}
	currentDuration, err := time.ParseDuration(*current)
	if err != nil {
		return 0, 0, fmt.Errorf("parse current bound %s: %w", field, err)
	}
	patchedDuration, err := time.ParseDuration(*patched)
	if err != nil {
		return 0, 0, fmt.Errorf("parse patched bound %s: %w", field, err)
	}
	return currentDuration, patchedDuration, nil
}
