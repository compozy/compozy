package settings

import (
	"context"
	"fmt"
	"maps"
	"reflect"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
)

type providerWriteClassification struct {
	modelOnly bool
	noOp      bool
}

func (s *service) classifyProviderWrite(
	ctx context.Context,
	name string,
	settings ProviderSettings,
) (providerWriteClassification, error) {
	currentConfig, _, err := s.loadConfig(ctx, ScopeGlobal, "")
	if err != nil {
		return providerWriteClassification{}, fmt.Errorf(
			"load config for provider %q mutation validation: %w",
			name,
			err,
		)
	}
	currentProvider, currentErr := currentConfig.ResolveProvider(name)
	nextConfig := currentConfig
	nextConfig.Providers = make(map[string]aghconfig.ProviderConfig, len(currentConfig.Providers)+1)
	maps.Copy(nextConfig.Providers, currentConfig.Providers)
	nextConfig.Providers[name] = providerConfigFromSettings(settings)
	if err := nextConfig.Validate(); err != nil {
		return providerWriteClassification{}, fmt.Errorf("validate provider %q mutation: %w", name, err)
	}
	nextProvider, err := nextConfig.ResolveProvider(name)
	if err != nil {
		return providerWriteClassification{}, fmt.Errorf(
			"resolve provider %q after mutation validation: %w",
			name,
			err,
		)
	}
	if currentErr != nil {
		return providerWriteClassification{}, nil
	}

	currentNormalized := providerConfigFromSettings(providerSettingsBaseFromConfig(name, currentProvider))
	nextNormalized := providerConfigFromSettings(providerSettingsBaseFromConfig(name, nextProvider))
	currentModels := currentNormalized.Models
	nextModels := nextNormalized.Models
	currentNormalized.Models = aghconfig.ProviderModelsConfig{}
	nextNormalized.Models = aghconfig.ProviderModelsConfig{}
	classification := providerWriteClassification{
		modelOnly: reflect.DeepEqual(currentNormalized, nextNormalized),
	}
	if !classification.modelOnly || !reflect.DeepEqual(currentModels, nextModels) {
		return classification, nil
	}
	currentRawCurated := providerModelConfigsFromSettings(currentConfig.Providers[name].Models.Curated)
	nextRawCurated := providerModelConfigsFromSettings(settings.Models.Curated)
	classification.noOp = reflect.DeepEqual(currentRawCurated, nextRawCurated)
	return classification, nil
}

func mutationResultForProvider(target WriteTargetKind, modelOnly bool) MutationResult {
	if !modelOnly {
		return mutationResultForCollection(CollectionProviders, ScopeGlobal, "", target)
	}
	return MutationResult{
		Section:     SectionName(CollectionProviders),
		Scope:       ScopeGlobal,
		WriteTarget: target,
		Behavior:    MutationBehaviorAppliedNow,
		Applied:     true,
		Lifecycle:   lifecycle.Live,
		DiffClass:   lifecycle.DiffClassLive,
	}
}
