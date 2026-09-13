package marketplace

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
)

const (
	defaultListLimit = 20
	maxListLimit     = 100
)

// SQLiteStore maps validated catalog domain values to the global repository.
type SQLiteStore struct {
	repository storepkg.MarketplaceCatalogRepository
}

var _ Store = (*SQLiteStore)(nil)

// NewSQLiteStore binds the projection to the global SQLite repository.
func NewSQLiteStore(repository storepkg.MarketplaceCatalogRepository) (*SQLiteStore, error) {
	if repository == nil {
		return nil, errors.New("marketplace catalog: SQLite repository is required")
	}
	return &SQLiteStore{repository: repository}, nil
}

func listCatalogRows(rows []storepkg.MarketplaceCatalogEntry, query string, offset, limit int) (ListResult, error) {
	needle := foldMarketplaceText(NormalizeQuery(query))
	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entry, mapErr := marketplaceEntryFromRow(row)
		if mapErr != nil {
			return ListResult{}, mapErr
		}
		if needle == "" || strings.Contains(foldMarketplaceText(entry.Name), needle) ||
			strings.Contains(foldMarketplaceText(entry.Description), needle) {
			entries = append(entries, entry)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left := foldMarketplaceText(entries[i].Name)
		right := foldMarketplaceText(entries[j].Name)
		if left == right {
			return entries[i].EntryID < entries[j].EntryID
		}
		return left < right
	})
	total := len(entries)
	if offset >= total {
		return ListResult{Entries: []Entry{}, Total: total}, nil
	}
	end := min(offset+normalizeListLimit(limit), total)
	return ListResult{Entries: entries[offset:end], Total: total}, nil
}

// GetEntry returns one projected entry by immutable feed identity.
func (s *SQLiteStore) GetEntry(ctx context.Context, source string, entryID string) (*Entry, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	trimmedID := strings.TrimSpace(entryID)
	if trimmedID == "" {
		return nil, errors.New("marketplace catalog: entry id is required")
	}
	row, err := s.repository.GetMarketplaceCatalogEntry(ctx, source, trimmedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s/%s", ErrEntryNotFound, source, trimmedID)
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: read %q entry %q: %w", source, trimmedID, err)
	}
	entry, err := marketplaceEntryFromRow(row)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// SourceState returns persisted freshness plus the current projected row count.
func (s *SQLiteStore) SourceState(ctx context.Context, source string) (*SourceState, error) {
	if err := s.checkReady(ctx); err != nil {
		return nil, err
	}
	row, err := s.repository.GetMarketplaceCatalogState(ctx, source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrSourceStateMissing, source)
	}
	if err != nil {
		return nil, fmt.Errorf("marketplace catalog: read %q state: %w", source, err)
	}
	state, err := marketplaceSourceStateFromRow(row)
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func marketplaceSourceStateFromRow(row storepkg.MarketplaceCatalogState) (SourceState, error) {
	manifestVersion, err := storedCatalogInt(row.ManifestVersion, "manifest_version")
	if err != nil {
		return SourceState{}, err
	}
	entryCount, err := storedCatalogInt(row.EntryCount, "entry_count")
	if err != nil {
		return SourceState{}, err
	}
	state := SourceState{
		Kind:            Kind(row.Kind),
		Source:          row.Source,
		Generation:      row.Generation,
		Revision:        row.Revision,
		ManifestVersion: manifestVersion,
		Stale:           row.Stale,
		EntryCount:      entryCount,
	}
	if strings.TrimSpace(row.GeneratedAt) != "" {
		parsed, err := storepkg.ParseTimestamp(row.GeneratedAt)
		if err != nil {
			return SourceState{}, fmt.Errorf("marketplace catalog: parse %q generated_at: %w", row.Kind, err)
		}
		state.GeneratedAt = parsed
	}
	if strings.TrimSpace(row.FetchedAt) != "" {
		parsed, err := storepkg.ParseTimestamp(row.FetchedAt)
		if err != nil {
			return SourceState{}, fmt.Errorf("marketplace catalog: parse %q fetched_at: %w", row.Kind, err)
		}
		state.FetchedAt = parsed
	}
	state.ErrorClass, state.LastError = decodeStoredError(row.LastError)
	return state, nil
}

func storedCatalogInt(value int64, field string) (int, error) {
	return storedCatalogIntForSize(value, field, strconv.IntSize)
}

func storedCatalogIntForSize(value int64, field string, intSize int) (int, error) {
	if value < 0 {
		return 0, fmt.Errorf("marketplace catalog: stored %s is negative: %d", field, value)
	}
	if intSize == 32 && value > int64(^uint32(0)>>1) {
		return 0, fmt.Errorf("marketplace catalog: stored %s exceeds int%d: %d", field, intSize, value)
	}
	return int(value), nil
}

func (s *SQLiteStore) checkReady(ctx context.Context) error {
	if ctx == nil {
		return errors.New("marketplace catalog: store context is required")
	}
	if s == nil || s.repository == nil {
		return errors.New("marketplace catalog: SQLite store is required")
	}
	return nil
}

func validateReplacement(kind Kind, document *Document) error {
	if _, err := kindFilename(kind); err != nil {
		return err
	}
	if document == nil {
		return fmt.Errorf("marketplace catalog %q document is required", kind)
	}
	if document.ManifestVersion != ManifestVersion {
		return &UnsupportedManifestVersionError{Kind: kind, Version: document.ManifestVersion}
	}
	if document.GeneratedAt.IsZero() || document.FetchedAt.IsZero() {
		return fmt.Errorf("marketplace catalog %q generated_at and fetched_at are required", kind)
	}
	return validateDocumentEntries(kind, document.Entries)
}

func marketplaceEntryToRow(entry Entry, fetchedAt time.Time) storepkg.MarketplaceCatalogEntry {
	return storepkg.MarketplaceCatalogEntry{
		Kind:           string(entry.Kind),
		Source:         entry.SourceName,
		Layout:         entry.Layout,
		Icon:           entry.Icon,
		Installable:    entry.Installable,
		InstallBlocker: entry.InstallBlocker,
		ResolvedRef:    entry.ResolvedRef,
		EntryID:        strings.TrimSpace(entry.EntryID),
		Name:           strings.TrimSpace(entry.Name),
		Description:    strings.TrimSpace(entry.Description),
		Version:        strings.TrimSpace(entry.Version),
		PublishedAt:    formatOptionalTime(entry.PublishedAt),
		UpdatedAt:      formatOptionalTime(entry.UpdatedAt),
		DigestSHA256:   strings.TrimSpace(entry.DigestSHA256),
		Tier:           strings.TrimSpace(entry.Tier),
		InstallSlug:    strings.TrimSpace(entry.InstallSlug),
		PayloadJSON:    string(entry.Payload),
		FetchedAt:      storepkg.FormatTimestamp(fetchedAt),
	}
}

func marketplaceEntryFromRow(row storepkg.MarketplaceCatalogEntry) (Entry, error) {
	entry := Entry{
		Kind:           Kind(row.Kind),
		SourceName:     row.Source,
		Layout:         row.Layout,
		Icon:           row.Icon,
		Installable:    row.Installable,
		InstallBlocker: row.InstallBlocker,
		ResolvedRef:    row.ResolvedRef,
		EntryID:        row.EntryID,
		Name:           row.Name,
		Description:    row.Description,
		Version:        row.Version,
		DigestSHA256:   row.DigestSHA256,
		Tier:           row.Tier,
		InstallSlug:    row.InstallSlug,
		Payload:        []byte(row.PayloadJSON),
	}
	if strings.TrimSpace(row.PublishedAt) != "" {
		parsed, err := storepkg.ParseTimestamp(row.PublishedAt)
		if err != nil {
			return Entry{}, fmt.Errorf("marketplace catalog: parse entry published_at: %w", err)
		}
		entry.PublishedAt = &parsed
	}
	if strings.TrimSpace(row.UpdatedAt) != "" {
		parsed, err := storepkg.ParseTimestamp(row.UpdatedAt)
		if err != nil {
			return Entry{}, fmt.Errorf("marketplace catalog: parse entry updated_at: %w", err)
		}
		entry.UpdatedAt = &parsed
	}
	parsed, err := storepkg.ParseTimestamp(row.FetchedAt)
	if err != nil {
		return Entry{}, fmt.Errorf("marketplace catalog: parse entry fetched_at: %w", err)
	}
	entry.FetchedAt = parsed
	var payload struct {
		Inputs []EntryInput `json:"inputs"`
	}
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		return Entry{}, fmt.Errorf("marketplace catalog: decode stored inputs: %w", err)
	}
	entry.Inputs = payload.Inputs
	return entry, nil
}

func formatOptionalTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return storepkg.FormatTimestamp(*value)
}

func normalizeListLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > maxListLimit {
		return maxListLimit
	}
	return limit
}

func encodeStoredError(errorClass string, lastError string) string {
	trimmedClass := strings.TrimSpace(errorClass)
	trimmedError := strings.TrimSpace(lastError)
	if trimmedClass == "" {
		return trimmedError
	}
	return "[" + trimmedClass + "] " + trimmedError
}

func decodeStoredError(stored string) (string, string) {
	trimmed := strings.TrimSpace(stored)
	if !strings.HasPrefix(trimmed, "[") {
		return "", trimmed
	}
	end := strings.IndexByte(trimmed, ']')
	if end <= 1 {
		return "", trimmed
	}
	return strings.TrimSpace(trimmed[1:end]), strings.TrimSpace(trimmed[end+1:])
}
