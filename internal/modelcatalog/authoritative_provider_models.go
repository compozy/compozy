package modelcatalog

import (
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	cursorProviderID     = "cursor"
	cursorGrok45HighFast = "grok-4.5[effort=high,fast=true]"
)

var authoritativeProviderModelIDs = map[string]map[string]struct{}{
	cursorProviderID: {
		cursorGrok45HighFast: {},
	},
}

// HasAuthoritativeProviderCatalog reports whether Compozy can validate explicit models before provider startup.
func HasAuthoritativeProviderCatalog(providerID string) bool {
	_, ok := authoritativeProviderModelIDs[strings.TrimSpace(providerID)]
	return ok
}

func builtinProviderConfigs() map[string]compozyconfig.ProviderConfig {
	providers := compozyconfig.BuiltinProviders()
	cursor := providers[cursorProviderID]
	cursor.Models.Curated = []compozyconfig.ProviderModelConfig{{
		ID:          cursorGrok45HighFast,
		DisplayName: "Grok 4.5 (High, Fast)",
	}}
	providers[cursorProviderID] = cursor
	return providers
}

func filterAuthoritativeProviderModels(models []Model) []Model {
	filtered := make([]Model, 0, len(models))
	for _, model := range models {
		allowed, authoritative := authoritativeProviderModelIDs[strings.TrimSpace(model.ProviderID)]
		if authoritative {
			if _, ok := allowed[strings.TrimSpace(model.ModelID)]; !ok {
				continue
			}
		}
		filtered = append(filtered, model)
	}
	return filtered
}
