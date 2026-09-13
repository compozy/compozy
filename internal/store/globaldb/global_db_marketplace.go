package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.MarketplaceCatalogRepository = (*MarketplaceRepo)(nil)

// ReplaceMarketplaceCatalog atomically prunes and replaces one catalog source.
func (r *MarketplaceRepo) ReplaceMarketplaceCatalog(
	ctx context.Context,
	replacement store.MarketplaceCatalogReplacement,
) (err error) {
	if err := r.checkReady(ctx, "replace marketplace catalog"); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin marketplace catalog %q replacement: %w", replacement.Source, err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(
				err,
				fmt.Errorf("store: roll back marketplace catalog %q replacement: %w", replacement.Source, rollbackErr),
			)
		}
	}()

	queries := sqlcgen.New(tx)
	claimed, err := queries.ClaimMarketplaceCatalogGeneration(ctx, sqlcgen.ClaimMarketplaceCatalogGenerationParams{
		Source: replacement.Source, Generation: replacement.Generation,
	})
	if err != nil {
		return fmt.Errorf("store: claim marketplace source generation: %w", err)
	}
	if claimed != 1 {
		return store.ErrMarketplaceCatalogGenerationStale
	}
	if err := queries.DeleteMarketplaceCatalogEntriesBySource(ctx, replacement.Source); err != nil {
		return fmt.Errorf("store: prune marketplace catalog %q projection: %w", replacement.Source, err)
	}
	installable := int64(0)
	for _, entry := range replacement.Entries {
		if entry.Source != replacement.Source {
			return errors.New("store: marketplace entry source does not match replacement")
		}
		if entry.Installable {
			installable++
		}
		if err := insertMarketplaceCatalogEntry(ctx, queries, entry); err != nil {
			return err
		}
	}
	if err := queries.UpsertMarketplaceCatalogStateFresh(ctx, sqlcgen.UpsertMarketplaceCatalogStateFreshParams{
		Source:          replacement.Source,
		Generation:      replacement.Generation,
		Revision:        replacement.Revision,
		Plugins:         int64(len(replacement.Entries)),
		Installable:     installable,
		ManifestVersion: replacement.ManifestVersion,
		GeneratedAt:     marketplaceCatalogNullString(replacement.GeneratedAt),
		FetchedAt:       replacement.FetchedAt,
	}); err != nil {
		return fmt.Errorf("store: upsert marketplace catalog %q state: %w", replacement.Source, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit marketplace catalog %q replacement: %w", replacement.Source, err)
	}
	committed = true
	return nil
}

// MarkMarketplaceCatalogStale records a refresh failure without changing entries.
func (r *MarketplaceRepo) MarkMarketplaceCatalogStale(
	ctx context.Context, source string, generation int64, lastError string,
) error {
	if err := r.checkReady(ctx, "mark marketplace catalog stale"); err != nil {
		return err
	}
	changed, err := r.queries.MarkMarketplaceCatalogStateStale(ctx, sqlcgen.MarkMarketplaceCatalogStateStaleParams{
		Source: source, Generation: generation, LastError: lastError,
	})
	if err != nil {
		return fmt.Errorf("store: mark marketplace catalog %q stale: %w", source, err)
	}
	if changed != 1 {
		return store.ErrMarketplaceCatalogGenerationStale
	}
	return nil
}

// ListMarketplaceCatalogEntries returns deterministic filtered projections.
func (r *MarketplaceRepo) ListMarketplaceCatalogEntries(
	ctx context.Context,
	source string,
	limit int64,
) ([]store.MarketplaceCatalogEntry, error) {
	if err := r.checkReady(ctx, "list marketplace catalog entries"); err != nil {
		return nil, err
	}
	rows, err := r.queries.ListMarketplaceCatalogEntries(ctx, sqlcgen.ListMarketplaceCatalogEntriesParams{
		Source:      source,
		ResultLimit: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list marketplace catalog %q entries: %w", source, err)
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
	source string,
	entryID string,
) (store.MarketplaceCatalogEntry, error) {
	if err := r.checkReady(ctx, "get marketplace catalog entry"); err != nil {
		return store.MarketplaceCatalogEntry{}, err
	}
	row, err := r.queries.GetMarketplaceCatalogEntry(ctx, sqlcgen.GetMarketplaceCatalogEntryParams{
		Source:  source,
		EntryID: entryID,
	})
	if err != nil {
		return store.MarketplaceCatalogEntry{}, fmt.Errorf(
			"store: get marketplace catalog %q entry %q: %w",
			source,
			entryID,
			err,
		)
	}
	return marketplaceCatalogEntryFromRow(row), nil
}

// GetMarketplaceCatalogState returns persisted freshness plus the projected count.
func (r *MarketplaceRepo) GetMarketplaceCatalogState(
	ctx context.Context,
	source string,
) (store.MarketplaceCatalogState, error) {
	if err := r.checkReady(ctx, "get marketplace catalog state"); err != nil {
		return store.MarketplaceCatalogState{}, err
	}
	row, err := r.queries.GetMarketplaceCatalogState(ctx, source)
	if err != nil {
		return store.MarketplaceCatalogState{}, fmt.Errorf("store: get marketplace catalog %q state: %w", source, err)
	}
	return marketplaceCatalogStateFromRow(row), nil
}

func marketplaceCatalogStateFromRow(row sqlcgen.GetMarketplaceCatalogStateRow) store.MarketplaceCatalogState {
	return store.MarketplaceCatalogState{
		Kind:            "extension",
		Source:          row.Source,
		Generation:      row.Generation,
		Revision:        row.Revision,
		ManifestVersion: row.ManifestVersion,
		GeneratedAt:     row.GeneratedAt.String,
		FetchedAt:       row.FetchedAt,
		Stale:           row.Stale != 0,
		LastError:       row.LastError,
		EntryCount:      row.EntryCount,
	}
}

func insertMarketplaceCatalogEntry(
	ctx context.Context,
	queries *sqlcgen.Queries,
	entry store.MarketplaceCatalogEntry,
) error {
	if err := queries.InsertMarketplaceCatalogEntry(ctx, sqlcgen.InsertMarketplaceCatalogEntryParams{
		Source:         entry.Source,
		Layout:         entry.Layout,
		Icon:           entry.Icon,
		Installable:    int64(boolToInt(entry.Installable)),
		InstallBlocker: entry.InstallBlocker,
		ResolvedRef:    entry.ResolvedRef,
		Kind:           entry.Kind,
		EntryID:        entry.EntryID,
		Name:           entry.Name,
		Description:    entry.Description,
		Version:        entry.Version,
		PublishedAt:    marketplaceCatalogNullString(entry.PublishedAt),
		UpdatedAt:      marketplaceCatalogNullString(entry.UpdatedAt),
		DigestSha256:   marketplaceCatalogNullString(entry.DigestSHA256),
		Tier:           marketplaceCatalogNullString(entry.Tier),
		InstallSlug:    marketplaceCatalogNullString(entry.InstallSlug),
		PayloadJson:    entry.PayloadJSON,
		FetchedAt:      entry.FetchedAt,
	}); err != nil {
		return fmt.Errorf("store: insert marketplace catalog %q entry %q: %w", entry.Kind, entry.EntryID, err)
	}
	return nil
}

func marketplaceCatalogEntryFromRow(row sqlcgen.MarketplaceCatalogEntry) store.MarketplaceCatalogEntry {
	return store.MarketplaceCatalogEntry{
		Source:         row.Source,
		Layout:         row.Layout,
		Icon:           row.Icon,
		Installable:    row.Installable != 0,
		InstallBlocker: row.InstallBlocker,
		ResolvedRef:    row.ResolvedRef,
		Kind:           row.Kind,
		EntryID:        row.EntryID,
		Name:           row.Name,
		Description:    row.Description,
		Version:        row.Version,
		PublishedAt:    row.PublishedAt.String,
		UpdatedAt:      row.UpdatedAt.String,
		DigestSHA256:   row.DigestSha256.String,
		Tier:           row.Tier.String,
		InstallSlug:    row.InstallSlug.String,
		PayloadJSON:    row.PayloadJson,
		FetchedAt:      row.FetchedAt,
	}
}

func marketplaceCatalogNullString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func (r *MarketplaceRepo) AdvanceMarketplaceCatalogGeneration(ctx context.Context, source string) (int64, error) {
	if err := r.checkReady(ctx, "advance marketplace source generation"); err != nil {
		return 0, err
	}
	generation, err := r.queries.AdvanceMarketplaceCatalogGeneration(ctx, source)
	if err != nil {
		return 0, fmt.Errorf("store: advance marketplace source generation: %w", err)
	}
	return generation, nil
}
