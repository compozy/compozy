package config

import "strings"

// RuntimeValueSource identifies the authored/default layer that supplied one
// effective runtime value.
type RuntimeValueSource string

const (
	// RuntimeValueSourceAgent identifies a value authored in AGENT.md.
	RuntimeValueSourceAgent RuntimeValueSource = "agent"
	// RuntimeValueSourceProjectDefault identifies defaults.provider.
	RuntimeValueSourceProjectDefault RuntimeValueSource = "project_default"
	// RuntimeValueSourceProviderDefault identifies provider.models.default.
	RuntimeValueSourceProviderDefault RuntimeValueSource = "provider_default"
	// RuntimeValueSourceModelDefault identifies a curated model's default_reasoning_effort.
	RuntimeValueSourceModelDefault RuntimeValueSource = "model_default"
)

// ResolvedRuntimeSources records the provenance of one effective agent runtime.
type ResolvedRuntimeSources struct {
	Provider        RuntimeValueSource
	Model           RuntimeValueSource
	ReasoningEffort RuntimeValueSource
}

func defaultReasoningEffortForModel(provider ProviderConfig, modelID string) string {
	target := strings.TrimSpace(modelID)
	if target == "" {
		return ""
	}
	for _, model := range provider.Models.Curated {
		if strings.TrimSpace(model.ID) == target {
			return strings.TrimSpace(model.DefaultReasoningEffort)
		}
	}
	return ""
}
