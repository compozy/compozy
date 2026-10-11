package session

import (
	"context"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/modelcatalog"
)

func (m *Manager) recentLiveProviderModel(
	ctx context.Context,
	providerID, modelID string,
	executionContext modelcatalog.CatalogExecutionContext,
) (modelcatalog.Model, bool) {
	if m == nil || m.modelCatalog == nil {
		return modelcatalog.Model{}, false
	}
	now := time.Now()
	if m.now != nil {
		now = m.now()
	}
	models, err := m.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: providerID, ExecutionContext: executionContext,
		View: modelcatalog.CatalogViewAll, SkipRefreshIfEmpty: true, Now: now,
	})
	if err != nil {
		return modelcatalog.Model{}, false
	}
	for _, model := range models {
		if model.ProviderID != providerID || model.ModelID != strings.TrimSpace(modelID) ||
			model.AvailabilityState != modelcatalog.AvailabilityStateAvailableLive {
			continue
		}
		for _, source := range model.Sources {
			age := now.Sub(source.RefreshedAt)
			if source.SourceID == modelcatalog.SourceKindProviderLiveID(providerID) && !source.Stale &&
				!source.RefreshedAt.IsZero() && age >= 0 && age < modelcatalog.DefaultLiveDiscoveryTTL {
				return model, true
			}
		}
	}
	return modelcatalog.Model{}, false
}
