package cli

func networkConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"network.enabled":                           configSetBool,
		"network.max_replay_age":                    configSetInt,
		"network.live.defaults.max_wakes":           configSetInt,
		"network.live.defaults.max_wake_wall_time":  configSetString,
		"network.live.defaults.max_total_wall_time": configSetString,
		"network.live.defaults.max_input_tokens":    configSetInt64,
		"network.live.defaults.max_output_tokens":   configSetInt64,
		"network.live.defaults.max_wake_depth":      configSetInt,
		"network.live.defaults.coalesce_window":     configSetString,
		"network.live.limits.max_wakes":             configSetInt,
		"network.live.limits.max_wake_wall_time":    configSetString,
		"network.live.limits.max_total_wall_time":   configSetString,
		"network.live.limits.max_input_tokens":      configSetInt64,
		"network.live.limits.max_output_tokens":     configSetInt64,
		"network.live.limits.max_wake_depth":        configSetInt,
		"network.live.limits.min_coalesce_window":   configSetString,
		"network.live.limits.max_coalesce_window":   configSetString,
	}
}
