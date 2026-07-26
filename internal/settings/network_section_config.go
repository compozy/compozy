package settings

import aghconfig "github.com/compozy/agh/internal/config"

func diffNetworkSettings(current aghconfig.NetworkConfig, desired aghconfig.NetworkConfig) []string {
	changed := make([]string, 0, 20)
	appendNetworkChange := func(path string, differs bool) {
		if differs {
			changed = append(changed, path)
		}
	}

	appendNetworkChange("network.enabled", current.Enabled != desired.Enabled)
	appendNetworkChange("network.max_replay_age", current.MaxReplayAge != desired.MaxReplayAge)
	appendNetworkChange(
		"network.live.defaults.max_wakes",
		current.Live.Defaults.MaxWakes != desired.Live.Defaults.MaxWakes,
	)
	appendNetworkChange(
		"network.live.defaults.max_wake_wall_time",
		current.Live.Defaults.MaxWakeWallTime != desired.Live.Defaults.MaxWakeWallTime,
	)
	appendNetworkChange(
		"network.live.defaults.max_total_wall_time",
		current.Live.Defaults.MaxTotalWallTime != desired.Live.Defaults.MaxTotalWallTime,
	)
	appendNetworkChange(
		"network.live.defaults.max_input_tokens",
		current.Live.Defaults.MaxInputTokens != desired.Live.Defaults.MaxInputTokens,
	)
	appendNetworkChange(
		"network.live.defaults.max_output_tokens",
		current.Live.Defaults.MaxOutputTokens != desired.Live.Defaults.MaxOutputTokens,
	)
	appendNetworkChange(
		"network.live.defaults.max_wake_depth",
		current.Live.Defaults.MaxWakeDepth != desired.Live.Defaults.MaxWakeDepth,
	)
	appendNetworkChange(
		"network.live.defaults.coalesce_window",
		current.Live.Defaults.CoalesceWindow != desired.Live.Defaults.CoalesceWindow,
	)
	appendNetworkChange(
		"network.live.limits.max_wakes",
		current.Live.Limits.MaxWakes != desired.Live.Limits.MaxWakes,
	)
	appendNetworkChange(
		"network.live.limits.max_wake_wall_time",
		current.Live.Limits.MaxWakeWallTime != desired.Live.Limits.MaxWakeWallTime,
	)
	appendNetworkChange(
		"network.live.limits.max_total_wall_time",
		current.Live.Limits.MaxTotalWallTime != desired.Live.Limits.MaxTotalWallTime,
	)
	appendNetworkChange(
		"network.live.limits.max_input_tokens",
		current.Live.Limits.MaxInputTokens != desired.Live.Limits.MaxInputTokens,
	)
	appendNetworkChange(
		"network.live.limits.max_output_tokens",
		current.Live.Limits.MaxOutputTokens != desired.Live.Limits.MaxOutputTokens,
	)
	appendNetworkChange(
		"network.live.limits.max_wake_depth",
		current.Live.Limits.MaxWakeDepth != desired.Live.Limits.MaxWakeDepth,
	)
	appendNetworkChange(
		"network.live.limits.min_coalesce_window",
		current.Live.Limits.MinCoalesceWindow != desired.Live.Limits.MinCoalesceWindow,
	)
	appendNetworkChange(
		"network.live.limits.max_coalesce_window",
		current.Live.Limits.MaxCoalesceWindow != desired.Live.Limits.MaxCoalesceWindow,
	)
	return changed
}

func applyNetworkSettings(editor *aghconfig.OverlayEditor, settings aghconfig.NetworkConfig) error {
	networkPath := func(parts ...string) []string {
		return append([]string{string(SectionNetwork)}, parts...)
	}
	updates := []struct {
		path  []string
		value any
	}{
		{path: networkPath(sectionsEnabledKey), value: settings.Enabled},
		{path: networkPath("max_replay_age"), value: settings.MaxReplayAge},
		{path: networkPath("live", "defaults", "max_wakes"), value: settings.Live.Defaults.MaxWakes},
		{path: networkPath("live", "defaults", "max_wake_wall_time"), value: settings.Live.Defaults.MaxWakeWallTime},
		{path: networkPath("live", "defaults", "max_total_wall_time"), value: settings.Live.Defaults.MaxTotalWallTime},
		{path: networkPath("live", "defaults", "max_input_tokens"), value: settings.Live.Defaults.MaxInputTokens},
		{path: networkPath("live", "defaults", "max_output_tokens"), value: settings.Live.Defaults.MaxOutputTokens},
		{path: networkPath("live", "defaults", "max_wake_depth"), value: settings.Live.Defaults.MaxWakeDepth},
		{path: networkPath("live", "defaults", "coalesce_window"), value: settings.Live.Defaults.CoalesceWindow},
		{path: networkPath("live", "limits", "max_wakes"), value: settings.Live.Limits.MaxWakes},
		{path: networkPath("live", "limits", "max_wake_wall_time"), value: settings.Live.Limits.MaxWakeWallTime},
		{path: networkPath("live", "limits", "max_total_wall_time"), value: settings.Live.Limits.MaxTotalWallTime},
		{path: networkPath("live", "limits", "max_input_tokens"), value: settings.Live.Limits.MaxInputTokens},
		{path: networkPath("live", "limits", "max_output_tokens"), value: settings.Live.Limits.MaxOutputTokens},
		{path: networkPath("live", "limits", "max_wake_depth"), value: settings.Live.Limits.MaxWakeDepth},
		{path: networkPath("live", "limits", "min_coalesce_window"), value: settings.Live.Limits.MinCoalesceWindow},
		{path: networkPath("live", "limits", "max_coalesce_window"), value: settings.Live.Limits.MaxCoalesceWindow},
	}
	return applyValueUpdates(editor, updates)
}
