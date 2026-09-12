package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/observe"
)

type sessionContextWindowResolver struct{ catalog observe.CostCatalog }

var _ core.ContextWindowResolver = sessionContextWindowResolver{}

func (r sessionContextWindowResolver) ContextWindow(ctx context.Context, provider, model string) (*int64, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if r.catalog == nil || provider == "" || model == "" {
		return nil, nil
	}
	models, err := r.catalog.ListModels(
		ctx,
		modelcatalog.ListOptions{
			ProviderID:         provider,
			View:               modelcatalog.CatalogViewAll,
			SkipRefreshIfEmpty: true,
			IncludeAll:         true,
			IncludeStale:       true,
		},
	)
	if err != nil {
		return nil, err
	}
	for _, item := range models {
		if item.ProviderID == provider && item.ModelID == model && item.ContextWindow != nil &&
			*item.ContextWindow > 0 {
			return new(*item.ContextWindow), nil
		}
	}
	return nil, nil
}
