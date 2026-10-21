package settings

import (
	"context"
	"errors"
	"maps"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type ProviderUsageSection struct {
	Config compozyconfig.ProviderUsageConfig
}

func (s *service) updateProviderUsageSection(ctx context.Context, req SectionUpdateRequest) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.ProviderUsage == nil {
		return MutationResult{}, validationError(errors.New("settings: provider-usage section payload is required"))
	}
	desired := req.ProviderUsage.Clone()
	if err := desired.Validate(cfg.Providers); err != nil {
		return MutationResult{}, validationError(err)
	}
	changed := diffProviderUsageSettings(cfg.ProviderUsage, desired)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyProviderUsageSettings(editor, desired)
	})
}

func diffProviderUsageSettings(current, desired compozyconfig.ProviderUsageConfig) []string {
	var changed []string
	if current.Enabled != desired.Enabled {
		changed = append(changed, "provider_usage.enabled")
	}
	if current.RefreshInterval != desired.RefreshInterval {
		changed = append(changed, "provider_usage.refresh_interval")
	}
	ids := make(map[string]struct{}, len(current.Providers)+len(desired.Providers))
	for id := range current.Providers {
		ids[id] = struct{}{}
	}
	for id := range desired.Providers {
		ids[id] = struct{}{}
	}
	for _, id := range slices.Sorted(maps.Keys(ids)) {
		before, had := current.Providers[id]
		after, has := desired.Providers[id]
		if had != has || before.Enabled != after.Enabled {
			changed = append(changed, "provider_usage.providers."+id+".enabled")
		}
		if before.KeychainAccess != after.KeychainAccess {
			changed = append(changed, "provider_usage.providers."+id+".keychain_access")
		}
	}
	return changed
}

func applyProviderUsageSettings(editor *compozyconfig.OverlayEditor, cfg compozyconfig.ProviderUsageConfig) error {
	providers := make(map[string]any, len(cfg.Providers))
	for id, provider := range cfg.Providers {
		values := map[string]any{"enabled": provider.Enabled}
		if id == "claude" {
			values["keychain_access"] = provider.KeychainAccess
		}
		providers[id] = values
	}
	return editor.SetTable([]string{"provider_usage"}, map[string]any{
		"enabled":                   cfg.Enabled,
		"refresh_interval":          cfg.RefreshInterval.String(),
		string(CollectionProviders): providers,
	})
}
