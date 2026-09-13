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

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
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
	if err := validateReplacement(document); err != nil {
		return err
	}
	sourceRef := document.SourceRef
	if sourceRef == "" && source == CompozyCatalogSource {
		sourceRef = CompozyCatalogRef
	}
	revision, err := sourceContentRevision(source, sourceRef, document)
	if err != nil {
		return err
	}
	diagnostics := document.Diagnostics
	if diagnostics == nil {
		diagnostics = []pluginsource.Diagnostic{}
	}
	encodedDiagnostics, err := json.Marshal(diagnostics)
	if err != nil {
		return err
	}
	replacement := storepkg.MarketplaceCatalogReplacement{
		SourceRef: sourceRef, Kind: document.SourceKind, DocumentDigest: document.DocumentDigest,
		DocumentPath: document.DocumentPath, Owner: document.Owner, DiagnosticsJSON: string(encodedDiagnostics),
		Source: source, Generation: generation, Revision: revision,
		ManifestVersion: int64(document.ManifestVersion),
		GeneratedAt:     storepkg.FormatNullableTimestamp(document.GeneratedAt),
		FetchedAt:       storepkg.FormatTimestamp(document.FetchedAt),
		Entries:         make([]storepkg.MarketplaceCatalogEntry, 0, len(document.Entries)),
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

func sourceContentRevision(source, sourceRef string, document *Document) (string, error) {
	content := make([]Entry, len(document.Entries))
	copy(content, document.Entries)
	for index := range content {
		content[index].FetchedAt = time.Time{}
		content[index].SourceName = source
		content[index].Installable = content[index].InstallBlocker == ""
	}
	slices.SortFunc(content, func(a, b Entry) int { return strings.Compare(a.EntryID, b.EntryID) })
	bytes, err := json.Marshal(struct {
		Source         string
		SourceRef      string
		DocumentDigest string
		Entries        []Entry
	}{source, sourceRef, document.DocumentDigest, content})
	if err != nil {
		return "", fmt.Errorf("marketplace catalog: encode source revision: %w", err)
	}
	digest := sha256.Sum256(bytes)
	return hex.EncodeToString(digest[:]), nil
}

// BrowseSource binds the filtered page to the revision that owns its rows.
func (s *SQLiteStore) BrowseSource(ctx context.Context, source, query string, offset, limit int) (SourcePage, error) {
	if err := s.checkReady(ctx); err != nil {
		return SourcePage{}, err
	}
	if offset < 0 {
		return SourcePage{}, fmt.Errorf("marketplace catalog: list offset must be non-negative: %d", offset)
	}
	snapshot, err := s.repository.ReadMarketplaceCatalogSnapshot(ctx, source, maxCatalogEntriesPerSource)
	if errors.Is(err, sql.ErrNoRows) {
		return SourcePage{}, fmt.Errorf("%w: %s", ErrSourceStateMissing, source)
	}
	if err != nil {
		return SourcePage{}, err
	}
	page, err := listCatalogRows(snapshot.Entries, query, offset, limit)
	if err != nil {
		return SourcePage{}, err
	}
	state, err := marketplaceSourceStateFromRow(snapshot.State)
	if err != nil {
		return SourcePage{}, err
	}
	return SourcePage{Entries: page.Entries, Total: page.Total, State: state}, nil
}

// MarkSourceStale refuses failure results from an obsolete source generation.
func (s *SQLiteStore) MarkSourceStale(
	ctx context.Context, source string, generation int64, errorClass, lastError string,
) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}
	return s.repository.MarkMarketplaceCatalogStale(ctx, source, generation, errorClass, lastError)
}
