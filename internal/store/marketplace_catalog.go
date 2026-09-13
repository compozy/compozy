package store

import (
	"context"
	"errors"
)

var ErrMarketplaceCatalogGenerationStale = errors.New("marketplace catalog: source generation is stale")

// MarketplaceCatalogEntry is the storage-boundary projection for one curated entry.
type MarketplaceCatalogEntry struct {
	Source         string
	Layout         string
	Icon           string
	Installable    bool
	InstallBlocker string
	ResolvedRef    string
	EntryID        string
	Name           string
	Description    string
	Version        string
	PublishedAt    string
	UpdatedAt      string
	DigestSHA256   string
	Tier           string
	InstallSlug    string
	PayloadJSON    string
	FetchedAt      string
}

// MarketplaceCatalogReplacement is one validated, atomic source projection.
type MarketplaceCatalogReplacement struct {
	SourceRef       string
	Kind            string
	DocumentDigest  string
	DocumentPath    string
	Owner           string
	DiagnosticsJSON string
	Source          string
	Generation      int64
	Revision        string
	ManifestVersion int64
	GeneratedAt     string
	FetchedAt       string
	Entries         []MarketplaceCatalogEntry
}

// MarketplaceCatalogState is the storage-boundary freshness record for one source.
type MarketplaceCatalogState struct {
	SourceRef       string
	Kind            string
	Enabled         bool
	DocumentDigest  string
	DocumentPath    string
	Owner           string
	DiagnosticsJSON string
	Installable     int64
	ErrorClass      string
	Source          string
	Generation      int64
	Revision        string
	ManifestVersion int64
	GeneratedAt     string
	FetchedAt       string
	Stale           bool
	LastError       string
	EntryCount      int64
}

// MarketplaceCatalogSnapshot keeps content and its revision in one database snapshot.
type MarketplaceCatalogSnapshot struct {
	Entries []MarketplaceCatalogEntry
	State   MarketplaceCatalogState
}

// MarketplaceCatalogRepository owns the global SQLite projection and its transactions.
type MarketplaceCatalogRepository interface {
	ReplaceMarketplaceCatalog(context.Context, MarketplaceCatalogReplacement) error
	ReadMarketplaceCatalogSnapshot(context.Context, string, int64) (MarketplaceCatalogSnapshot, error)
	AdvanceMarketplaceCatalogGeneration(context.Context, string) (int64, error)
	MarkMarketplaceCatalogStale(context.Context, string, int64, string, string) error
	ListMarketplaceCatalogEntries(context.Context, string, int64) ([]MarketplaceCatalogEntry, error)
	GetMarketplaceCatalogEntry(context.Context, string, string) (MarketplaceCatalogEntry, error)
	GetMarketplaceExtensionByInstallSlug(context.Context, string, string) (MarketplaceCatalogEntry, error)
	GetMarketplaceCatalogState(context.Context, string) (MarketplaceCatalogState, error)
}
