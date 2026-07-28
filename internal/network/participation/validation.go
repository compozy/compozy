package participation

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var channelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func Validate(request Request) (Request, error) {
	return validateRequest(request, Bounds{}, Limits{}, true)
}

func ValidateWithBounds(request Request, defaults Bounds, limits Limits) (Request, error) {
	return validateRequest(request, defaults, limits, false)
}

// NormalizeIntent validates and canonicalizes persisted execution intent
// without resolving omitted live bounds against runtime defaults. Resolution
// remains the single place that fills defaults and enforces ceilings.
func NormalizeIntent(request Request) (Request, error) {
	normalized := normalizeRequest(request)
	if normalized.isZero() {
		return normalized, nil
	}
	mode := ModeLocal
	if normalized.Mode != nil {
		mode = *normalized.Mode
	}
	if err := validateMode(mode); err != nil {
		return Request{}, err
	}
	if mode == ModeLocal {
		if normalized.ChannelStrategy != nil || normalized.ChannelID != nil || normalized.Bounds != nil {
			return Request{}, newContractError(
				ErrStrategyChannelConflict,
				"mode",
				"local mode cannot carry channel strategy, channel id, or bounds",
			)
		}
		return normalized, nil
	}
	if normalized.ChannelStrategy == nil {
		return Request{}, newContractError(ErrStrategyInvalid, "channel_strategy", "live mode requires a strategy")
	}
	if err := validateStrategy(*normalized.ChannelStrategy); err != nil {
		return Request{}, err
	}
	if *normalized.ChannelStrategy == StrategyNamed {
		if normalized.ChannelID == nil || *normalized.ChannelID == "" {
			return Request{}, newContractError(ErrStrategyInvalid, "channel_id", "named strategy requires a channel id")
		}
		if !channelPattern.MatchString(*normalized.ChannelID) {
			return Request{}, newContractError(
				ErrStrategyInvalid,
				"channel_id",
				"must match %q",
				channelPattern.String(),
			)
		}
	} else if normalized.ChannelID != nil {
		return Request{}, newContractError(
			ErrStrategyChannelConflict,
			"channel_id",
			"strategy %q derives its channel and cannot carry channel_id",
			*normalized.ChannelStrategy,
		)
	}
	if err := validatePartialBounds(normalized.Bounds); err != nil {
		return Request{}, err
	}
	return normalized, nil
}

func validateRequest(request Request, defaults Bounds, limits Limits, requireCompleteBounds bool) (Request, error) {
	normalized := normalizeRequest(request)
	mode := ModeLocal
	if normalized.Mode != nil {
		mode = *normalized.Mode
	}
	if err := validateMode(mode); err != nil {
		return Request{}, err
	}
	if normalized.Mode == nil {
		normalized.Mode = new(ModeLocal)
	}
	if mode == ModeLocal {
		if normalized.ChannelStrategy != nil || normalized.ChannelID != nil || normalized.Bounds != nil {
			return Request{}, newContractError(
				ErrStrategyChannelConflict,
				"mode",
				"local mode cannot carry channel strategy, channel id, or bounds",
			)
		}
		return normalized, nil
	}
	if normalized.ChannelStrategy == nil {
		return Request{}, newContractError(ErrStrategyInvalid, "channel_strategy", "live mode requires a strategy")
	}
	if err := validateStrategy(*normalized.ChannelStrategy); err != nil {
		return Request{}, err
	}
	if *normalized.ChannelStrategy == StrategyNamed {
		if normalized.ChannelID == nil || *normalized.ChannelID == "" {
			return Request{}, newContractError(ErrStrategyInvalid, "channel_id", "named strategy requires a channel id")
		}
		if !channelPattern.MatchString(*normalized.ChannelID) {
			return Request{}, newContractError(
				ErrStrategyInvalid,
				"channel_id",
				"must match %q",
				channelPattern.String(),
			)
		}
	} else if normalized.ChannelID != nil {
		return Request{}, newContractError(
			ErrStrategyChannelConflict,
			"channel_id",
			"strategy %q derives its channel and cannot carry channel_id",
			*normalized.ChannelStrategy,
		)
	}
	if requireCompleteBounds && normalized.Bounds == nil {
		return Request{}, newContractError(
			ErrBoundsExceedCeiling,
			"bounds",
			"bounds_required: live mode requires finite bounds",
		)
	}
	resolved, err := ResolveBounds(normalized.Bounds, defaults, limits)
	if err != nil {
		return Request{}, err
	}
	normalized.Bounds = boundsRequestFromBounds(resolved)
	return normalized, nil
}

func normalizeRequest(request Request) Request {
	if request.Mode != nil {
		mode := Mode(strings.TrimSpace(string(*request.Mode)))
		request.Mode = &mode
	}
	if request.ChannelStrategy != nil {
		strategy := ChannelStrategy(strings.TrimSpace(string(*request.ChannelStrategy)))
		request.ChannelStrategy = &strategy
	}
	if request.ChannelID != nil {
		channelID := strings.TrimSpace(*request.ChannelID)
		request.ChannelID = &channelID
	}
	request.Bounds = normalizeBoundsRequest(request.Bounds)
	return request
}

func normalizeBoundsRequest(request *BoundsRequest) *BoundsRequest {
	if request == nil {
		return nil
	}
	normalized := *request
	if request.MaxWakes != nil {
		normalized.MaxWakes = new(*request.MaxWakes)
	}
	if request.MaxWakeWallTime != nil {
		normalized.MaxWakeWallTime = new(strings.TrimSpace(*request.MaxWakeWallTime))
	}
	if request.MaxTotalWallTime != nil {
		normalized.MaxTotalWallTime = new(strings.TrimSpace(*request.MaxTotalWallTime))
	}
	if request.MaxInputTokens != nil {
		normalized.MaxInputTokens = new(*request.MaxInputTokens)
	}
	if request.MaxOutputTokens != nil {
		normalized.MaxOutputTokens = new(*request.MaxOutputTokens)
	}
	if request.MaxWakeDepth != nil {
		normalized.MaxWakeDepth = new(*request.MaxWakeDepth)
	}
	if request.CoalesceWindow != nil {
		normalized.CoalesceWindow = new(strings.TrimSpace(*request.CoalesceWindow))
	}
	return &normalized
}

func validatePartialBounds(request *BoundsRequest) error {
	if request == nil {
		return nil
	}
	for _, candidate := range []struct {
		field string
		value *int64
	}{
		{field: "bounds.max_wakes", value: intPointerAsInt64(request.MaxWakes)},
		{field: boundsMaxInputCountPath, value: request.MaxInputTokens},
		{field: boundsMaxOutputCountPath, value: request.MaxOutputTokens},
		{field: "bounds.max_wake_depth", value: intPointerAsInt64(request.MaxWakeDepth)},
	} {
		if candidate.value != nil && *candidate.value <= 0 {
			return newContractError(
				ErrBoundsExceedCeiling,
				candidate.field,
				"bounds_required: value must be positive",
			)
		}
	}
	if err := validatePartialTokenBounds(request); err != nil {
		return err
	}
	var wakeWall, totalWall *time.Duration
	for _, candidate := range []struct {
		field string
		value *string
	}{
		{field: "bounds.max_wake_wall_time", value: request.MaxWakeWallTime},
		{field: "bounds.max_total_wall_time", value: request.MaxTotalWallTime},
		{field: "bounds.coalesce_window", value: request.CoalesceWindow},
	} {
		if candidate.value == nil {
			continue
		}
		duration, err := parsePositiveDuration(candidate.field, *candidate.value)
		if err != nil {
			return err
		}
		switch candidate.field {
		case "bounds.max_wake_wall_time":
			wakeWall = &duration
		case "bounds.max_total_wall_time":
			totalWall = &duration
		}
	}
	if wakeWall != nil && totalWall != nil && *wakeWall > *totalWall {
		return exceedsLimit(
			"bounds.max_wake_wall_time",
			*request.MaxWakeWallTime,
			"bounds.max_total_wall_time",
			*request.MaxTotalWallTime,
		)
	}
	return nil
}

func validatePartialTokenBounds(request *BoundsRequest) error {
	for _, candidate := range []struct {
		field string
		value *int64
	}{
		{field: boundsMaxInputCountPath, value: request.MaxInputTokens},
		{field: boundsMaxOutputCountPath, value: request.MaxOutputTokens},
	} {
		if candidate.value != nil && *candidate.value > maxJavaScriptSafeInteger {
			return exceedsLimit(
				candidate.field,
				*candidate.value,
				"JavaScript safe integer ceiling",
				maxJavaScriptSafeInteger,
			)
		}
	}
	return nil
}

func intPointerAsInt64(value *int) *int64 {
	if value == nil {
		return nil
	}
	converted := int64(*value)
	return &converted
}

func validateMode(mode Mode) error {
	switch mode {
	case ModeLocal, ModeLive:
		return nil
	default:
		return newContractError(ErrStrategyInvalid, "mode", "unsupported value %q; allowed values: local, live", mode)
	}
}

func validateStrategy(strategy ChannelStrategy) error {
	switch strategy {
	case StrategyNamed, StrategyRun, StrategyLoopRun:
		return nil
	default:
		return newContractError(
			ErrStrategyInvalid,
			"channel_strategy",
			"unsupported value %q; allowed values: named, run, loop_run",
			strategy,
		)
	}
}

func boundsRequestFromBounds(bounds Bounds) *BoundsRequest {
	return &BoundsRequest{
		MaxWakes:         new(bounds.MaxWakes),
		MaxWakeWallTime:  new(bounds.MaxWakeWallTime),
		MaxTotalWallTime: new(bounds.MaxTotalWallTime),
		MaxInputTokens:   new(bounds.MaxInputTokens),
		MaxOutputTokens:  new(bounds.MaxOutputTokens),
		MaxWakeDepth:     new(bounds.MaxWakeDepth),
		CoalesceWindow:   new(bounds.CoalesceWindow),
	}
}

func requestBounds(request Request) (Bounds, error) {
	if request.Bounds == nil {
		return Bounds{}, fmt.Errorf("validated live request has no bounds")
	}
	return ResolveBounds(request.Bounds, Bounds{}, Limits{})
}
