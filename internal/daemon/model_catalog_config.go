package daemon

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
)

type modelCatalogMergeOptionsUpdater interface {
	UpdateMergeOptions(options modelcatalog.MergeOptions)
}

func (r *modelCatalogRuntime) ReconcileConfig(ctx context.Context, cfg *aghconfig.Config) error {
	if r == nil || r.configSource == nil {
		return errors.New("daemon: model catalog config source is unavailable")
	}
	if ctx == nil {
		return errors.New("daemon: model catalog config reconciliation context is required")
	}
	previousProviders := r.configSource.ProviderIDs()
	nextProviders := map[string]aghconfig.ProviderConfig(nil)
	if cfg != nil {
		nextProviders = cfg.Providers
	}
	r.configSource.ReplaceProviders(nextProviders)

	reasoningApply, err := effectiveCatalogReasoningApply(cfg)
	if err != nil {
		return err
	}
	if updater, ok := r.service.(modelCatalogMergeOptionsUpdater); ok {
		updater.UpdateMergeOptions(modelcatalog.MergeOptions{ReasoningApply: reasoningApply})
	}

	providerSet := make(map[string]struct{}, len(previousProviders)+len(nextProviders))
	for _, providerID := range previousProviders {
		providerSet[providerID] = struct{}{}
	}
	for providerID := range nextProviders {
		providerSet[providerID] = struct{}{}
	}
	providerIDs := make([]string, 0, len(providerSet))
	for providerID := range providerSet {
		providerIDs = append(providerIDs, providerID)
	}
	sort.Strings(providerIDs)
	now := time.Now().UTC()
	if r.now != nil {
		now = r.now().UTC()
	}
	for _, providerID := range providerIDs {
		if _, err := r.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: providerID,
			SourceID:   modelcatalog.SourceIDConfig,
			Force:      true,
			Now:        now,
		}); err != nil {
			return fmt.Errorf("daemon: reconcile model catalog config provider %q: %w", providerID, err)
		}
	}
	return nil
}
