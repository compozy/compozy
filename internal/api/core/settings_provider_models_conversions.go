package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
)

func settingsProviderModelsPayload(
	value aghconfig.ProviderModelsConfig,
) *contract.SettingsProviderModelsPayload {
	if providerModelsConfigIsEmpty(value) {
		return nil
	}
	payload := &contract.SettingsProviderModelsPayload{
		Default:   strings.TrimSpace(value.Default),
		Curated:   settingsProviderModelPayloads(value.Curated),
		Discovery: settingsProviderModelsDiscoveryPayload(value.Discovery),
	}
	if value.Reasoning.Apply != "" {
		payload.Reasoning = &contract.SettingsProviderReasoningPayload{Apply: string(value.Reasoning.Apply)}
	}
	return payload
}

func settingsProviderModelsDiscoveryPayload(
	value aghconfig.ProviderModelsDiscoveryConfig,
) *contract.SettingsProviderModelsDiscoveryPayload {
	if value.Enabled == nil &&
		strings.TrimSpace(value.Command) == "" &&
		strings.TrimSpace(value.Endpoint) == "" &&
		strings.TrimSpace(value.Timeout) == "" {
		return nil
	}
	return &contract.SettingsProviderModelsDiscoveryPayload{
		Enabled:  cloneBoolPtr(value.Enabled),
		Command:  strings.TrimSpace(value.Command),
		Endpoint: strings.TrimSpace(value.Endpoint),
		Timeout:  strings.TrimSpace(value.Timeout),
	}
}

func settingsProviderModelPayloads(
	values []aghconfig.ProviderModelConfig,
) []contract.SettingsProviderModelPayload {
	if values == nil {
		return nil
	}
	payloads := make([]contract.SettingsProviderModelPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsProviderModelPayload{
			ID:                       strings.TrimSpace(value.ID),
			DisplayName:              strings.TrimSpace(value.DisplayName),
			ContextWindow:            cloneInt64Ptr(value.ContextWindow),
			MaxInputTokens:           cloneInt64Ptr(value.MaxInputTokens),
			MaxOutputTokens:          cloneInt64Ptr(value.MaxOutputTokens),
			SupportsTools:            cloneBoolPtr(value.SupportsTools),
			SupportsReasoning:        cloneBoolPtr(value.SupportsReasoning),
			ReasoningEfforts:         reasoningEffortsFromStrings(value.ReasoningEfforts),
			DefaultReasoningEffort:   contract.ReasoningEffort(strings.TrimSpace(value.DefaultReasoningEffort)),
			CostInputPerMillion:      cloneFloat64Ptr(value.CostInputPerMillion),
			CostOutputPerMillion:     cloneFloat64Ptr(value.CostOutputPerMillion),
			CostCacheReadPerMillion:  cloneFloat64Ptr(value.CostCacheReadPerMillion),
			CostCacheWritePerMillion: cloneFloat64Ptr(value.CostCacheWritePerMillion),
			CostReasoningPerMillion:  cloneFloat64Ptr(value.CostReasoningPerMillion),
			Deprecated:               cloneBoolPtr(value.Deprecated),
			Hidden:                   cloneBoolPtr(value.Hidden),
			Featured:                 cloneBoolPtr(value.Featured),
			ReleaseDate:              strings.TrimSpace(value.ReleaseDate),
		})
	}
	return payloads
}

func providerModelsConfigIsEmpty(value aghconfig.ProviderModelsConfig) bool {
	return strings.TrimSpace(value.Default) == "" &&
		value.Curated == nil &&
		value.Discovery.Enabled == nil &&
		strings.TrimSpace(value.Discovery.Command) == "" &&
		strings.TrimSpace(value.Discovery.Endpoint) == "" &&
		strings.TrimSpace(value.Discovery.Timeout) == "" &&
		value.Reasoning.Apply == ""
}
