package modelcatalog

import (
	"context"
	"time"
)

// SourceKind identifies the provenance family for a catalog source row.
type SourceKind string

const (
	// SourceKindBuiltin identifies AGH's offline bootstrap catalog.
	SourceKindBuiltin SourceKind = "builtin"
	// SourceKindConfig identifies operator-authored provider model config.
	SourceKindConfig SourceKind = "config"
	// SourceKindModelsDev identifies enrichment from models.dev.
	SourceKindModelsDev SourceKind = "models_dev"
	// SourceKindProviderLive identifies live provider discovery.
	SourceKindProviderLive SourceKind = "provider_live"
	// SourceKindExtension identifies extension-provided model source rows.
	SourceKindExtension SourceKind = "extension"
	// SourceKindACPSession identifies session-scoped ACP observations.
	SourceKindACPSession SourceKind = "acp_session"
)

const (
	// SourceIDBuiltin is AGH's offline bootstrap catalog source.
	SourceIDBuiltin = "builtin"
	// SourceIDConfig is the operator-authored provider config source.
	SourceIDConfig = "config"
	// SourceIDModelsDev is the models.dev catalog source.
	SourceIDModelsDev = "models_dev"
)

const (
	// PriorityConfig lets explicit operator config win source conflicts.
	PriorityConfig = 120
	// PriorityProviderLive ranks live provider data above extension rows.
	PriorityProviderLive = 110
	// PriorityExtension ranks extension-provided rows above catalog enrichment.
	PriorityExtension = 100
	// PriorityModelsDev ranks models.dev as catalog enrichment.
	PriorityModelsDev = 50
	// PriorityBuiltin ranks offline bootstrap rows last.
	PriorityBuiltin = 10
)

// RefreshState identifies one source refresh lifecycle state.
type RefreshState string

const (
	// RefreshStateIdle indicates a source has no active refresh state.
	RefreshStateIdle RefreshState = "idle"
	// RefreshStateRefreshing indicates a source refresh is currently running.
	RefreshStateRefreshing RefreshState = "refreshing"
	// RefreshStateSucceeded indicates the last source refresh succeeded.
	RefreshStateSucceeded RefreshState = "succeeded"
	// RefreshStateFailed indicates the last source refresh failed.
	RefreshStateFailed RefreshState = "failed"
	// RefreshStateDisabled indicates a configured source is disabled.
	RefreshStateDisabled RefreshState = "disabled"
)

// AvailabilityState identifies how reliable the merged availability signal is.
type AvailabilityState string

const (
	// AvailabilityStateAvailableLive means a fresh live source reports availability.
	AvailabilityStateAvailableLive AvailabilityState = "available_live"
	// AvailabilityStateAvailableStale means a stale live source reports availability.
	AvailabilityStateAvailableStale AvailabilityState = "available_stale"
	// AvailabilityStateUnavailableLive means a fresh live source reports unavailability.
	AvailabilityStateUnavailableLive AvailabilityState = "unavailable_live"
	// AvailabilityStateUnavailableStale means a stale live source reports unavailability.
	AvailabilityStateUnavailableStale AvailabilityState = "unavailable_stale"
	// AvailabilityStateUnknown means no live or extension source reported availability.
	AvailabilityStateUnknown AvailabilityState = "unknown"
)

// CatalogView identifies the public catalog projection to return.
type CatalogView string

const (
	// CatalogViewCurated returns the default operator-facing model set.
	CatalogViewCurated CatalogView = "curated"
	// CatalogViewAll returns every current model, including hidden and deprecated rows.
	CatalogViewAll CatalogView = "all"
)

// ListOptions filters persisted catalog source rows.
type ListOptions struct {
	ProviderID         string
	SourceID           string
	View               CatalogView
	Refresh            bool
	SkipRefreshIfEmpty bool
	IncludeAll         bool
	IncludeStale       bool
	Now                time.Time
}

// RefreshOptions controls a model catalog refresh request.
type RefreshOptions struct {
	ProviderID string
	SourceID   string
	Force      bool
	RequestID  string
	Now        time.Time
}

// ModelRow is one provider/model record contributed by one catalog source.
type ModelRow struct {
	ProviderID               string
	ModelID                  string
	DisplayName              string
	SourceID                 string
	SourceKind               SourceKind
	Priority                 int
	Available                *bool
	Stale                    bool
	RefreshedAt              time.Time
	ExpiresAt                time.Time
	ContextWindow            *int64
	MaxInputTokens           *int64
	MaxOutputTokens          *int64
	SupportsTools            *bool
	SupportsReasoning        *bool
	ReasoningEfforts         []ReasoningEffort
	DefaultReasoningEffort   *ReasoningEffort
	CostInputPerMillion      *float64
	CostOutputPerMillion     *float64
	CostCacheReadPerMillion  *float64
	CostCacheWritePerMillion *float64
	CostReasoningPerMillion  *float64
	ExplicitlyCurated        bool
	Deprecated               *bool
	Hidden                   *bool
	Featured                 *bool
	ReleaseDate              *string
	LastError                string
}

// SourceRef identifies one source participating in a merged catalog projection.
type SourceRef struct {
	SourceID    string
	SourceKind  SourceKind
	Priority    int
	RefreshedAt time.Time
	Stale       bool
	LastError   string
}

// Model is the deterministic merged projection for one provider/model key.
type Model struct {
	ProviderID               string
	ModelID                  string
	DisplayName              string
	Sources                  []SourceRef
	Available                *bool
	AvailabilityState        AvailabilityState
	Stale                    bool
	RefreshedAt              time.Time
	ContextWindow            *int64
	MaxInputTokens           *int64
	MaxOutputTokens          *int64
	SupportsTools            *bool
	SupportsReasoning        *bool
	ReasoningEfforts         []ReasoningEffort
	DefaultReasoningEffort   *ReasoningEffort
	CostInputPerMillion      *float64
	CostOutputPerMillion     *float64
	CostCacheReadPerMillion  *float64
	CostCacheWritePerMillion *float64
	CostReasoningPerMillion  *float64
	CostInputSource          SourceKind
	CostOutputSource         SourceKind
	CostCacheReadSource      SourceKind
	CostCacheWriteSource     SourceKind
	CostReasoningSource      SourceKind
	ExplicitlyCurated        bool
	Curated                  bool
	Deprecated               bool
	Hidden                   bool
	Featured                 bool
	ReleaseDate              *string
	ReasoningSource          ReasoningSource
	LastError                string
}

// SourceStatus reports provider-scoped source health and row counts.
type SourceStatus struct {
	SourceID     string
	SourceKind   SourceKind
	ProviderID   string
	Priority     int
	LastRefresh  time.Time
	NextRefresh  time.Time
	LastSuccess  time.Time
	LastError    string
	RefreshState RefreshState
	RowCount     int
	Stale        bool
}

// Source produces model rows for one catalog source.
type Source interface {
	ID() string
	Kind() SourceKind
	Priority() int
	ListModels(ctx context.Context, opts ListOptions) ([]ModelRow, error)
}

// Store persists source rows and provider-scoped source status.
type Store interface {
	ReplaceSourceRows(
		ctx context.Context,
		sourceID string,
		providerID string,
		rows []ModelRow,
		status SourceStatus,
	) error
	ListRows(ctx context.Context, opts ListOptions) ([]ModelRow, error)
	ListSourceStatus(ctx context.Context, providerID string) ([]SourceStatus, error)
}

// Service exposes merged model catalog projections.
type Service interface {
	ListModels(ctx context.Context, opts ListOptions) ([]Model, error)
	Refresh(ctx context.Context, opts RefreshOptions) ([]SourceStatus, error)
	ListSourceStatus(ctx context.Context, providerID string) ([]SourceStatus, error)
}
