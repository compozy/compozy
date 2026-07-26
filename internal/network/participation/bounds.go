package participation

import (
	"fmt"
	"strings"
	"time"
)

const (
	maxMillisecondCeilingDuration = time.Duration(1<<63-1) - (time.Millisecond - 1)
	maxJavaScriptSafeInteger      = int64(9_007_199_254_740_991)
	boundsMaxInputCountPath       = "bounds.max_input_tokens"
	boundsMaxOutputCountPath      = "bounds.max_output_tokens"
)

type Bounds struct {
	MaxWakes         int    `json:"max_wakes"           yaml:"max_wakes"           toml:"max_wakes"`
	MaxWakeWallTime  string `json:"max_wake_wall_time"  yaml:"max_wake_wall_time"  toml:"max_wake_wall_time"`
	MaxTotalWallTime string `json:"max_total_wall_time" yaml:"max_total_wall_time" toml:"max_total_wall_time"`
	MaxInputTokens   int64  `json:"max_input_tokens"    yaml:"max_input_tokens"    toml:"max_input_tokens"`
	MaxOutputTokens  int64  `json:"max_output_tokens"   yaml:"max_output_tokens"   toml:"max_output_tokens"`
	MaxWakeDepth     int    `json:"max_wake_depth"      yaml:"max_wake_depth"      toml:"max_wake_depth"`
	CoalesceWindow   string `json:"coalesce_window"     yaml:"coalesce_window"     toml:"coalesce_window"`
}

func (b Bounds) IsZero() bool {
	return b == Bounds{}
}

type BoundsRequest struct {
	MaxWakes         *int    `json:"max_wakes,omitempty"           yaml:"max_wakes,omitempty"           toml:"max_wakes,omitempty"`
	MaxWakeWallTime  *string `json:"max_wake_wall_time,omitempty"  yaml:"max_wake_wall_time,omitempty"  toml:"max_wake_wall_time,omitempty"`
	MaxTotalWallTime *string `json:"max_total_wall_time,omitempty" yaml:"max_total_wall_time,omitempty" toml:"max_total_wall_time,omitempty"`
	MaxInputTokens   *int64  `json:"max_input_tokens,omitempty"    yaml:"max_input_tokens,omitempty"    toml:"max_input_tokens,omitempty"`
	MaxOutputTokens  *int64  `json:"max_output_tokens,omitempty"   yaml:"max_output_tokens,omitempty"   toml:"max_output_tokens,omitempty"`
	MaxWakeDepth     *int    `json:"max_wake_depth,omitempty"      yaml:"max_wake_depth,omitempty"      toml:"max_wake_depth,omitempty"`
	CoalesceWindow   *string `json:"coalesce_window,omitempty"     yaml:"coalesce_window,omitempty"     toml:"coalesce_window,omitempty"`
}

type Limits struct {
	MaxWakes          int
	MaxWakeWallTime   string
	MaxTotalWallTime  string
	MaxInputTokens    int64
	MaxOutputTokens   int64
	MaxWakeDepth      int
	MinCoalesceWindow string
	MaxCoalesceWindow string
}

func ResolveBounds(request *BoundsRequest, defaults Bounds, limits Limits) (Bounds, error) {
	resolved := defaults
	if request != nil {
		applyBoundsRequest(&resolved, *request)
	}
	resolved = normalizeBounds(resolved)
	if err := validateBounds(resolved); err != nil {
		return Bounds{}, err
	}
	if err := validateBoundsLimits(resolved, limits); err != nil {
		return Bounds{}, err
	}
	return resolved, nil
}

func applyBoundsRequest(dst *Bounds, request BoundsRequest) {
	if request.MaxWakes != nil {
		dst.MaxWakes = *request.MaxWakes
	}
	if request.MaxWakeWallTime != nil {
		dst.MaxWakeWallTime = *request.MaxWakeWallTime
	}
	if request.MaxTotalWallTime != nil {
		dst.MaxTotalWallTime = *request.MaxTotalWallTime
	}
	if request.MaxInputTokens != nil {
		dst.MaxInputTokens = *request.MaxInputTokens
	}
	if request.MaxOutputTokens != nil {
		dst.MaxOutputTokens = *request.MaxOutputTokens
	}
	if request.MaxWakeDepth != nil {
		dst.MaxWakeDepth = *request.MaxWakeDepth
	}
	if request.CoalesceWindow != nil {
		dst.CoalesceWindow = *request.CoalesceWindow
	}
}

func normalizeBounds(bounds Bounds) Bounds {
	bounds.MaxWakeWallTime = strings.TrimSpace(bounds.MaxWakeWallTime)
	bounds.MaxTotalWallTime = strings.TrimSpace(bounds.MaxTotalWallTime)
	bounds.CoalesceWindow = strings.TrimSpace(bounds.CoalesceWindow)
	return bounds
}

func validateBounds(bounds Bounds) error {
	positiveInts := []struct {
		field string
		value int64
	}{
		{field: "bounds.max_wakes", value: int64(bounds.MaxWakes)},
		{field: boundsMaxInputCountPath, value: bounds.MaxInputTokens},
		{field: boundsMaxOutputCountPath, value: bounds.MaxOutputTokens},
		{field: "bounds.max_wake_depth", value: int64(bounds.MaxWakeDepth)},
	}
	for _, candidate := range positiveInts {
		if candidate.value <= 0 {
			return newContractError(ErrBoundsExceedCeiling, candidate.field, "bounds_required: value must be positive")
		}
	}
	if err := validateJavaScriptSafeTokenBounds(bounds.MaxInputTokens, bounds.MaxOutputTokens); err != nil {
		return err
	}
	wakeWall, err := parsePositiveDuration("bounds.max_wake_wall_time", bounds.MaxWakeWallTime)
	if err != nil {
		return err
	}
	totalWall, err := parsePositiveDuration("bounds.max_total_wall_time", bounds.MaxTotalWallTime)
	if err != nil {
		return err
	}
	if wakeWall > totalWall {
		return exceedsLimit(
			"bounds.max_wake_wall_time",
			bounds.MaxWakeWallTime,
			"bounds.max_total_wall_time",
			bounds.MaxTotalWallTime,
		)
	}
	_, err = parsePositiveDuration("bounds.coalesce_window", bounds.CoalesceWindow)
	return err
}

func validateBoundsLimits(bounds Bounds, limits Limits) error {
	if limits == (Limits{}) {
		return nil
	}
	if limits.MaxWakes > 0 && bounds.MaxWakes > limits.MaxWakes {
		return exceedsLimit("bounds.max_wakes", bounds.MaxWakes, "network.live.limits.max_wakes", limits.MaxWakes)
	}
	if limits.MaxInputTokens > 0 && bounds.MaxInputTokens > limits.MaxInputTokens {
		return exceedsLimit(
			boundsMaxInputCountPath,
			bounds.MaxInputTokens,
			"network.live.limits.max_input_tokens",
			limits.MaxInputTokens,
		)
	}
	if limits.MaxOutputTokens > 0 && bounds.MaxOutputTokens > limits.MaxOutputTokens {
		return exceedsLimit(
			boundsMaxOutputCountPath,
			bounds.MaxOutputTokens,
			"network.live.limits.max_output_tokens",
			limits.MaxOutputTokens,
		)
	}
	if limits.MaxWakeDepth > 0 && bounds.MaxWakeDepth > limits.MaxWakeDepth {
		return exceedsLimit(
			"bounds.max_wake_depth",
			bounds.MaxWakeDepth,
			"network.live.limits.max_wake_depth",
			limits.MaxWakeDepth,
		)
	}
	if err := validateDurationCeiling(
		"bounds.max_wake_wall_time",
		bounds.MaxWakeWallTime,
		"network.live.limits.max_wake_wall_time",
		limits.MaxWakeWallTime,
	); err != nil {
		return err
	}
	if err := validateDurationCeiling(
		"bounds.max_total_wall_time",
		bounds.MaxTotalWallTime,
		"network.live.limits.max_total_wall_time",
		limits.MaxTotalWallTime,
	); err != nil {
		return err
	}
	return validateDurationRange(
		"bounds.coalesce_window",
		bounds.CoalesceWindow,
		"network.live.limits.min_coalesce_window",
		limits.MinCoalesceWindow,
		"network.live.limits.max_coalesce_window",
		limits.MaxCoalesceWindow,
	)
}

func validateDurationCeiling(field, value, limitField, limitValue string) error {
	if strings.TrimSpace(limitValue) == "" {
		return nil
	}
	duration, err := parsePositiveDuration(field, value)
	if err != nil {
		return err
	}
	limit, err := parsePositiveDuration(limitField, limitValue)
	if err != nil {
		return err
	}
	if duration > limit {
		return exceedsLimit(field, value, limitField, limitValue)
	}
	return nil
}

func validateDurationRange(field, value, minField, minValue, maxField, maxValue string) error {
	duration, err := parsePositiveDuration(field, value)
	if err != nil {
		return err
	}
	if strings.TrimSpace(minValue) != "" {
		minimum, minErr := parsePositiveDuration(minField, minValue)
		if minErr != nil {
			return minErr
		}
		if duration < minimum {
			return newContractError(ErrBoundsExceedCeiling, field, "value %s is below %s=%s", value, minField, minValue)
		}
	}
	if strings.TrimSpace(maxValue) != "" {
		maximum, maxErr := parsePositiveDuration(maxField, maxValue)
		if maxErr != nil {
			return maxErr
		}
		if duration > maximum {
			return exceedsLimit(field, value, maxField, maxValue)
		}
	}
	return nil
}

func parsePositiveDuration(field, value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, newContractError(ErrBoundsExceedCeiling, field, "bounds_required: duration is required")
	}
	duration, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, newContractError(ErrBoundsExceedCeiling, field, "invalid duration %q: %v", value, err)
	}
	if duration <= 0 {
		return 0, newContractError(ErrBoundsExceedCeiling, field, "duration must be positive: %q", value)
	}
	if duration > maxMillisecondCeilingDuration {
		return 0, newContractError(
			ErrBoundsExceedCeiling,
			field,
			"duration %q exceeds the safe millisecond ceiling",
			value,
		)
	}
	return duration, nil
}

func exceedsLimit(field string, value any, limitField string, limit any) error {
	return newContractError(
		ErrBoundsExceedCeiling,
		field,
		"value %v exceeds %s=%v",
		value,
		limitField,
		limit,
	)
}

func validateJavaScriptSafeTokenBounds(inputTokens, outputTokens int64) error {
	for _, candidate := range []struct {
		field string
		value int64
	}{
		{field: boundsMaxInputCountPath, value: inputTokens},
		{field: boundsMaxOutputCountPath, value: outputTokens},
	} {
		if candidate.value > maxJavaScriptSafeInteger {
			return exceedsLimit(
				candidate.field,
				candidate.value,
				"JavaScript safe integer ceiling",
				maxJavaScriptSafeInteger,
			)
		}
	}
	return nil
}

func (l Limits) Validate() error {
	if l.MaxWakes <= 0 || l.MaxInputTokens <= 0 || l.MaxOutputTokens <= 0 || l.MaxWakeDepth <= 0 {
		return fmt.Errorf("network participation limits must be positive: %+v", l)
	}
	if err := validateJavaScriptSafeTokenBounds(l.MaxInputTokens, l.MaxOutputTokens); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("network.live.limits.max_wake_wall_time", l.MaxWakeWallTime); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("network.live.limits.max_total_wall_time", l.MaxTotalWallTime); err != nil {
		return err
	}
	minimum, err := parsePositiveDuration("network.live.limits.min_coalesce_window", l.MinCoalesceWindow)
	if err != nil {
		return err
	}
	maximum, err := parsePositiveDuration("network.live.limits.max_coalesce_window", l.MaxCoalesceWindow)
	if err != nil {
		return err
	}
	if minimum > maximum {
		return fmt.Errorf(
			"network.live.limits.min_coalesce_window must be <= network.live.limits.max_coalesce_window: %s > %s",
			l.MinCoalesceWindow,
			l.MaxCoalesceWindow,
		)
	}
	return nil
}
