package globaldb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// GetMarketplaceExtensionByInstallSlug returns the exact curated extension acquisition row.
func (r *MarketplaceRepo) GetMarketplaceExtensionByInstallSlug(
	ctx context.Context,
	installSlug string,
	version string,
) (store.MarketplaceCatalogEntry, error) {
	if err := r.checkReady(ctx, "get marketplace extension by install slug"); err != nil {
		return store.MarketplaceCatalogEntry{}, err
	}
	row, err := r.queries.GetMarketplaceExtensionByInstallSlug(
		ctx,
		sqlcgen.GetMarketplaceExtensionByInstallSlugParams{
			InstallSlug: sql.NullString{String: installSlug, Valid: true},
			Version:     version,
		},
	)
	if err != nil {
		return store.MarketplaceCatalogEntry{}, fmt.Errorf(
			"store: get marketplace extension install %q version %q: %w",
			installSlug,
			version,
			err,
		)
	}
	return marketplaceCatalogEntryFromRow(row), nil
}
