package daemon

import (
	"github.com/compozy/compozy/internal/api/contract"
)

type compactModel struct {
	ProviderID             string                                       `json:"provider_id"`
	ModelID                string                                       `json:"model_id"`
	DisplayName            string                                       `json:"display_name,omitempty"`
	Available              *bool                                        `json:"available"`
	AvailabilityState      string                                       `json:"availability_state"`
	Stale                  bool                                         `json:"stale"`
	SupportsTools          *bool                                        `json:"supports_tools,omitempty"`
	SupportsReasoning      *bool                                        `json:"supports_reasoning,omitempty"`
	ReasoningEfforts       []contract.ReasoningEffort                   `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort *contract.ReasoningEffort                    `json:"default_reasoning_effort,omitempty"`
	Configurations         []contract.ProviderModelConfigurationPayload `json:"configurations,omitempty"`
	Curated                bool                                         `json:"curated"`
	Deprecated             bool                                         `json:"deprecated"`
	Hidden                 bool                                         `json:"hidden"`
	Featured               bool                                         `json:"featured"`
	LastError              string                                       `json:"last_error,omitempty"`
}

func projectCompactModel(p contract.ProviderModelPayload) compactModel {
	return compactModel{
		ProviderID:             p.ProviderID,
		ModelID:                p.ModelID,
		DisplayName:            p.DisplayName,
		Available:              p.Available,
		AvailabilityState:      p.AvailabilityState,
		Stale:                  p.Stale,
		SupportsTools:          p.SupportsTools,
		SupportsReasoning:      p.SupportsReasoning,
		ReasoningEfforts:       p.ReasoningEfforts,
		DefaultReasoningEffort: p.DefaultReasoningEffort,
		Configurations:         p.Configurations,
		Curated:                p.Curated,
		Deprecated:             p.Deprecated,
		Hidden:                 p.Hidden,
		Featured:               p.Featured,
		LastError:              p.LastError,
	}
}

type compactApply struct {
	Applied         bool                                   `json:"applied"`
	Lifecycle       contract.SettingsApplyLifecycle        `json:"lifecycle"`
	NextAction      contract.SettingsApplyNextAction       `json:"next_action"`
	RestartRequired bool                                   `json:"restart_required,omitzero"`
	RestartScope    string                                 `json:"restart_scope,omitempty"`
	Warnings        []string                               `json:"warnings,omitempty"`
	PartialFailures []contract.SettingsApplyFailurePayload `json:"partial_failures,omitempty"`
	Skipped         bool                                   `json:"skipped,omitzero"`
	SkippedReason   string                                 `json:"skipped_reason,omitempty"`
}

func projectCompactApply(p contract.SettingsApplyResponse) compactApply {
	return compactApply{
		Applied:         p.Applied,
		Lifecycle:       p.Lifecycle,
		NextAction:      p.NextAction,
		RestartRequired: p.RestartRequired,
		RestartScope:    p.RestartScope,
		Warnings:        p.Warnings,
		PartialFailures: p.PartialFailures,
		Skipped:         p.Skipped,
		SkippedReason:   p.SkippedReason,
	}
}

func compactProviderModels(p contract.ProviderModelListResponse) any {
	var models []compactModel
	if p.Models != nil {
		models = make([]compactModel, len(p.Models))
		for i := range p.Models {
			models[i] = projectCompactModel(p.Models[i])
		}
	}
	return map[string]any{"models": models}
}
func compactProviderCuration(p *contract.ProviderModelCurationResponse) any {
	return struct {
		Model        compactModel   `json:"model"`
		Apply        compactApply   `json:"apply"`
		DefaultSpeed contract.Speed `json:"default_speed,omitempty"`
	}{projectCompactModel(p.Model), projectCompactApply(p.Apply), p.DefaultSpeed}
}
