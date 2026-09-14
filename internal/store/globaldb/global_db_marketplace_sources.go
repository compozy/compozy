package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ConfigureMarketplaceSources publishes source membership and its fence in one transaction.
func (r *MarketplaceRepo) ConfigureMarketplaceSources(
	ctx context.Context, sources []store.MarketplaceSourceDefinition, revision string,
) (configuration store.MarketplaceSourceConfiguration, err error) {
	if err := r.checkReady(ctx, "configure marketplace sources"); err != nil {
		return configuration, err
	}
	if revision == "" {
		return configuration, errors.New("store: marketplace source configuration revision is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return configuration, err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: roll back marketplace sources: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	previous, err := queries.ClaimMarketplaceSourceConfiguration(ctx)
	if err != nil {
		return configuration, err
	}
	configuration = store.MarketplaceSourceConfiguration{Generation: previous.Generation, Revision: previous.Revision}
	if previous.Revision != revision {
		next, err := queries.SetMarketplaceSourceConfiguration(ctx, revision)
		if err != nil {
			return configuration, err
		}
		configuration = store.MarketplaceSourceConfiguration{Generation: next.Generation, Revision: next.Revision}
		names := make([]string, 0, len(sources))
		for _, source := range sources {
			if err := registerMarketplaceSource(ctx, queries, source, next.Generation); err != nil {
				return configuration, err
			}
			names = append(names, source.Name)
		}
		encodedNames, err := json.Marshal(names)
		if err != nil {
			return configuration, err
		}
		if err := queries.DeleteUnconfiguredMarketplaceEntries(ctx, string(encodedNames)); err != nil {
			return configuration, err
		}
		if err := queries.DeleteUnconfiguredMarketplaceStates(ctx, string(encodedNames)); err != nil {
			return configuration, err
		}
	}
	configuration.SourceGenerations = make(map[string]int64, len(sources))
	for _, source := range sources {
		state, err := queries.GetMarketplaceCatalogState(ctx, source.Name)
		if err != nil {
			return configuration, err
		}
		configuration.SourceGenerations[source.Name] = state.Generation
	}
	if err := tx.Commit(); err != nil {
		return configuration, err
	}
	return configuration, nil
}

func registerMarketplaceSource(
	ctx context.Context, queries *sqlcgen.Queries, source store.MarketplaceSourceDefinition, generation int64,
) error {
	retainers, err := queries.ListMarketplaceSourceNameRetainers(ctx, sqlcgen.ListMarketplaceSourceNameRetainersParams{
		Source: source.Name, SourceRef: source.Ref,
	})
	if err != nil {
		return err
	}
	if len(retainers) != 0 {
		return &store.MarketplaceSourceNameRetainedError{Source: source.Name, RetainedBy: retainers}
	}
	previous, err := queries.GetMarketplaceCatalogState(ctx, source.Name)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && previous.SourceRef != source.Ref {
		if err := queries.DeleteMarketplaceCatalogEntriesBySource(ctx, source.Name); err != nil {
			return err
		}
		if err := queries.DeleteMarketplaceCatalogState(ctx, source.Name); err != nil {
			return err
		}
	}
	return queries.RegisterMarketplaceSource(ctx, sqlcgen.RegisterMarketplaceSourceParams{
		Source:         source.Name,
		SourceRef:      source.Ref,
		ConfigRevision: source.ConfigurationRevision,
		KindOfSource:   source.Kind,
		Enabled:        int64(boolToInt(source.Enabled)),
		Generation:     generation,
	})
}

// ReadMarketplaceCatalogSources binds every selected source to the same SQLite snapshot.
func (r *MarketplaceRepo) ReadMarketplaceCatalogSources(
	ctx context.Context, sources []string, limit int64,
) (snapshot store.MarketplaceCatalogSourcesSnapshot, err error) {
	if err := r.checkReady(ctx, "read marketplace sources"); err != nil {
		return snapshot, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return snapshot, err
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, fmt.Errorf("store: close marketplace source snapshot: %w", rollbackErr))
		}
	}()
	queries := sqlcgen.New(tx)
	configuration, err := queries.GetMarketplaceSourceConfiguration(ctx)
	if err != nil {
		return snapshot, err
	}
	snapshot.Generation = configuration.Generation
	snapshot.Sources = make([]store.MarketplaceCatalogSnapshot, 0, len(sources))
	for _, source := range sources {
		state, err := queries.GetMarketplaceCatalogState(ctx, source)
		if err != nil {
			return snapshot, err
		}
		sourceSnapshot := store.MarketplaceCatalogSnapshot{
			State:   marketplaceCatalogStateFromRow(state),
			Entries: []store.MarketplaceCatalogEntry{},
		}
		if state.Enabled != 0 {
			rows, err := queries.ListMarketplaceCatalogEntries(
				ctx,
				sqlcgen.ListMarketplaceCatalogEntriesParams{Source: source, ResultLimit: limit},
			)
			if err != nil {
				return snapshot, err
			}
			for _, row := range rows {
				sourceSnapshot.Entries = append(sourceSnapshot.Entries, marketplaceCatalogEntryFromRow(row))
			}
		}
		snapshot.Sources = append(snapshot.Sources, sourceSnapshot)
	}
	return snapshot, nil
}
