package config

import (
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// RuntimeValueSource identifies the authored/default layer that supplied one
// effective runtime value.
type RuntimeValueSource uint8

const (
	// RuntimeValueSourceUnspecified identifies an unresolved runtime value.
	RuntimeValueSourceUnspecified RuntimeValueSource = iota
	// RuntimeValueSourceAgent identifies a value authored in AGENT.md.
	RuntimeValueSourceAgent
	// RuntimeValueSourceProjectDefault identifies defaults.provider.
	RuntimeValueSourceProjectDefault
	// RuntimeValueSourceProviderDefault identifies provider.models.default.
	RuntimeValueSourceProviderDefault
	// RuntimeValueSourceModelDefault identifies a curated model runtime default.
	RuntimeValueSourceModelDefault
	// RuntimeValueSourceConfigOverride identifies an operator [agents.overrides.<name>] value.
	RuntimeValueSourceConfigOverride
)

// String returns the public runtime-source value.
func (s RuntimeValueSource) String() string {
	switch s {
	case RuntimeValueSourceAgent:
		return "agent"
	case RuntimeValueSourceProjectDefault:
		return "project_default"
	case RuntimeValueSourceProviderDefault:
		return "provider_default"
	case RuntimeValueSourceModelDefault:
		return "model_default"
	case RuntimeValueSourceConfigOverride:
		return "config_override"
	case RuntimeValueSourceUnspecified:
		return ""
	default:
		return ""
	}
}

// ResolvedRuntimeSources records the provenance of one effective agent runtime.
type ResolvedRuntimeSources struct {
	Provider        RuntimeValueSource
	Model           RuntimeValueSource
	ReasoningEffort RuntimeValueSource
	Speed           RuntimeValueSource
	Permissions     RuntimeValueSource
}

func defaultSpeedForModel(provider ProviderConfig, modelID string) speedpkg.Speed {
	target := strings.TrimSpace(modelID)
	if target == "" {
		return ""
	}
	for _, model := range provider.Models.Curated {
		if strings.TrimSpace(model.ID) == target {
			return model.DefaultSpeed
		}
	}
	return ""
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

// permissionsSource attributes an effective level to AGENT.md or to [permissions].mode.
func permissionsSource(authored string) RuntimeValueSource {
	if strings.TrimSpace(authored) != "" {
		return RuntimeValueSourceAgent
	}
	return RuntimeValueSourceProjectDefault
}
