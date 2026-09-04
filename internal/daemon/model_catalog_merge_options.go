package daemon

import (
	"fmt"
	"sort"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func effectiveCatalogMergeOptions(cfg *compozyconfig.Config) (modelcatalog.MergeOptions, error) {
	providerIDs := make(map[string]struct{})
	for providerID := range compozyconfig.BuiltinProviders() {
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

	options := modelcatalog.MergeOptions{
		ReasoningApply: make(map[string]bool, len(ordered)),
		DefaultModels:  make(map[string]string, len(ordered)),
	}
	for _, providerID := range ordered {
		provider, err := cfg.ResolveProvider(providerID)
		if err != nil {
			return modelcatalog.MergeOptions{}, fmt.Errorf(
				"daemon: resolve model catalog provider %q: %w",
				providerID,
				err,
			)
		}
		options.ReasoningApply[providerID] =
			provider.Models.EffectiveReasoningApply() == compozyconfig.ReasoningApplyACPOption
		if defaultModel := strings.TrimSpace(provider.Models.Default); defaultModel != "" {
			options.DefaultModels[providerID] = defaultModel
		}
	}
	return options, nil
}
