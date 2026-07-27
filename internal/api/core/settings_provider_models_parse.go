package core

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func providerModelsFromPayload(payload *contract.SettingsProviderModelsPayload) compozyconfig.ProviderModelsConfig {
	if payload == nil {
		return compozyconfig.ProviderModelsConfig{}
	}
	models := compozyconfig.ProviderModelsConfig{
		Default:   strings.TrimSpace(payload.Default),
		Curated:   providerModelConfigsFromPayload(payload.Curated),
		Discovery: providerModelsDiscoveryFromPayload(payload.Discovery),
	}
	if payload.Reasoning != nil {
		models.Reasoning.Apply = compozyconfig.ReasoningApplyStrategy(strings.TrimSpace(payload.Reasoning.Apply))
	}
	return models
}

func providerModelsDiscoveryFromPayload(
	payload *contract.SettingsProviderModelsDiscoveryPayload,
) compozyconfig.ProviderModelsDiscoveryConfig {
	if payload == nil {
		return compozyconfig.ProviderModelsDiscoveryConfig{}
	}
	return compozyconfig.ProviderModelsDiscoveryConfig{
		Enabled:  cloneBoolPtr(payload.Enabled),
		Command:  strings.TrimSpace(payload.Command),
		Endpoint: strings.TrimSpace(payload.Endpoint),
		Timeout:  strings.TrimSpace(payload.Timeout),
	}
}

func providerModelConfigsFromPayload(
	payloads []contract.SettingsProviderModelPayload,
) []compozyconfig.ProviderModelConfig {
	if payloads == nil {
		return nil
	}
	models := make([]compozyconfig.ProviderModelConfig, 0, len(payloads))
	for _, payload := range payloads {
		models = append(models, compozyconfig.ProviderModelConfig{
			ID:                       strings.TrimSpace(payload.ID),
			DisplayName:              strings.TrimSpace(payload.DisplayName),
			ContextWindow:            cloneInt64Ptr(payload.ContextWindow),
			MaxInputTokens:           cloneInt64Ptr(payload.MaxInputTokens),
			MaxOutputTokens:          cloneInt64Ptr(payload.MaxOutputTokens),
			SupportsTools:            cloneBoolPtr(payload.SupportsTools),
			SupportsReasoning:        cloneBoolPtr(payload.SupportsReasoning),
			ReasoningEfforts:         reasoningEffortsToStrings(payload.ReasoningEfforts),
			DefaultReasoningEffort:   strings.TrimSpace(string(payload.DefaultReasoningEffort)),
			CostInputPerMillion:      cloneFloat64Ptr(payload.CostInputPerMillion),
			CostOutputPerMillion:     cloneFloat64Ptr(payload.CostOutputPerMillion),
			CostCacheReadPerMillion:  cloneFloat64Ptr(payload.CostCacheReadPerMillion),
			CostCacheWritePerMillion: cloneFloat64Ptr(payload.CostCacheWritePerMillion),
			CostReasoningPerMillion:  cloneFloat64Ptr(payload.CostReasoningPerMillion),
			Deprecated:               cloneBoolPtr(payload.Deprecated),
			Hidden:                   cloneBoolPtr(payload.Hidden),
			Featured:                 cloneBoolPtr(payload.Featured),
			ReleaseDate:              strings.TrimSpace(payload.ReleaseDate),
		})
	}
	return models
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
