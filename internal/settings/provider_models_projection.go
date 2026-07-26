package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
)

func (s *service) providerSettingsFromConfig(
	ctx context.Context,
	name string,
	provider aghconfig.ProviderConfig,
) (ProviderSettings, error) {
	settings := providerSettingsBaseFromConfig(name, provider)
	if s.modelCatalog == nil {
		return settings, nil
	}
	models, err := s.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: name,
		View:       modelcatalog.CatalogViewCurated,
	})
	if err != nil {
		if errors.Is(err, modelcatalog.ErrAllSourcesFailed) {
			return settings, nil
		}
		return ProviderSettings{}, fmt.Errorf("settings: list merged models for provider %q: %w", name, err)
	}
	settings.Models.Curated = providerModelConfigsFromCatalog(models, provider.Models.Curated)
	return settings, nil
}

func providerSettingsBaseFromConfig(name string, provider aghconfig.ProviderConfig) ProviderSettings {
	models := cloneProviderModelsConfig(provider.Models)
	models.Curated = nil
	return ProviderSettings{
		Command:         provider.Command,
		DisplayName:     provider.DisplayName,
		Models:          models,
		Harness:         provider.EffectiveHarness(),
		RuntimeProvider: provider.RuntimeProviderName(name),
		Transport:       strings.TrimSpace(provider.Transport),
		BaseURL:         strings.TrimSpace(provider.BaseURL),
		AuthMode:        provider.EffectiveAuthMode(),
		EnvPolicy:       provider.EffectiveEnvPolicy(),
		HomePolicy:      provider.EffectiveHomePolicy(),
		AuthStatusCmd:   strings.TrimSpace(provider.AuthStatusCmd),
		AuthLoginCmd:    strings.TrimSpace(provider.AuthLoginCmd),
		CredentialSlots: provider.EffectiveCredentialSlots(),
	}
}

func providerFallbackFromBuiltin(name string, builtin aghconfig.ProviderConfig) *ProviderFallback {
	return &ProviderFallback{
		Source:   builtinProviderSource(),
		Settings: providerSettingsBaseFromConfig(name, builtin),
	}
}

func providerModelConfigsFromCatalog(
	models []modelcatalog.Model,
	configured []aghconfig.ProviderModelConfig,
) []aghconfig.ProviderModelConfig {
	if len(models) == 0 {
		return nil
	}
	configuredByID := make(map[string]aghconfig.ProviderModelConfig, len(configured))
	for _, model := range configured {
		if id := strings.TrimSpace(model.ID); id != "" {
			configuredByID[id] = model
		}
	}
	values := make([]aghconfig.ProviderModelConfig, 0, len(models))
	for _, model := range models {
		projected := providerModelConfigFromCatalog(model)
		if desired, ok := configuredByID[model.ModelID]; ok {
			applyDesiredProviderModelCuration(&projected, desired)
		}
		values = append(values, projected)
	}
	return values
}

func applyDesiredProviderModelCuration(
	projected *aghconfig.ProviderModelConfig,
	desired aghconfig.ProviderModelConfig,
) {
	if projected == nil {
		return
	}
	if effort := strings.TrimSpace(desired.DefaultReasoningEffort); effort != "" {
		projected.DefaultReasoningEffort = effort
	}
	if desired.Deprecated != nil {
		projected.Deprecated = cloneBoolPtr(desired.Deprecated)
	}
	if desired.Hidden != nil {
		projected.Hidden = cloneBoolPtr(desired.Hidden)
	}
	if desired.Featured != nil {
		projected.Featured = cloneBoolPtr(desired.Featured)
	}
}

func providerModelConfigFromCatalog(model modelcatalog.Model) aghconfig.ProviderModelConfig {
	defaultEffort := ""
	if model.DefaultReasoningEffort != nil {
		defaultEffort = string(*model.DefaultReasoningEffort)
	}
	releaseDate := ""
	if model.ReleaseDate != nil {
		releaseDate = *model.ReleaseDate
	}
	efforts := make([]string, 0, len(model.ReasoningEfforts))
	for _, effort := range model.ReasoningEfforts {
		efforts = append(efforts, string(effort))
	}
	return aghconfig.ProviderModelConfig{
		ID:                       model.ModelID,
		DisplayName:              model.DisplayName,
		ContextWindow:            cloneInt64Ptr(model.ContextWindow),
		MaxInputTokens:           cloneInt64Ptr(model.MaxInputTokens),
		MaxOutputTokens:          cloneInt64Ptr(model.MaxOutputTokens),
		SupportsTools:            cloneBoolPtr(model.SupportsTools),
		SupportsReasoning:        cloneBoolPtr(model.SupportsReasoning),
		ReasoningEfforts:         efforts,
		DefaultReasoningEffort:   defaultEffort,
		CostInputPerMillion:      cloneFloat64Ptr(model.CostInputPerMillion),
		CostOutputPerMillion:     cloneFloat64Ptr(model.CostOutputPerMillion),
		CostCacheReadPerMillion:  cloneFloat64Ptr(model.CostCacheReadPerMillion),
		CostCacheWritePerMillion: cloneFloat64Ptr(model.CostCacheWritePerMillion),
		CostReasoningPerMillion:  cloneFloat64Ptr(model.CostReasoningPerMillion),
		Deprecated:               new(model.Deprecated),
		Hidden:                   new(model.Hidden),
		Featured:                 new(model.Featured),
		ReleaseDate:              releaseDate,
	}
}

func cloneProviderModelsConfig(value aghconfig.ProviderModelsConfig) aghconfig.ProviderModelsConfig {
	return aghconfig.ProviderModelsConfig{
		Default:   value.Default,
		Curated:   cloneProviderModelConfigs(value.Curated),
		Discovery: cloneProviderModelsDiscoveryConfig(value.Discovery),
		Reasoning: value.Reasoning,
	}
}

func cloneProviderModelsDiscoveryConfig(
	value aghconfig.ProviderModelsDiscoveryConfig,
) aghconfig.ProviderModelsDiscoveryConfig {
	return aghconfig.ProviderModelsDiscoveryConfig{
		Enabled:  cloneBoolPtr(value.Enabled),
		Command:  value.Command,
		Endpoint: value.Endpoint,
		Timeout:  value.Timeout,
	}
}

func cloneProviderModelConfigs(values []aghconfig.ProviderModelConfig) []aghconfig.ProviderModelConfig {
	if values == nil {
		return nil
	}
	cloned := make([]aghconfig.ProviderModelConfig, len(values))
	for idx, value := range values {
		cloned[idx] = aghconfig.ProviderModelConfig{
			ID:                       value.ID,
			DisplayName:              value.DisplayName,
			ContextWindow:            cloneInt64Ptr(value.ContextWindow),
			MaxInputTokens:           cloneInt64Ptr(value.MaxInputTokens),
			MaxOutputTokens:          cloneInt64Ptr(value.MaxOutputTokens),
			SupportsTools:            cloneBoolPtr(value.SupportsTools),
			SupportsReasoning:        cloneBoolPtr(value.SupportsReasoning),
			ReasoningEfforts:         cloneStringSlicePreserveNil(value.ReasoningEfforts),
			DefaultReasoningEffort:   value.DefaultReasoningEffort,
			CostInputPerMillion:      cloneFloat64Ptr(value.CostInputPerMillion),
			CostOutputPerMillion:     cloneFloat64Ptr(value.CostOutputPerMillion),
			CostCacheReadPerMillion:  cloneFloat64Ptr(value.CostCacheReadPerMillion),
			CostCacheWritePerMillion: cloneFloat64Ptr(value.CostCacheWritePerMillion),
			CostReasoningPerMillion:  cloneFloat64Ptr(value.CostReasoningPerMillion),
			Deprecated:               cloneBoolPtr(value.Deprecated),
			Hidden:                   cloneBoolPtr(value.Hidden),
			Featured:                 cloneBoolPtr(value.Featured),
			ReleaseDate:              value.ReleaseDate,
		}
	}
	return cloned
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringSlicePreserveNil(value []string) []string {
	if value == nil {
		return nil
	}
	cloned := make([]string, len(value))
	copy(cloned, value)
	return cloned
}
