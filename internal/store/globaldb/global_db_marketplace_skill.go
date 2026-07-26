package globaldb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/agh/internal/store"
)

// ListMarketplaceSkillsByInstallSlugs returns curated skill rows for exact acquisition slugs.
func (r *MarketplaceRepo) ListMarketplaceSkillsByInstallSlugs(
	ctx context.Context,
	installSlugs []string,
) ([]store.MarketplaceCatalogEntry, error) {
	if err := r.checkReady(ctx, "list marketplace skills by install slug"); err != nil {
		return nil, err
	}
	params := make([]sql.NullString, 0, len(installSlugs))
	for _, installSlug := range installSlugs {
		params = append(params, marketplaceCatalogNullString(installSlug))
	}
	rows, err := r.queries.ListMarketplaceSkillsByInstallSlugs(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("store: list marketplace skills by install slug: %w", err)
	}
	entries := make([]store.MarketplaceCatalogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, marketplaceCatalogEntryFromRow(row))
	}
	return entries, nil
}
