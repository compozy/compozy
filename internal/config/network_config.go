package config

import (
	"fmt"
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

const maxNetworkDurationSeconds = int64(1<<63-1) / int64(time.Second)

const networkLiveLimitsMaxWakesPath = "network.live.limits.max_wakes"
const networkLiveLimitsMinCoalesceWindowPath = "network.live.limits.min_coalesce_window"
const defaultNetworkLiveTotalWallTime = "30m"

// NetworkConfig controls Network availability and bounded Live participation defaults.
type NetworkConfig struct {
	Enabled      bool              `toml:"enabled"`
	MaxReplayAge int               `toml:"max_replay_age"`
	Live         NetworkLiveConfig `toml:"live"`
}

// NetworkLiveConfig groups bounds that apply only after an execution explicitly selects Live mode.
type NetworkLiveConfig struct {
	Defaults NetworkLiveDefaultsConfig `toml:"defaults"`
	Limits   NetworkLiveLimitsConfig   `toml:"limits"`
}

// NetworkLiveDefaultsConfig fills omitted bounds for an explicitly Live execution.
type NetworkLiveDefaultsConfig struct {
	MaxWakes         int    `toml:"max_wakes"`
	MaxWakeWallTime  string `toml:"max_wake_wall_time"`
	MaxTotalWallTime string `toml:"max_total_wall_time"`
	MaxInputTokens   int64  `toml:"max_input_tokens"`
	MaxOutputTokens  int64  `toml:"max_output_tokens"`
	MaxWakeDepth     int    `toml:"max_wake_depth"`
	CoalesceWindow   string `toml:"coalesce_window"`
}

// NetworkLiveLimitsConfig defines inclusive administrative ceilings for Live bounds.
type NetworkLiveLimitsConfig struct {
	MaxWakes          int    `toml:"max_wakes"`
	MaxWakeWallTime   string `toml:"max_wake_wall_time"`
	MaxTotalWallTime  string `toml:"max_total_wall_time"`
	MaxInputTokens    int64  `toml:"max_input_tokens"`
	MaxOutputTokens   int64  `toml:"max_output_tokens"`
	MaxWakeDepth      int    `toml:"max_wake_depth"`
	MinCoalesceWindow string `toml:"min_coalesce_window"`
	MaxCoalesceWindow string `toml:"max_coalesce_window"`
}

// DefaultNetworkConfig returns built-in Network availability and Live-bound defaults.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		Enabled:      true,
		MaxReplayAge: 300,
		Live: NetworkLiveConfig{
			Defaults: NetworkLiveDefaultsConfig{
				MaxWakes:         8,
				MaxWakeWallTime:  "5m",
				MaxTotalWallTime: defaultNetworkLiveTotalWallTime,
				MaxInputTokens:   200_000,
				MaxOutputTokens:  50_000,
				MaxWakeDepth:     3,
				CoalesceWindow:   "500ms",
			},
			Limits: NetworkLiveLimitsConfig{
				MaxWakes:          64,
				MaxWakeWallTime:   "15m",
				MaxTotalWallTime:  "2h",
				MaxInputTokens:    1_000_000,
				MaxOutputTokens:   200_000,
				MaxWakeDepth:      5,
				MinCoalesceWindow: "100ms",
				MaxCoalesceWindow: "5s",
			},
		},
	}
}

// Validate ensures availability and Live-bound configuration is internally consistent.
func (c NetworkConfig) Validate() error {
	if c.MaxReplayAge <= 0 || int64(c.MaxReplayAge) > maxNetworkDurationSeconds {
		return fmt.Errorf(
			"network.max_replay_age must be between 1 and %d seconds: %d",
			maxNetworkDurationSeconds,
			c.MaxReplayAge,
		)
	}
	limits := c.Live.Limits.ParticipationLimits()
	if err := limits.Validate(); err != nil {
		return fmt.Errorf("network.live.limits: %w", err)
	}
	if _, err := participation.ResolveBounds(nil, c.Live.Defaults.ParticipationBounds(), limits); err != nil {
		return fmt.Errorf("network.live.defaults: %w", err)
	}
	return nil
}

// ParticipationBounds converts configured Live defaults into the shared participation contract.
func (c NetworkLiveDefaultsConfig) ParticipationBounds() participation.Bounds {
	return participation.Bounds{
		MaxWakes:         c.MaxWakes,
		MaxWakeWallTime:  c.MaxWakeWallTime,
		MaxTotalWallTime: c.MaxTotalWallTime,
		MaxInputTokens:   c.MaxInputTokens,
		MaxOutputTokens:  c.MaxOutputTokens,
		MaxWakeDepth:     c.MaxWakeDepth,
		CoalesceWindow:   c.CoalesceWindow,
	}
}

// ParticipationLimits converts configured Live ceilings into the shared participation contract.
func (c NetworkLiveLimitsConfig) ParticipationLimits() participation.Limits {
	return participation.Limits{
		MaxWakes:          c.MaxWakes,
		MaxWakeWallTime:   c.MaxWakeWallTime,
		MaxTotalWallTime:  c.MaxTotalWallTime,
		MaxInputTokens:    c.MaxInputTokens,
		MaxOutputTokens:   c.MaxOutputTokens,
		MaxWakeDepth:      c.MaxWakeDepth,
		MinCoalesceWindow: c.MinCoalesceWindow,
		MaxCoalesceWindow: c.MaxCoalesceWindow,
	}
}

// MaxReplayAgeDuration returns the configured replay age window as a duration.
func (c NetworkConfig) MaxReplayAgeDuration() time.Duration {
	return time.Duration(c.MaxReplayAge) * time.Second
}
