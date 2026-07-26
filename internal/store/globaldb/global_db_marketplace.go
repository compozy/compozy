package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

var _ store.MarketplaceCatalogRepository = (*MarketplaceRepo)(nil)

// ReplaceMarketplaceCatalog atomically prunes and replaces one curated kind.
func (r *MarketplaceRepo) ReplaceMarketplaceCatalog(
	ctx context.Context,
	replacement store.MarketplaceCatalogReplacement,
) (err error) {
	if err := r.checkReady(ctx, "replace marketplace catalog"); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin marketplace catalog %q replacement: %w", replacement.Kind, err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(
				err,
				fmt.Errorf("store: roll back marketplace catalog %q replacement: %w", replacement.Kind, rollbackErr),
			)
		}
	}()

	queries := sqlcgen.New(tx)
	if err := queries.DeleteMarketplaceCatalogEntriesByKind(ctx, replacement.Kind); err != nil {
		return fmt.Errorf("store: prune marketplace catalog %q projection: %w", replacement.Kind, err)
	}
	for _, entry := range replacement.Entries {
		if err := insertMarketplaceCatalogEntry(ctx, queries, entry); err != nil {
			return err
		}
	}
	if err := queries.UpsertMarketplaceCatalogStateFresh(ctx, sqlcgen.UpsertMarketplaceCatalogStateFreshParams{
		Kind:            replacement.Kind,
		ManifestVersion: replacement.ManifestVersion,
		GeneratedAt:     marketplaceCatalogNullString(replacement.GeneratedAt),
		FetchedAt:       replacement.FetchedAt,
	}); err != nil {
		return fmt.Errorf("store: upsert marketplace catalog %q state: %w", replacement.Kind, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit marketplace catalog %q replacement: %w", replacement.Kind, err)
	}
	committed = true
	return nil
}

// MarkMarketplaceCatalogStale records a refresh failure without changing entries.
func (r *MarketplaceRepo) MarkMarketplaceCatalogStale(
	ctx context.Context,
	kind string,
	lastError string,
) error {
	if err := r.checkReady(ctx, "mark marketplace catalog stale"); err != nil {
		return err
	}
	if err := r.queries.MarkMarketplaceCatalogStateStale(ctx, sqlcgen.MarkMarketplaceCatalogStateStaleParams{
		Kind:      kind,
		LastError: lastError,
	}); err != nil {
		return fmt.Errorf("store: mark marketplace catalog %q stale: %w", kind, err)
	}
	return nil
}

// ListMarketplaceCatalogEntries returns deterministic filtered projections.
func (r *MarketplaceRepo) ListMarketplaceCatalogEntries(
	ctx context.Context,
	kind string,
	limit int64,
) ([]store.MarketplaceCatalogEntry, error) {
	if err := r.checkReady(ctx, "list marketplace catalog entries"); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListMarketplaceCatalogEntries(ctx, sqlcgen.ListMarketplaceCatalogEntriesParams{
		Kind:        kind,
		ResultLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list marketplace catalog %q entries: %w", kind, err)
	}
	entries := make([]store.MarketplaceCatalogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, marketplaceCatalogEntryFromRow(row))
	}
	return entries, nil
}

// GetMarketplaceCatalogEntry returns one projection by immutable feed identity.
func (r *MarketplaceRepo) GetMarketplaceCatalogEntry(
	ctx context.Context,
	kind string,
	entryID string,
) (store.MarketplaceCatalogEntry, error) {
	if err := r.checkReady(ctx, "get marketplace catalog entry"); err != nil {
		return store.MarketplaceCatalogEntry{}, err
	}
	row, err := r.queries.GetMarketplaceCatalogEntry(ctx, sqlcgen.GetMarketplaceCatalogEntryParams{
		Kind:    kind,
		EntryID: entryID,
	})
	if err != nil {
		return store.MarketplaceCatalogEntry{}, fmt.Errorf(
			"store: get marketplace catalog %q entry %q: %w",
			kind,
			entryID,
			err,
		)
	}
	return marketplaceCatalogEntryFromRow(row), nil
}

// GetMarketplaceCatalogState returns persisted freshness plus the projected count.
func (r *MarketplaceRepo) GetMarketplaceCatalogState(
	ctx context.Context,
	kind string,
) (store.MarketplaceCatalogState, error) {
	if err := r.checkReady(ctx, "get marketplace catalog state"); err != nil {
		return store.MarketplaceCatalogState{}, err
	}
	row, err := r.queries.GetMarketplaceCatalogState(ctx, kind)
	if err != nil {
		return store.MarketplaceCatalogState{}, fmt.Errorf("store: get marketplace catalog %q state: %w", kind, err)
	}
	return store.MarketplaceCatalogState{
		Kind:            row.Kind,
		ManifestVersion: row.ManifestVersion,
		GeneratedAt:     row.GeneratedAt.String,
		FetchedAt:       row.FetchedAt,
		Stale:           row.Stale != 0,
		LastError:       row.LastError,
		EntryCount:      row.EntryCount,
	}, nil
}

func insertMarketplaceCatalogEntry(
	ctx context.Context,
	queries *sqlcgen.Queries,
	entry store.MarketplaceCatalogEntry,
) error {
	if err := queries.InsertMarketplaceCatalogEntry(ctx, sqlcgen.InsertMarketplaceCatalogEntryParams{
		Kind:         entry.Kind,
		EntryID:      entry.EntryID,
		Name:         entry.Name,
		Description:  entry.Description,
		Version:      entry.Version,
		PublishedAt:  marketplaceCatalogNullString(entry.PublishedAt),
		UpdatedAt:    marketplaceCatalogNullString(entry.UpdatedAt),
		DigestSha256: marketplaceCatalogNullString(entry.DigestSHA256),
		Tier:         marketplaceCatalogNullString(entry.Tier),
		InstallSlug:  marketplaceCatalogNullString(entry.InstallSlug),
		PayloadJson:  entry.PayloadJSON,
		FetchedAt:    entry.FetchedAt,
	}); err != nil {
		return fmt.Errorf("store: insert marketplace catalog %q entry %q: %w", entry.Kind, entry.EntryID, err)
	}
	return nil
}

func marketplaceCatalogEntryFromRow(row sqlcgen.MarketplaceCatalogEntry) store.MarketplaceCatalogEntry {
	return store.MarketplaceCatalogEntry{
		Kind:         row.Kind,
		EntryID:      row.EntryID,
		Name:         row.Name,
		Description:  row.Description,
		Version:      row.Version,
		PublishedAt:  row.PublishedAt.String,
		UpdatedAt:    row.UpdatedAt.String,
		DigestSHA256: row.DigestSha256.String,
		Tier:         row.Tier.String,
		InstallSlug:  row.InstallSlug.String,
		PayloadJSON:  row.PayloadJson,
		FetchedAt:    row.FetchedAt,
	}
}

func marketplaceCatalogNullString(value string) sql.NullString {
	return store.SQLNullString(value)
}
