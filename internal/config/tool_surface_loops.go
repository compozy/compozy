package config

import "maps"

func loopAndGoalToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		goalMaxTurnsPath:                                ConfigValueInt,
		goalContextNudgeRatioPath:                       ConfigValueFloat,
		goalOutboxBatchSizePath:                         ConfigValueInt,
		goalOutboxPollIntervalPath:                      ConfigValueDuration,
		"loops.defaults.delivery.iteration_cap":         ConfigValueInt,
		"loops.defaults.delivery.no_progress.window":    ConfigValueInt,
		"loops.defaults.delivery.gates.max_revisions":   ConfigValueInt,
		"loops.defaults.delivery.budget.tokens":         ConfigValueInt,
		"loops.defaults.delivery.budget.wall_clock_sec": ConfigValueInt,
		"loops.defaults.delivery.budget.on_exceeded":    ConfigValueString,
		"loops.defaults.delivery.model_defaults.worker": ConfigValueString,
		"loops.defaults.delivery.model_defaults.judge":  ConfigValueString,
		"loops.defaults.delivery.fan_out_width":         ConfigValueInt,
		"loops.defaults.watch.iteration_cap":            ConfigValueInt,
		"loops.defaults.watch.no_progress.window":       ConfigValueInt,
		"loops.defaults.watch.gates.max_revisions":      ConfigValueInt,
		"loops.defaults.watch.budget.tokens":            ConfigValueInt,
		"loops.defaults.watch.budget.wall_clock_sec":    ConfigValueInt,
		"loops.defaults.watch.budget.on_exceeded":       ConfigValueString,
		"loops.defaults.watch.model_defaults.worker":    ConfigValueString,
		"loops.defaults.watch.model_defaults.judge":     ConfigValueString,
		"loops.defaults.watch.fan_out_width":            ConfigValueInt,
	}
}

func mergeAgentMutableConfigKinds(
	base map[string]ValueKind,
	overlays ...map[string]ValueKind,
) map[string]ValueKind {
	merged := maps.Clone(base)
	for _, overlay := range overlays {
		maps.Copy(merged, overlay)
	}
	return merged
}
