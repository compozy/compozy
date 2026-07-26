package daemon

import (
	"fmt"
	"sort"

	aghconfig "github.com/compozy/agh/internal/config"
)

func effectiveCatalogReasoningApply(cfg *aghconfig.Config) (map[string]bool, error) {
	providerIDs := make(map[string]struct{})
	for providerID := range aghconfig.BuiltinProviders() {
		providerIDs[providerID] = struct{}{}
	}
	if cfg != nil {
		for providerID := range cfg.Providers {
			providerIDs[providerID] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(providerIDs))
	for providerID := range providerIDs {
		ordered = append(ordered, providerID)
	}
	sort.Strings(ordered)

	result := make(map[string]bool, len(ordered))
	for _, providerID := range ordered {
		provider, err := cfg.ResolveProvider(providerID)
		if err != nil {
			return nil, fmt.Errorf("daemon: resolve model catalog provider %q: %w", providerID, err)
		}
		result[providerID] = provider.Models.EffectiveReasoningApply() == aghconfig.ReasoningApplyACPOption
	}
	return result, nil
}
