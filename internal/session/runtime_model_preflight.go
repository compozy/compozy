package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
)

func (m *Manager) validateExplicitModel(
	ctx context.Context,
	spec *sessionStartSpec,
	resolved aghconfig.ResolvedAgent,
) error {
	modelID := strings.TrimSpace(spec.model)
	providerID := strings.TrimSpace(resolved.Provider)
	if modelID == "" || !modelcatalog.HasAuthoritativeProviderCatalog(providerID) {
		return nil
	}
	if m.modelCatalog == nil {
		return fmt.Errorf("%w: model catalog is unavailable for provider %q", ErrInvalidRuntimeOverride, providerID)
	}

	models, err := m.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:         providerID,
		View:               modelcatalog.CatalogViewAll,
		IncludeAll:         true,
		IncludeStale:       true,
		SkipRefreshIfEmpty: true,
	})
	if err != nil {
		return fmt.Errorf("session: list models for provider %q: %w", providerID, err)
	}

	choices := make([]string, 0, len(models))
	for _, model := range models {
		candidate := strings.TrimSpace(model.ModelID)
		if candidate == "" {
			continue
		}
		if model.Available != nil && !*model.Available {
			continue
		}
		choices = append(choices, candidate)
		if candidate == modelID {
			return nil
		}
	}
	sort.Strings(choices)
	if len(choices) == 0 {
		return fmt.Errorf(
			"%w: provider %q has no available explicit models; leave model empty to use its native default",
			ErrInvalidRuntimeOverride,
			providerID,
		)
	}
	return fmt.Errorf(
		"%w: model %q is unavailable for provider %q; choose one of: %s",
		ErrInvalidRuntimeOverride,
		modelID,
		providerID,
		strings.Join(choices, ", "),
	)
}
