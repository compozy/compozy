package marketplace

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
)

// ReplaceSource commits only to the generation captured before fetching its document.
func (s *SQLiteStore) ReplaceSource(ctx context.Context, source string, generation int64, document *Document) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(source) == "" {
		return fmt.Errorf("marketplace catalog: source is required")
	}
	if err := validateReplacement(KindExtension, document); err != nil {
		return err
	}
	revision, err := sourceContentRevision(source, document.Entries)
	if err != nil {
		return err
	}
	replacement := storepkg.MarketplaceCatalogReplacement{
		Source: source, Generation: generation, Revision: revision,
		Kind: string(KindExtension), ManifestVersion: int64(document.ManifestVersion),
		GeneratedAt: storepkg.FormatNullableTimestamp(document.GeneratedAt),
		FetchedAt:   storepkg.FormatTimestamp(document.FetchedAt),
		Entries:     make([]storepkg.MarketplaceCatalogEntry, 0, len(document.Entries)),
	}
	for _, entry := range document.Entries {
		entry.SourceName = source
		// Curated feed entries have no dry-load blocker; plugin entries carry one explicitly.
		entry.Installable = entry.InstallBlocker == ""
		replacement.Entries = append(replacement.Entries, marketplaceEntryToRow(entry, document.FetchedAt))
	}
	if err := s.repository.ReplaceMarketplaceCatalog(ctx, replacement); err != nil {
		return fmt.Errorf("marketplace catalog: replace source %q: %w", source, err)
	}
	return nil
}

func sourceContentRevision(source string, entries []Entry) (string, error) {
	content := make([]Entry, len(entries))
	copy(content, entries)
	for index := range content {
		content[index].FetchedAt = time.Time{}
		content[index].SourceName = source
		content[index].Installable = content[index].InstallBlocker == ""
	}
	slices.SortFunc(content, func(a, b Entry) int { return strings.Compare(a.EntryID, b.EntryID) })
	bytes, err := json.Marshal(struct {
		Source  string
		Entries []Entry
	}{source, content})
	if err != nil {
		return "", fmt.Errorf("marketplace catalog: encode source revision: %w", err)
	}
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:]), nil
}

// BrowseSource binds the filtered page to the revision that owns its rows.
func (s *SQLiteStore) BrowseSource(ctx context.Context, source, query string, offset, limit int) (BrowseResult, error) {
	if err := s.checkReady(ctx); err != nil {
		return BrowseResult{}, err
	}
	if offset < 0 {
		return BrowseResult{}, fmt.Errorf("marketplace catalog: list offset must be non-negative: %d", offset)
	}
	snapshot, err := s.repository.ReadMarketplaceCatalogSnapshot(ctx, source, maxCatalogEntriesPerKind)
	if errors.Is(err, sql.ErrNoRows) {
		return BrowseResult{}, fmt.Errorf("%w: %s", ErrSourceStateMissing, source)
	}
	if err != nil {
		return BrowseResult{}, err
	}
	page, err := listCatalogRows(snapshot.Entries, query, offset, limit)
	if err != nil {
		return BrowseResult{}, err
	}
	state, err := marketplaceSourceStateFromRow(snapshot.State)
	if err != nil {
		return BrowseResult{}, err
	}
	return BrowseResult{Entries: page.Entries, Total: page.Total, State: state}, nil
}

// MarkSourceStale refuses failure results from an obsolete source generation.
func (s *SQLiteStore) MarkSourceStale(
	ctx context.Context, source string, generation int64, errorClass, lastError string,
) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}
	return s.repository.MarkMarketplaceCatalogStale(ctx, source, generation, encodeStoredError(errorClass, lastError))
}

// AdvanceSourceGeneration invalidates outstanding reads when a source configuration changes.
func (s *SQLiteStore) AdvanceSourceGeneration(ctx context.Context, source string) (int64, error) {
	if err := s.checkReady(ctx); err != nil {
		return 0, err
	}
	return s.repository.AdvanceMarketplaceCatalogGeneration(ctx, source)
}
