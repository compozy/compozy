package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	settingspkg "github.com/compozy/agh/internal/settings"
)

func settingsNetworkConfigPayload(value aghconfig.NetworkConfig) contract.SettingsNetworkConfigPayload {
	return contract.SettingsNetworkConfigPayload{
		Enabled:      value.Enabled,
		MaxReplayAge: value.MaxReplayAge,
		Live: contract.SettingsNetworkLiveConfigPayload{
			Defaults: contract.SettingsNetworkLiveDefaultsPayload{
				MaxWakes:         value.Live.Defaults.MaxWakes,
				MaxWakeWallTime:  value.Live.Defaults.MaxWakeWallTime,
				MaxTotalWallTime: value.Live.Defaults.MaxTotalWallTime,
				MaxInputTokens:   value.Live.Defaults.MaxInputTokens,
				MaxOutputTokens:  value.Live.Defaults.MaxOutputTokens,
				MaxWakeDepth:     value.Live.Defaults.MaxWakeDepth,
				CoalesceWindow:   value.Live.Defaults.CoalesceWindow,
			},
			Limits: contract.SettingsNetworkLiveLimitsPayload{
				MaxWakes:          value.Live.Limits.MaxWakes,
				MaxWakeWallTime:   value.Live.Limits.MaxWakeWallTime,
				MaxTotalWallTime:  value.Live.Limits.MaxTotalWallTime,
				MaxInputTokens:    value.Live.Limits.MaxInputTokens,
				MaxOutputTokens:   value.Live.Limits.MaxOutputTokens,
				MaxWakeDepth:      value.Live.Limits.MaxWakeDepth,
				MinCoalesceWindow: value.Live.Limits.MinCoalesceWindow,
				MaxCoalesceWindow: value.Live.Limits.MaxCoalesceWindow,
			},
		},
	}
}

func settingsNetworkRuntimePayload(value settingspkg.NetworkRuntimeStatus) contract.SettingsNetworkRuntimePayload {
	return contract.SettingsNetworkRuntimePayload{
		Available:         value.Available,
		Enabled:           value.Enabled,
		Status:            strings.TrimSpace(value.Status),
		LocalPeers:        value.LocalPeers,
		Channels:          value.Channels,
		MessagesReceived:  value.MessagesReceived,
		MessagesDelivered: value.MessagesDelivered,
		MessagesRejected:  value.MessagesRejected,
	}
}

func networkConfigFromPayload(payload contract.SettingsNetworkConfigPayload) (aghconfig.NetworkConfig, error) {
	value := aghconfig.DefaultNetworkConfig()
	value.Enabled = payload.Enabled
	value.MaxReplayAge = payload.MaxReplayAge
	value.Live = aghconfig.NetworkLiveConfig{
		Defaults: aghconfig.NetworkLiveDefaultsConfig{
			MaxWakes:         payload.Live.Defaults.MaxWakes,
			MaxWakeWallTime:  strings.TrimSpace(payload.Live.Defaults.MaxWakeWallTime),
			MaxTotalWallTime: strings.TrimSpace(payload.Live.Defaults.MaxTotalWallTime),
			MaxInputTokens:   payload.Live.Defaults.MaxInputTokens,
			MaxOutputTokens:  payload.Live.Defaults.MaxOutputTokens,
			MaxWakeDepth:     payload.Live.Defaults.MaxWakeDepth,
			CoalesceWindow:   strings.TrimSpace(payload.Live.Defaults.CoalesceWindow),
		},
		Limits: aghconfig.NetworkLiveLimitsConfig{
			MaxWakes:          payload.Live.Limits.MaxWakes,
			MaxWakeWallTime:   strings.TrimSpace(payload.Live.Limits.MaxWakeWallTime),
			MaxTotalWallTime:  strings.TrimSpace(payload.Live.Limits.MaxTotalWallTime),
			MaxInputTokens:    payload.Live.Limits.MaxInputTokens,
			MaxOutputTokens:   payload.Live.Limits.MaxOutputTokens,
			MaxWakeDepth:      payload.Live.Limits.MaxWakeDepth,
			MinCoalesceWindow: strings.TrimSpace(payload.Live.Limits.MinCoalesceWindow),
			MaxCoalesceWindow: strings.TrimSpace(payload.Live.Limits.MaxCoalesceWindow),
		},
	}
	if err := value.Validate(); err != nil {
		return aghconfig.NetworkConfig{}, NewSettingsValidationError(err)
	}
	return value, nil
}
