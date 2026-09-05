package config

import (
	"slices"
	"strings"
)

// mergeProviderCuratedModels overlays curated entries by model id. A partial override
// must not delete the curated models it never mentions: settings writes one entry at a
// time, and a wholesale replace would strip every other model's identity and metadata.
func mergeProviderCuratedModels(
	base []ProviderModelConfig,
	override []ProviderModelConfig,
) []ProviderModelConfig {
	if override == nil {
		return cloneProviderModelConfigs(base)
	}
	merged := cloneProviderModelConfigs(base)
	if merged == nil {
		merged = make([]ProviderModelConfig, 0, len(override))
	}
	positions := make(map[string]int, len(merged))
	for index := range merged {
		positions[strings.TrimSpace(merged[index].ID)] = index
	}
	applied := make(map[string]struct{}, len(override))
	for _, model := range override {
		id := strings.TrimSpace(model.ID)
		position, exists := positions[id]
		_, repeated := applied[id]
		// A blank or repeated id is an operator error. Append it verbatim rather than
		// folding it away, so Validate still reports the entry the operator wrote.
		if id == "" || repeated || !exists {
			if id != "" {
				if !repeated {
					positions[id] = len(merged)
				}
				applied[id] = struct{}{}
			}
			merged = append(merged, cloneProviderModelConfig(model))
			continue
		}
		applied[id] = struct{}{}
		merged[position] = mergeProviderModelConfig(merged[position], model)
	}
	return merged
}

func mergeProviderModelConfig(base ProviderModelConfig, override ProviderModelConfig) ProviderModelConfig {
	merged := cloneProviderModelConfig(base)
	mergeProviderModelString(&merged.DisplayName, override.DisplayName)
	mergeProviderModelString(&merged.DefaultReasoningEffort, override.DefaultReasoningEffort)
	mergeProviderModelString(&merged.ReleaseDate, override.ReleaseDate)
	mergeProviderModelString(&merged.DefaultSpeed, override.DefaultSpeed)
	if len(override.ReasoningEfforts) > 0 {
		merged.ReasoningEfforts = cloneStrings(override.ReasoningEfforts)
		// The effort set and its default are one unit: a narrowed set cannot keep an
		// inherited default it can no longer honor.
		if strings.TrimSpace(override.DefaultReasoningEffort) == "" &&
			!slices.Contains(merged.ReasoningEfforts, merged.DefaultReasoningEffort) {
			merged.DefaultReasoningEffort = ""
		}
	}
	mergeProviderModelRef(&merged.ContextWindow, override.ContextWindow)
	mergeProviderModelRef(&merged.MaxInputTokens, override.MaxInputTokens)
	mergeProviderModelRef(&merged.MaxOutputTokens, override.MaxOutputTokens)
	mergeProviderModelRef(&merged.SupportsTools, override.SupportsTools)
	mergeProviderModelRef(&merged.SupportsReasoning, override.SupportsReasoning)
	mergeProviderModelRef(&merged.CostInputPerMillion, override.CostInputPerMillion)
	mergeProviderModelRef(&merged.CostOutputPerMillion, override.CostOutputPerMillion)
	mergeProviderModelRef(&merged.CostCacheReadPerMillion, override.CostCacheReadPerMillion)
	mergeProviderModelRef(&merged.CostCacheWritePerMillion, override.CostCacheWritePerMillion)
	mergeProviderModelRef(&merged.CostReasoningPerMillion, override.CostReasoningPerMillion)
	mergeProviderModelRef(&merged.Deprecated, override.Deprecated)
	mergeProviderModelRef(&merged.Hidden, override.Hidden)
	mergeProviderModelRef(&merged.Featured, override.Featured)
	return merged
}

func mergeProviderModelString[T ~string](target *T, override T) {
	if strings.TrimSpace(string(override)) == "" {
		return
	}
	*target = override
}

func mergeProviderModelRef[T any](target **T, override *T) {
	if override == nil {
		return
	}
	value := *override
	*target = &value
}
