package globaldb

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/store"
)

// ListRowOwners returns distinct source/provider pairs owning rows that match opts.
// It shares ListRows filters but skips row scanning and option/effort/binding hydration.
func (g *ModelCatalogRepo) ListRowOwners(
	ctx context.Context,
	opts modelcatalog.ListOptions,
) (owners []modelcatalog.SourceProviderKey, err error) {
	if err := g.checkReady(ctx, "list model catalog row owners"); err != nil {
		return nil, err
	}
	where, args, err := modelCatalogRowFilterClauses(opts, "")
	if err != nil {
		return nil, err
	}
	// dynamic-sql: the owner scan reuses the optional model-row filter set selected at runtime.
	sqlQuery := store.AppendWhere(`SELECT DISTINCT source_id, provider_id FROM model_catalog_rows`, where)
	sqlQuery += ` ORDER BY source_id ASC, provider_id ASC`

	rows, err := g.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("store: query model catalog row owners: %w", err)
	}
	defer func() {
		err = joinRowsCloseError(rows, err, "model catalog row owner query")
	}()

	owners = make([]modelcatalog.SourceProviderKey, 0)
	for rows.Next() {
		var owner modelcatalog.SourceProviderKey
		if err := rows.Scan(&owner.SourceID, &owner.ProviderID); err != nil {
			return nil, fmt.Errorf("store: scan model catalog row owner: %w", err)
		}
		owners = append(owners, owner)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate model catalog row owners: %w", err)
	}
	return owners, nil
}
