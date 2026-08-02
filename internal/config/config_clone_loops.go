package config

import "github.com/compozy/compozy/internal/loop/dsl"

func cloneLoopsConfig(source LoopsConfig) LoopsConfig {
	cloned := source
	cloned.Defaults.Delivery = cloneLoopDefaultConfig(source.Defaults.Delivery)
	cloned.Defaults.Watch = cloneLoopDefaultConfig(source.Defaults.Watch)
	cloned.Inputs = cloneLoopInputDefaults(source.Inputs)
	cloned.inputSources = cloneLoopInputSources(source.inputSources)
	return cloned
}

func cloneLoopDefaultConfig(source LoopDefaultConfig) LoopDefaultConfig {
	cloned := source
	cloned.RuntimeDefaults.Worker = cloneRuntimeSpec(source.RuntimeDefaults.Worker)
	cloned.RuntimeDefaults.Judge = cloneRuntimeSpec(source.RuntimeDefaults.Judge)
	if source.RuntimeRules != nil {
		cloned.RuntimeRules = make([]dsl.RuntimeRule, len(source.RuntimeRules))
		for index, rule := range source.RuntimeRules {
			cloned.RuntimeRules[index] = rule
			cloned.RuntimeRules[index].Match.Extra = cloneConfigAnyMap(rule.Match.Extra)
			cloned.RuntimeRules[index].Runtime = cloneRuntimeSpec(rule.Runtime)
		}
	}
	return cloned
}

func cloneRuntimeSpec(source dsl.RuntimeSpec) dsl.RuntimeSpec {
	cloned := source
	cloned.Extra = cloneConfigAnyMap(source.Extra)
	return cloned
}

func cloneLoopInputDefaults(source LoopInputDefaults) LoopInputDefaults {
	if source == nil {
		return nil
	}
	cloned := make(LoopInputDefaults, len(source))
	for loopName, values := range source {
		cloned[loopName] = cloneConfigAnyMap(values)
	}
	return cloned
}

func cloneLoopInputSources(source LoopInputDefaultSources) LoopInputDefaultSources {
	if source == nil {
		return nil
	}
	cloned := make(LoopInputDefaultSources, len(source))
	for loopName, values := range source {
		cloned[loopName] = mergeStringMaps(nil, values)
	}
	return cloned
}

func cloneConfigAnyMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = cloneConfigAnyValue(value)
	}
	return cloned
}

func cloneConfigAnyValue(source any) any {
	switch value := source.(type) {
	case map[string]any:
		return cloneConfigAnyMap(value)
	case []any:
		cloned := make([]any, len(value))
		for index, item := range value {
			cloned[index] = cloneConfigAnyValue(item)
		}
		return cloned
	case map[string]string:
		return mergeStringMaps(nil, value)
	case []string:
		return cloneStrings(value)
	case []byte:
		return append([]byte(nil), value...)
	case []map[string]any:
		cloned := make([]map[string]any, len(value))
		for index, item := range value {
			cloned[index] = cloneConfigAnyMap(item)
		}
		return cloned
	case []map[string]string:
		cloned := make([]map[string]string, len(value))
		for index, item := range value {
			cloned[index] = mergeStringMaps(nil, item)
		}
		return cloned
	default:
		return source
	}
}
