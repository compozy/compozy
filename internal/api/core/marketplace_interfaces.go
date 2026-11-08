package core

import (
	"context"

	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
)

// MarketplaceCatalogService exposes the daemon-owned curated feed projection.
type MarketplaceCatalogService interface {
	Entry(context.Context, marketplacepkg.Origin) (*marketplacepkg.Entry, error)
	BrowseFiltered(context.Context, marketplacepkg.BrowseFilter, int, int) (marketplacepkg.BrowseResult, error)
	Detail(context.Context, string, string) (*marketplacepkg.Entry, error)
	Refresh(context.Context, ...string) (marketplacepkg.RefreshReport, error)
}
