// Package marketplace owns AGH-curated discovery feeds and their durable projection.
package marketplace

import (
	"context"
	"encoding/json"
	"time"
)

const ManifestVersion = 1

// RemoteSkillEntryPrefix reserves the synthetic ID namespace used for registry-only skills.
const RemoteSkillEntryPrefix = "skill_"

const maxCatalogEntriesPerKind = 50_000

const (
	RefreshOutcomeSucceeded = "succeeded"
	RefreshOutcomeFailed    = "failed"
	InstallOutcomeSucceeded = "succeeded"
	InstallOutcomeFailed    = "failed"
	InstallPolicyGatePassed = "passed"
)

// Kind identifies one curated feed document and projection partition.
type Kind string

const (
	KindMCP       Kind = "mcp"
	KindExtension Kind = "extension"
	KindSkill     Kind = "skill"
)

// AllKinds returns the curated kinds in stable display-independent order.
func AllKinds() []Kind {
	return []Kind{KindMCP, KindExtension, KindSkill}
}

// Document is one validated per-kind catalog snapshot.
type Document struct {
	ManifestVersion int
	GeneratedAt     time.Time
	FetchedAt       time.Time
	Entries         []Entry
}

// Entry is the durable common projection plus the kind-specific payload.
type Entry struct {
	Kind         Kind
	EntryID      string
	Name         string
	Description  string
	Version      string
	PublishedAt  *time.Time
	UpdatedAt    *time.Time
	DigestSHA256 string
	Tier         string
	InstallSlug  string
	Payload      json.RawMessage
	FetchedAt    time.Time
}

// KindState reports the freshness and failure state for one feed projection.
type KindState struct {
	Kind            Kind
	ManifestVersion int
	GeneratedAt     time.Time
	FetchedAt       time.Time
	Stale           bool
	LastError       string
	ErrorClass      string
	EntryCount      int
}

// BrowseResult returns projected rows with their truthful freshness state.
type BrowseResult struct {
	Entries []Entry
	Total   int
	State   KindState
}

// ListResult is one deterministic page from the durable catalog projection.
type ListResult struct {
	Entries []Entry
	Total   int
}

// RefreshOutcome is the canonical per-kind refresh result.
type RefreshOutcome struct {
	Kind       Kind   `json:"kind"`
	Outcome    string `json:"outcome"`
	EntryCount int    `json:"entry_count"`
	Stale      bool   `json:"stale"`
	ErrorClass string `json:"error_class,omitempty"`
}

// InstallOutcome is the redacted canonical observation for one marketplace install attempt.
type InstallOutcome struct {
	Kind       Kind   `json:"kind"`
	EntryID    string `json:"entry_id"`
	Outcome    string `json:"outcome"`
	PolicyGate string `json:"policy_gate"`
}

// RefreshReport contains deterministic per-kind refresh outcomes.
type RefreshReport struct {
	Outcomes []RefreshOutcome `json:"outcomes"`
}

// Source fetches and validates one curated kind.
type Source interface {
	Kind() Kind
	Fetch(ctx context.Context) (*Document, error)
}

// Store persists the curated projection and freshness state.
type Store interface {
	ReplaceKind(ctx context.Context, kind Kind, document *Document) error
	MarkKindStale(ctx context.Context, kind Kind, errorClass string, lastError string) error
	ListKind(ctx context.Context, kind Kind, query string, offset int, limit int) (ListResult, error)
	GetEntry(ctx context.Context, kind Kind, entryID string) (*Entry, error)
	GetExtensionByInstallSlug(ctx context.Context, installSlug string, version string) (*Entry, error)
	ListSkillsByInstallSlugs(ctx context.Context, installSlugs []string) ([]Entry, error)
	KindState(ctx context.Context, kind Kind) (*KindState, error)
}

// Service exposes internal curated browse, detail, refresh, and status operations.
type Service interface {
	Browse(ctx context.Context, kind Kind, query string, offset int, limit int) (BrowseResult, error)
	Detail(ctx context.Context, kind Kind, entryID string) (*Entry, error)
	ResolveExtensionInstall(ctx context.Context, installSlug string, version string) (*Entry, error)
	Refresh(ctx context.Context, kinds ...Kind) (RefreshReport, error)
	Status(ctx context.Context) ([]KindState, error)
	Close(ctx context.Context) error
}

// SkillInstallResolver batches curated identity reads for remote skill listings.
type SkillInstallResolver interface {
	ResolveSkillInstalls(ctx context.Context, installSlugs []string) ([]Entry, error)
}

// Notifier persists canonical marketplace observations.
type Notifier interface {
	NotifyCatalogRefresh(ctx context.Context, outcome RefreshOutcome) error
	NotifyInstall(ctx context.Context, outcome InstallOutcome) error
}
