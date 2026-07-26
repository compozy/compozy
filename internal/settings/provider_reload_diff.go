package settings

import (
	"reflect"
	"sort"

	aghconfig "github.com/compozy/agh/internal/config"
)

func diffProviderSettings(
	current map[string]aghconfig.ProviderConfig,
	desired map[string]aghconfig.ProviderConfig,
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
		currentProvider, currentExists := current[providerID]
		desiredProvider, desiredExists := desired[providerID]
		if !currentExists || !desiredExists {
			changed = append(changed, "providers."+providerID)
			continue
		}
		if reflect.DeepEqual(currentProvider, desiredProvider) {
			continue
		}

		currentModels := currentProvider.Models
		desiredModels := desiredProvider.Models
		currentProvider.Models = aghconfig.ProviderModelsConfig{}
		desiredProvider.Models = aghconfig.ProviderModelsConfig{}
		if !reflect.DeepEqual(currentProvider, desiredProvider) {
			changed = append(changed, "providers."+providerID)
		}
		if !reflect.DeepEqual(currentModels, desiredModels) {
			changed = append(changed, "providers."+providerID+".models")
		}
	}
	return changed
}
