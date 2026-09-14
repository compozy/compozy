package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ReadMarketplaceCatalogSnapshot reads entries and their revision in one transaction.
func (r *MarketplaceRepo) ReadMarketplaceCatalogSnapshot(
	ctx context.Context, source string, limit int64,
) (snapshot store.MarketplaceCatalogSnapshot, err error) {
	if err := r.checkReady(ctx, "read marketplace catalog snapshot"); err != nil {
		return snapshot, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return snapshot, fmt.Errorf("store: begin marketplace snapshot: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: close marketplace snapshot: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	state, err := queries.GetMarketplaceCatalogState(ctx, source)
	if err != nil {
		return snapshot, fmt.Errorf("store: read marketplace snapshot state: %w", err)
	}
	rows, err := queries.ListMarketplaceCatalogEntries(ctx, sqlcgen.ListMarketplaceCatalogEntriesParams{
		Source: source, ResultLimit: limit,
	})
	if err != nil {
		return snapshot, fmt.Errorf("store: read marketplace snapshot entries: %w", err)
	}
	snapshot.State = marketplaceCatalogStateFromRow(state)
	snapshot.Entries = make([]store.MarketplaceCatalogEntry, 0, len(rows))
	for _, row := range rows {
		snapshot.Entries = append(snapshot.Entries, marketplaceCatalogEntryFromRow(row))
	}
	return snapshot, nil
}
