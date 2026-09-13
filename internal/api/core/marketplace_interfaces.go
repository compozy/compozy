package core

import (
	"context"

	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

// MarketplaceCatalogService exposes the daemon-owned curated feed projection.
type MarketplaceCatalogService interface {
	Browse(context.Context, marketplacepkg.Kind, string, int, int) (marketplacepkg.BrowseResult, error)
	Detail(context.Context, marketplacepkg.Kind, string) (*marketplacepkg.Entry, error)
	Refresh(context.Context, ...marketplacepkg.Kind) (marketplacepkg.RefreshReport, error)
}
