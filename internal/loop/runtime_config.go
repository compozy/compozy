package loop

import (
	"maps"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

func cloneRuntimeDefaults(defaults *RuntimeDefaults) *RuntimeDefaults {
	if defaults == nil {
		return nil
	}
	return &RuntimeDefaults{
		Worker: cloneRuntimeSpec(defaults.Worker),
		Judge:  cloneRuntimeSpec(defaults.Judge),
	}
}

func cloneRuntimeRules(rules []RuntimeRule) []RuntimeRule {
	if rules == nil {
		return nil
	}
	cloned := make([]RuntimeRule, len(rules))
	for index, rule := range rules {
		cloned[index] = RuntimeRule{
			Match: RuntimeMatch{
				ID:         rule.Match.ID,
				Type:       rule.Match.Type,
				Complexity: rule.Match.Complexity,
				Extra:      maps.Clone(rule.Match.Extra),
			},
			Runtime: cloneRuntimeSpec(rule.Runtime),
		}
	}
	return cloned
}

func cloneRuntimeSpec(runtime RuntimeSpec) RuntimeSpec {
	runtime.Extra = maps.Clone(runtime.Extra)
	runtime.ACPOptions = dsl.CloneACPOptionSelections(runtime.ACPOptions)
	return runtime
}

func definitionRuntimeConfig(definition dsl.Definition) (*RuntimeDefaults, []RuntimeRule) {
	var defaults *RuntimeDefaults
	if definition.Contract.RuntimeDefaults != nil {
		defaults = cloneRuntimeDefaults(definition.Contract.RuntimeDefaults)
	}
	return defaults, cloneRuntimeRules(definition.Contract.RuntimeRules)
}

func mergeRuntimeConfigLayer(effective *EffectiveConfig, layer LoopConfig, source string) {
	if layer.RuntimeDefaults != nil {
		mergeRuntimeSpec(
			effective,
			&effective.RuntimeDefaults.Worker,
			layer.RuntimeDefaults.Worker,
			"/runtime_defaults/worker",
			source,
		)
		mergeRuntimeSpec(
			effective,
			&effective.RuntimeDefaults.Judge,
			layer.RuntimeDefaults.Judge,
			"/runtime_defaults/judge",
			source,
		)
	}
	start := len(effective.RuntimeRules)
	effective.RuntimeRules = append(effective.RuntimeRules, cloneRuntimeRules(layer.RuntimeRules)...)
	setRuntimeRuleSources(effective, "/runtime_rules", start, len(layer.RuntimeRules), source)
}

func mergeRuntimeSpec(
	effective *EffectiveConfig,
	target *RuntimeSpec,
	layer RuntimeSpec,
	path string,
	source string,
) {
	if value := strings.TrimSpace(layer.Provider); value != "" {
		target.Provider = value
		setEffectiveConfigSource(effective, path+"/provider", source)
	}
	if value := strings.TrimSpace(layer.Model); value != "" {
		target.Model = value
		setEffectiveConfigSource(effective, path+"/model", source)
	}
	if value := strings.TrimSpace(layer.Reasoning); value != "" {
		target.Reasoning = value
		setEffectiveConfigSource(effective, path+"/reasoning", source)
	}
	if value := strings.TrimSpace(string(layer.Speed)); value != "" {
		target.Speed = layer.Speed
		setEffectiveConfigSource(effective, path+"/speed", source)
	}
	for _, option := range layer.ACPOptions {
		option.ID = strings.TrimSpace(option.ID)
		if option.ID == "" {
			continue
		}
		updated := false
		for index := range target.ACPOptions {
			if strings.TrimSpace(target.ACPOptions[index].ID) != option.ID {
				continue
			}
			target.ACPOptions[index] = option
			updated = true
			break
		}
		if !updated {
			target.ACPOptions = append(target.ACPOptions, option)
		}
		setEffectiveConfigSource(effective, path+"/acp_options/"+option.ID, source)
	}
	target.ACPOptions = dsl.CloneACPOptionSelections(target.ACPOptions)
}
