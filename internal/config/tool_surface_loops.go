package config

import "maps"

const (
	loopDeliveryRetryMaxAttemptsPath  = "loops.defaults.delivery.retry.max_attempts"
	loopDeliveryAdmissionIntervalPath = "loops.defaults.delivery.waits.admission_retry_interval"
	// #nosec G101 -- This is a public configuration path, not a credential.
	loopWatchRetryBackoffBasePath = "loops.defaults.watch.retry.backoff_base"
	loopBreakerThresholdPath      = "loops.breaker.threshold"
)

func loopAndGoalToolPathKinds() map[string]ValueKind {
	return map[string]ValueKind{
		"loops.reconcile_interval":                                  ConfigValueDuration,
		goalMaxTurnsPath:                                            ConfigValueInt,
		goalContextNudgeRatioPath:                                   ConfigValueFloat,
		goalOutboxBatchSizePath:                                     ConfigValueInt,
		goalOutboxPollIntervalPath:                                  ConfigValueDuration,
		"loops.defaults.delivery.iteration_cap":                     ConfigValueInt,
		"loops.defaults.delivery.no_progress.window":                ConfigValueInt,
		"loops.defaults.delivery.gates.max_revisions":               ConfigValueInt,
		"loops.defaults.delivery.budget.tokens":                     ConfigValueInt,
		"loops.defaults.delivery.budget.wall_clock_sec":             ConfigValueInt,
		"loops.defaults.delivery.budget.on_exceeded":                ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.worker.provider":  ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.worker.model":     ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.worker.reasoning": ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.worker.speed":     ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.judge.provider":   ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.judge.model":      ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.judge.reasoning":  ConfigValueString,
		"loops.defaults.delivery.runtime_defaults.judge.speed":      ConfigValueString,
		"loops.defaults.delivery.fan_out_width":                     ConfigValueInt,
		"loops.defaults.delivery.requests.expire_after":             ConfigValueString,
		loopDeliveryRetryMaxAttemptsPath:                            ConfigValueInt,
		"loops.defaults.delivery.retry.backoff_base":                ConfigValueDuration,
		"loops.defaults.delivery.retry.backoff_max":                 ConfigValueDuration,
		"loops.defaults.delivery.liveness.silence_window":           ConfigValueDuration,
		"loops.defaults.delivery.resume.death_streak_limit":         ConfigValueInt,
		"loops.defaults.delivery.predicates.cost_limit":             ConfigValueUint64,
		"loops.defaults.delivery.waits.admission_attempts":          ConfigValueInt,
		loopDeliveryAdmissionIntervalPath:                           ConfigValueDuration,
		"loops.defaults.delivery.admission.tombstone_horizon":       ConfigValueDuration,
		"loops.defaults.watch.iteration_cap":                        ConfigValueInt,
		"loops.defaults.watch.no_progress.window":                   ConfigValueInt,
		"loops.defaults.watch.gates.max_revisions":                  ConfigValueInt,
		"loops.defaults.watch.budget.tokens":                        ConfigValueInt,
		"loops.defaults.watch.budget.wall_clock_sec":                ConfigValueInt,
		"loops.defaults.watch.budget.on_exceeded":                   ConfigValueString,
		"loops.defaults.watch.runtime_defaults.worker.provider":     ConfigValueString,
		"loops.defaults.watch.runtime_defaults.worker.model":        ConfigValueString,
		"loops.defaults.watch.runtime_defaults.worker.reasoning":    ConfigValueString,
		"loops.defaults.watch.runtime_defaults.worker.speed":        ConfigValueString,
		"loops.defaults.watch.runtime_defaults.judge.provider":      ConfigValueString,
		"loops.defaults.watch.runtime_defaults.judge.model":         ConfigValueString,
		"loops.defaults.watch.runtime_defaults.judge.reasoning":     ConfigValueString,
		"loops.defaults.watch.runtime_defaults.judge.speed":         ConfigValueString,
		"loops.defaults.watch.fan_out_width":                        ConfigValueInt,
		"loops.defaults.watch.requests.expire_after":                ConfigValueString,
		"loops.defaults.watch.retry.max_attempts":                   ConfigValueInt,
		loopWatchRetryBackoffBasePath:                               ConfigValueDuration,
		"loops.defaults.watch.retry.backoff_max":                    ConfigValueDuration,
		"loops.defaults.watch.liveness.silence_window":              ConfigValueDuration,
		"loops.defaults.watch.resume.death_streak_limit":            ConfigValueInt,
		"loops.defaults.watch.predicates.cost_limit":                ConfigValueUint64,
		"loops.defaults.watch.waits.admission_attempts":             ConfigValueInt,
		"loops.defaults.watch.waits.admission_retry_interval":       ConfigValueDuration,
		"loops.defaults.watch.admission.tombstone_horizon":          ConfigValueDuration,
		loopBreakerThresholdPath:                                    ConfigValueInt,
		"loops.breaker.probe_interval":                              ConfigValueDuration,
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
