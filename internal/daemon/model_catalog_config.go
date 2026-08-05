package daemon

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
)

type modelCatalogMergeOptionsUpdater interface {
	UpdateMergeOptions(options modelcatalog.MergeOptions)
}

func (r *modelCatalogRuntime) ReconcileConfig(ctx context.Context, cfg *compozyconfig.Config) error {
	if r == nil || r.configSource == nil {
		return errors.New("daemon: model catalog config source is unavailable")
	}
	if ctx == nil {
		return errors.New("daemon: model catalog config reconciliation context is required")
	}
	reasoningApply, err := effectiveCatalogReasoningApply(cfg)
	if err != nil {
		return err
	}
	previousProviders := r.configSource.ProviderIDs()
	nextProviders := map[string]compozyconfig.ProviderConfig(nil)
	if cfg != nil {
		nextProviders = cfg.Providers
	}
	r.configSource.ReplaceProviders(nextProviders)

	if updater, ok := r.service.(modelCatalogMergeOptionsUpdater); ok {
		updater.UpdateMergeOptions(modelcatalog.MergeOptions{ReasoningApply: reasoningApply})
	}

	changedLiveProviders := r.replaceLiveProviderConfigs(nextProviders)

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
	for _, providerID := range changedLiveProviders {
		statuses, err := r.Refresh(ctx, modelcatalog.RefreshOptions{
			ProviderID: providerID,
			SourceID:   modelcatalog.SourceKindProviderLiveID(providerID),
			Force:      true,
			Now:        now,
		})
		if err != nil {
			if ctx.Err() != nil {
				return fmt.Errorf(
					"daemon: reconcile live model catalog provider %q: %w",
					providerID,
					ctx.Err(),
				)
			}
			if !errors.Is(err, modelcatalog.ErrAllSourcesFailed) &&
				!recordedLiveRefreshFailure(statuses, providerID) {
				return fmt.Errorf(
					"daemon: reconcile live model catalog provider %q: %w",
					providerID,
					err,
				)
			}
		}
	}
	return nil
}

func (r *modelCatalogRuntime) replaceLiveProviderConfigs(
	providers map[string]compozyconfig.ProviderConfig,
) []string {
	effectiveProviders := compozyconfig.BuiltinProviders()
	maps.Copy(effectiveProviders, providers)
	changed := make([]string, 0, len(r.liveSources))
	for providerID, source := range r.liveSources {
		if source.ReplaceProvider(effectiveProviders[providerID]) {
			changed = append(changed, providerID)
		}
	}
	sort.Strings(changed)
	return changed
}

func recordedLiveRefreshFailure(statuses []modelcatalog.SourceStatus, providerID string) bool {
	sourceID := modelcatalog.SourceKindProviderLiveID(providerID)
	for _, status := range statuses {
		if status.ProviderID == providerID &&
			status.SourceID == sourceID &&
			status.RefreshState == modelcatalog.RefreshStateFailed {
			return true
		}
	}
	return false
}
