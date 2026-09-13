// Package marketplace owns Compozy-curated discovery feeds and their durable projection.
package marketplace

import (
	"context"
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

const ManifestVersion = 3

const maxCatalogEntriesPerSource = 50_000

const (
	RefreshOutcomeSucceeded = "succeeded"
	RefreshOutcomeFailed    = "failed"
	InstallOutcomeSucceeded = "succeeded"
	InstallOutcomeFailed    = "failed"
	InstallPolicyGatePassed = "passed"
)

// Document is one validated catalog snapshot.
type Document struct {
	SourceRef       string
	SourceKind      string
	DocumentDigest  string
	DocumentPath    string
	Owner           string
	Diagnostics     []pluginsource.Diagnostic
	ManifestVersion int
	GeneratedAt     time.Time
	FetchedAt       time.Time
	Entries         []Entry
}

// Entry is the durable extension projection and payload.
type Entry struct {
	Inputs         []EntryInput
	Diagnostics    []CatalogDiagnostic
	SourceName     string
	Layout         string
	Icon           string
	Installable    bool
	InstallBlocker string
	ResolvedRef    string
	EntryID        string
	Name           string
	Description    string
	Version        string
	PublishedAt    *time.Time
	UpdatedAt      *time.Time
	DigestSHA256   string
	Tier           string
	InstallSlug    string
	Payload        json.RawMessage
	FetchedAt      time.Time
}

// SourceState reports the freshness and failure state for one feed projection.
type SourceState struct {
	SourceRef       string
	Kind            string
	Enabled         bool
	DocumentDigest  string
	DocumentPath    string
	Owner           string
	Diagnostics     []pluginsource.Diagnostic
	Installable     int
	Source          string
	Generation      int64
	Revision        string
	ManifestVersion int
	GeneratedAt     time.Time
	FetchedAt       time.Time
	Stale           bool
	LastError       string
	ErrorClass      string
	EntryCount      int
}

// BrowseResult is one ordered page from the complete enabled source set.
type BrowseResult struct {
	Refreshing bool
	Entries    []Entry
	Total      int
	Revision   string
	Sources    []SourceState
	Stale      bool
	ErrorClass string
	LastError  string
}

type SourcePage struct {
	Entries []Entry
	Total   int
	State   SourceState
}

// SourceBinding ties immutable configuration to its acquisition owner.
type SourceConfiguration struct {
	Generation        int64
	SourceGenerations map[string]int64
}

type SourceBinding struct {
	Config  ResolvedSource
	Fetcher Source
}

// ListResult is one deterministic page from the durable catalog projection.
type ListResult struct {
	Entries []Entry
	Total   int
}

// RefreshOutcome is the canonical per-source refresh result.
type RefreshOutcome struct {
	Source     string `json:"source,omitempty"`
	Generation int64  `json:"generation"`
	Outcome    string `json:"outcome"`
	EntryCount int    `json:"entry_count"`
	Stale      bool   `json:"stale"`
	ErrorClass string `json:"error_class,omitempty"`
}

// InstallOutcome is the redacted canonical observation for one marketplace install attempt.
type InstallOutcome struct {
	Origin      *Origin `json:"origin,omitempty"`
	ResolvedRef string  `json:"resolved_ref,omitempty"`
	EntryID     string  `json:"entry_id"`
	Outcome     string  `json:"outcome"`
	PolicyGate  string  `json:"policy_gate"`
}

// RefreshReport contains deterministic per-source refresh outcomes.
type RefreshReport struct {
	Outcomes []RefreshOutcome `json:"outcomes"`
}

// Source fetches and validates the curated extension feed.
type Source interface {
	Fetch(ctx context.Context) (*Document, error)
}

// Store persists the curated projection and freshness state.
type Store interface {
	PackageDigests(context.Context, []string) (map[string]struct{}, error)
	ConfigureSources(context.Context, []ResolvedSource, string) (SourceConfiguration, error)
	BrowseSources(context.Context, []string, string, int, int) (BrowseResult, error)
	ReplaceSource(ctx context.Context, source string, generation int64, document *Document) error
	MarkSourceStale(ctx context.Context, source string, generation int64, errorClass, lastError string) error
	BrowseSource(ctx context.Context, source, query string, offset, limit int) (SourcePage, error)
	GetEntry(ctx context.Context, source string, entryID string) (*Entry, error)
	GetExtensionByInstallSlug(ctx context.Context, installSlug string, version string) (*Entry, error)
	SourceState(ctx context.Context, source string) (*SourceState, error)
}

// Service exposes internal curated browse, detail, refresh, and status operations.
type Service interface {
	Entry(context.Context, Origin) (*Entry, error)
	Browse(ctx context.Context, query string, offset int, limit int) (BrowseResult, error)
	Detail(ctx context.Context, source, entryID string) (*Entry, error)
	ResolveExtensionInstall(ctx context.Context, installSlug string, version string) (*Entry, error)
	Refresh(ctx context.Context, names ...string) (RefreshReport, error)
	Status(ctx context.Context) ([]SourceState, error)
	Close(ctx context.Context) error
}

// Notifier persists canonical marketplace observations.
type Notifier interface {
	NotifyCatalogRefresh(ctx context.Context, outcome RefreshOutcome) error
	NotifyInstall(ctx context.Context, outcome InstallOutcome) error
}
