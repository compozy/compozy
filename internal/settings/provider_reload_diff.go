package settings

import (
	"reflect"
	"sort"

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
	orderedIDs := make([]string, 0, len(providerIDs))
	for providerID := range providerIDs {
		orderedIDs = append(orderedIDs, providerID)
	}
	sort.Strings(orderedIDs)

	changed := make([]string, 0, len(orderedIDs))
	for _, providerID := range orderedIDs {
		currentProvider := current[providerID]
		desiredProvider := desired[providerID]
		if reflect.DeepEqual(currentProvider, desiredProvider) {
			continue
		}

		currentModels := currentProvider.Models
		desiredModels := desiredProvider.Models
		currentProvider.Models = compozyconfig.ProviderModelsConfig{}
		desiredProvider.Models = compozyconfig.ProviderModelsConfig{}
		if !reflect.DeepEqual(currentProvider, desiredProvider) {
			changed = append(changed, "providers."+providerID)
		}
		if !reflect.DeepEqual(currentModels, desiredModels) {
			changed = append(changed, "providers."+providerID+".models")
		}
	}
	return changed
}
