package settings

import (
	"context"
	"errors"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const marketplaceTTLKey = "ttl"

func (s *service) updateMarketplaceSection(ctx context.Context, req SectionUpdateRequest) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Marketplace == nil {
		return MutationResult{}, validationError(errors.New("settings: marketplace catalog payload is required"))
	}
	if err := req.Marketplace.Validate("marketplace.catalog"); err != nil {
		return MutationResult{}, validationError(err)
	}
	current, desired := cfg.Marketplace.Catalog, *req.Marketplace
	var changed []string
	if current.BaseURL != desired.BaseURL {
		changed = append(changed, "marketplace.catalog.base_url")
	}
	if current.TTL != desired.TTL {
		changed = append(changed, "marketplace.catalog.ttl")
	}
	if current.Timeout != desired.Timeout {
		changed = append(changed, "marketplace.catalog.timeout")
	}
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		for _, field := range []struct{ key, value string }{
			{"base_url", desired.BaseURL}, {marketplaceTTLKey, desired.TTL}, {sectionsTimeoutKey, desired.Timeout},
		} {
			if !slices.Contains(changed, "marketplace.catalog."+field.key) {
				continue
			}
			if err := editor.SetValue([]string{"marketplace", "catalog", field.key}, field.value); err != nil {
				return err
			}
		}
		return nil
	})
}
