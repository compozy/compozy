package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	storepkg "github.com/compozy/compozy/internal/store"
)

func (s *SQLiteStore) PackageDigests(ctx context.Context, sources []string) (map[string]struct{}, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	pins := make(map[string]struct{})
	for _, source := range sources {
		rows, err := s.repository.ListMarketplaceCatalogEntries(ctx, source, maxCatalogEntriesPerSource)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.DigestSHA256 != "" {
				pins[row.DigestSHA256] = struct{}{}
			}
		}
	}
	return pins, nil
}

func (s *SQLiteStore) ConfigureSources(
	ctx context.Context,
	sources []ResolvedSource,
	revision string,
) (SourceConfiguration, error) {
	if err := s.checkReady(ctx); err != nil {
		return SourceConfiguration{}, err
	}
	definitions := make([]storepkg.MarketplaceSourceDefinition, 0, len(sources))
	for _, source := range sources {
		definitions = append(
			definitions,
			storepkg.MarketplaceSourceDefinition{ConfigurationRevision: source.Revision,
				Name:    source.Name,
				Ref:     source.Ref,
				Kind:    source.Kind,
				Enabled: source.Enabled,
			},
		)
	}
	configuration, err := s.repository.ConfigureMarketplaceSources(ctx, definitions, revision)
	return SourceConfiguration{
		Generation:        configuration.Generation,
		SourceGenerations: configuration.SourceGenerations,
	}, err
}

func (s *SQLiteStore) BrowseSources(
	ctx context.Context,
	sources []string,
	query string,
	offset, limit int,
) (BrowseResult, error) {
	if err := s.checkReady(ctx); err != nil {
		return BrowseResult{}, err
	}
	if offset < 0 {
		return BrowseResult{}, fmt.Errorf("marketplace catalog: list offset must be non-negative: %d", offset)
	}
	snapshot, err := s.repository.ReadMarketplaceCatalogSources(ctx, sources, maxCatalogEntriesPerSource)
	if err != nil {
		return BrowseResult{}, err
	}
	result := BrowseResult{Entries: []Entry{}, Sources: make([]SourceState, 0, len(snapshot.Sources))}
	remaining := normalizeListLimit(limit)
	type sourceRevision struct {
		Name, Ref, Revision string
		Generation          int64
		Enabled             bool
	}
	revisions := make([]sourceRevision, 0, len(snapshot.Sources))
	for _, sourceSnapshot := range snapshot.Sources {
		state, err := marketplaceSourceStateFromRow(sourceSnapshot.State)
		if err != nil {
			return BrowseResult{}, err
		}
		result.Sources = append(result.Sources, state)
		revisions = append(
			revisions,
			sourceRevision{state.Source, state.SourceRef, state.Revision, state.Generation, state.Enabled},
		)
		if !state.Enabled {
			continue
		}
		page, err := listCatalogRows(sourceSnapshot.Entries, query, offset, max(1, remaining))
		if err != nil {
			return BrowseResult{}, err
		}
		result.Total += page.Total
		offset = max(0, offset-page.Total)
		if remaining > 0 {
			result.Entries = append(result.Entries, page.Entries...)
			remaining -= len(page.Entries)
		}
	}
	encoded, err := json.Marshal(struct {
		Generation int64
		Sources    []sourceRevision
	}{snapshot.Generation, revisions})
	if err != nil {
		return BrowseResult{}, err
	}
	digest := sha256.Sum256(encoded)
	result.Revision = hex.EncodeToString(digest[:])
	return result, nil
}
