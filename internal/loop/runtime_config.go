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
	return runtime
}

func definitionRuntimeConfig(definition dsl.Definition) (*RuntimeDefaults, []RuntimeRule) {
	var defaults *RuntimeDefaults
	if definition.Contract.RuntimeDefaults != nil {
		defaults = cloneRuntimeDefaults(definition.Contract.RuntimeDefaults)
	}
	return defaults, cloneRuntimeRules(definition.Contract.RuntimeRules)
}

func mergeRuntimeConfigLayer(effective *EffectiveConfig, layer LoopConfig) {
	if layer.RuntimeDefaults != nil {
		mergeRuntimeSpec(&effective.RuntimeDefaults.Worker, layer.RuntimeDefaults.Worker)
		mergeRuntimeSpec(&effective.RuntimeDefaults.Judge, layer.RuntimeDefaults.Judge)
	}
	effective.RuntimeRules = append(effective.RuntimeRules, cloneRuntimeRules(layer.RuntimeRules)...)
}

func mergeRuntimeSpec(target *RuntimeSpec, layer RuntimeSpec) {
	if value := strings.TrimSpace(layer.Provider); value != "" {
		target.Provider = value
	}
	if value := strings.TrimSpace(layer.Model); value != "" {
		target.Model = value
	}
	if value := strings.TrimSpace(layer.Reasoning); value != "" {
		target.Reasoning = value
	}
}
