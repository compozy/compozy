package store

import "context"

// MarketplaceCatalogEntry is the storage-boundary projection for one curated entry.
type MarketplaceCatalogEntry struct {
	Kind         string
	EntryID      string
	Name         string
	Description  string
	Version      string
	PublishedAt  string
	UpdatedAt    string
	DigestSHA256 string
	Tier         string
	InstallSlug  string
	PayloadJSON  string
	FetchedAt    string
}

// MarketplaceCatalogReplacement is one validated, atomic per-kind projection.
type MarketplaceCatalogReplacement struct {
	Kind            string
	ManifestVersion int64
	GeneratedAt     string
	FetchedAt       string
	Entries         []MarketplaceCatalogEntry
}

// MarketplaceCatalogState is the storage-boundary freshness record for one kind.
type MarketplaceCatalogState struct {
	Kind            string
	ManifestVersion int64
	GeneratedAt     string
	FetchedAt       string
	Stale           bool
	LastError       string
	EntryCount      int64
}

// MarketplaceCatalogRepository owns the global SQLite projection and its transactions.
type MarketplaceCatalogRepository interface {
	ReplaceMarketplaceCatalog(context.Context, MarketplaceCatalogReplacement) error
	MarkMarketplaceCatalogStale(context.Context, string, string) error
	ListMarketplaceCatalogEntries(context.Context, string, int64) ([]MarketplaceCatalogEntry, error)
	GetMarketplaceCatalogEntry(context.Context, string, string) (MarketplaceCatalogEntry, error)
	GetMarketplaceExtensionByInstallSlug(context.Context, string, string) (MarketplaceCatalogEntry, error)
	ListMarketplaceSkillsByInstallSlugs(context.Context, []string) ([]MarketplaceCatalogEntry, error)
	GetMarketplaceCatalogState(context.Context, string) (MarketplaceCatalogState, error)
}
