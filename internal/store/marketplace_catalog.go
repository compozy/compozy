package store

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrMarketplaceCatalogGenerationStale = errors.New("marketplace catalog: source generation is stale")
	ErrMarketplaceSourceNameRetained     = errors.New("marketplace_source_name_retained")
)

type MarketplaceSourceNameRetainedError struct {
	Source     string
	RetainedBy []string
}

func (e *MarketplaceSourceNameRetainedError) Error() string {
	return fmt.Sprintf("%s: %s is retained by %v", ErrMarketplaceSourceNameRetained, e.Source, e.RetainedBy)
}

func (e *MarketplaceSourceNameRetainedError) Unwrap() error { return ErrMarketplaceSourceNameRetained }

type MarketplaceSourceDefinition struct {
	ConfigurationRevision string
	Name                  string
	Ref                   string
	Kind                  string
	Enabled               bool
}

type MarketplaceSourceConfiguration struct {
	SourceGenerations map[string]int64
	Generation        int64
	Revision          string
}

// MarketplaceCatalogEntry is the storage-boundary projection for one marketplace source entry.
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
type MarketplaceCatalogSourcesSnapshot struct {
	Generation int64
	Sources    []MarketplaceCatalogSnapshot
}

type MarketplaceCatalogSnapshot struct {
	Entries []MarketplaceCatalogEntry
	State   MarketplaceCatalogState
}

// MarketplaceCatalogRepository owns the global SQLite projection and its transactions.
type MarketplaceCatalogRepository interface {
	ConfigureMarketplaceSources(
		context.Context,
		[]MarketplaceSourceDefinition,
		string,
	) (MarketplaceSourceConfiguration, error)
	ReadMarketplaceCatalogSources(context.Context, []string, int64) (MarketplaceCatalogSourcesSnapshot, error)
	ReplaceMarketplaceCatalog(context.Context, MarketplaceCatalogReplacement) error
	ReadMarketplaceCatalogSnapshot(context.Context, string, int64) (MarketplaceCatalogSnapshot, error)
	MarkMarketplaceCatalogStale(context.Context, string, int64, string, string) error
	ListMarketplaceCatalogEntries(context.Context, string, int64) ([]MarketplaceCatalogEntry, error)
	GetMarketplaceCatalogEntry(context.Context, string, string) (MarketplaceCatalogEntry, error)
	GetMarketplaceExtensionByInstallSlug(context.Context, string, string) (MarketplaceCatalogEntry, error)
	GetMarketplaceCatalogState(context.Context, string) (MarketplaceCatalogState, error)
}
