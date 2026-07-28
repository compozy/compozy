package cli

import "maps"

func loopAndGoalConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"goals.max_turns":                               configSetInt,
		"goals.context_nudge_ratio":                     configSetFloat,
		"loops.defaults.delivery.iteration_cap":         configSetInt,
		"loops.defaults.delivery.no_progress.window":    configSetInt,
		"loops.defaults.delivery.gates.max_revisions":   configSetInt,
		"loops.defaults.delivery.budget.tokens":         configSetInt,
		"loops.defaults.delivery.budget.wall_clock_sec": configSetInt,
		"loops.defaults.delivery.budget.on_exceeded":    configSetString,
		"loops.defaults.delivery.fan_out_width":         configSetInt,
		"loops.defaults.watch.iteration_cap":            configSetInt,
		"loops.defaults.watch.no_progress.window":       configSetInt,
		"loops.defaults.watch.gates.max_revisions":      configSetInt,
		"loops.defaults.watch.budget.tokens":            configSetInt,
		"loops.defaults.watch.budget.wall_clock_sec":    configSetInt,
		"loops.defaults.watch.budget.on_exceeded":       configSetString,
		"loops.defaults.watch.fan_out_width":            configSetInt,
	}
}

func mergeConfigSetValueKinds(
	groups ...map[string]configSetValueKind,
) map[string]configSetValueKind {
	merged := make(map[string]configSetValueKind)
	for _, group := range groups {
		maps.Copy(merged, group)
	}
	return merged
}
