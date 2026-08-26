package loop

import "strconv"

const (
	EffectiveConfigSourceBuiltin              = "builtin"
	EffectiveConfigSourceDefinition           = "definition"
	EffectiveConfigSourceDeliveryDefaults     = "loops.defaults.delivery"
	EffectiveConfigSourceWatchDefaults        = "loops.defaults.watch"
	EffectiveConfigSourceInheritedEnvironment = "inherited_environment"
	EffectiveConfigSourceLoopConfig           = "loop_config"
	EffectiveConfigSourcePerRun               = "per_run"
)

func builtinEffectiveConfigSources() map[string]string {
	paths := []string{
		"/human_gate_enabled",
		"/reattempt_strategy",
		"/enabled_checks_json",
		"/iteration_cap",
		"/budget_tokens",
		"/budget_wall_sec",
		"/budget_on_exceeded",
		"/no_progress_window",
		"/fan_out_width",
		"/gate_max_revisions",
		"/runtime_defaults/worker/provider",
		"/runtime_defaults/worker/model",
		"/runtime_defaults/worker/reasoning",
		"/runtime_defaults/worker/speed",
		"/runtime_defaults/judge/provider",
		"/runtime_defaults/judge/model",
		"/runtime_defaults/judge/reasoning",
		"/runtime_defaults/judge/speed",
		"/environment",
		"/request_expire_after",
	}
	sources := make(map[string]string, len(paths))
	for _, path := range paths {
		sources[path] = EffectiveConfigSourceBuiltin
	}
	return sources
}

func setEffectiveConfigSource(effective *EffectiveConfig, path string, source string) {
	if effective.Sources == nil {
		effective.Sources = make(map[string]string)
	}
	effective.Sources[path] = source
}

func setRuntimeRuleSources(effective *EffectiveConfig, path string, start int, count int, source string) {
	for index := range count {
		setEffectiveConfigSource(effective, path+"/"+strconv.Itoa(start+index), source)
	}
}
