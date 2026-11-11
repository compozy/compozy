package settings

import (
	"maps"
	"reflect"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func diffProviderSettings(
	current map[string]compozyconfig.ProviderConfig,
	desired map[string]compozyconfig.ProviderConfig,
) []string {
	if reflect.DeepEqual(current, desired) {
		return nil
	}
	providerIDs := make(map[string]struct{}, len(current)+len(desired))
	for providerID := range current {
		providerIDs[providerID] = struct{}{}
	}
	for providerID := range desired {
		providerIDs[providerID] = struct{}{}
	}
	orderedIDs := slices.Sorted(maps.Keys(providerIDs))

	changed := make([]string, 0, len(orderedIDs))
	for _, providerID := range orderedIDs {
		currentProvider := current[providerID]
		desiredProvider := desired[providerID]
		if reflect.DeepEqual(currentProvider, desiredProvider) {
			continue
		}

		currentModels := currentProvider.Models
		desiredModels := desiredProvider.Models
		currentEnabled := currentProvider.Enabled
		desiredEnabled := desiredProvider.Enabled
		currentProvider.Models = compozyconfig.ProviderModelsConfig{}
		desiredProvider.Models = compozyconfig.ProviderModelsConfig{}
		currentProvider.Enabled = nil
		desiredProvider.Enabled = nil
		if !reflect.DeepEqual(currentProvider, desiredProvider) {
			changed = append(changed, "providers."+providerID)
		}
		if !reflect.DeepEqual(currentModels, desiredModels) {
			changed = append(changed, "providers."+providerID+".models")
		}
		if !reflect.DeepEqual(currentEnabled, desiredEnabled) {
			changed = append(changed, "providers."+providerID+".enabled")
		}
	}
	return changed
}
