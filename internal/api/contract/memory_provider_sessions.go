package contract

import (
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// MemoryProviderPayload is one redaction-safe provider registry entry.
type MemoryProviderPayload struct {
	Name          string              `json:"name"`
	Status        MemoryProviderState `json:"status"`
	Active        bool                `json:"active"`
	Builtin       bool                `json:"builtin"`
	Tools         []string            `json:"tools,omitempty"`
	FailureCount  int                 `json:"failure_count"`
	CooldownUntil *time.Time          `json:"cooldown_until,omitempty"`
	LastErrorCode string              `json:"last_error_code,omitempty"`
}

// MemoryProviderListResponse wraps registered memory providers.
type MemoryProviderListResponse struct {
	Providers []MemoryProviderPayload `json:"providers"`
}

// MemoryProviderResponse wraps one memory provider.
type MemoryProviderResponse struct {
	Provider MemoryProviderPayload `json:"provider"`
}

// MemoryProviderSelectRequest selects the active provider by name.
type MemoryProviderSelectRequest struct {
	Name string `json:"name"`
}

// MemoryProviderLifecycleRequest changes a provider lifecycle state.
type MemoryProviderLifecycleRequest struct {
	Reason string `json:"reason,omitempty"`
}

// MemoryProviderLifecycleResponse reports the provider lifecycle state after mutation.
type MemoryProviderLifecycleResponse struct {
	Provider MemoryProviderPayload `json:"provider"`
	Changed  bool                  `json:"changed"`
}

// MemoryAdhocNoteRequest is the only public ad-hoc memory note write surface.
type MemoryAdhocNoteRequest struct {
	Scope       memcontract.Scope     `json:"scope"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	AgentName   string                `json:"agent_name,omitempty"`
	AgentTier   memcontract.AgentTier `json:"agent_tier,omitempty"`
	Content     string                `json:"content"`
	Slug        string                `json:"slug,omitempty"`
}

// MemoryAdhocNoteResponse reports the created ad-hoc note artifact.
type MemoryAdhocNoteResponse struct {
	Path      string    `json:"path"`
	Accepted  bool      `json:"accepted"`
	CreatedAt time.Time `json:"created_at"`
}

// MemoryConfigMetadataResponse exposes Memory v2 settings metadata without secrets.
type MemoryConfigMetadataResponse struct {
	Config       SettingsMemoryConfigPayload `json:"config"`
	MutablePaths []string                    `json:"mutable_paths"`
	LockedPaths  []string                    `json:"locked_paths"`
	Providers    []MemoryProviderPayload     `json:"providers"`
}

// MemorySessionLedgerMetaPayload describes one forensic session ledger projection.
type MemorySessionLedgerMetaPayload struct {
	Version         int        `json:"version"`
	SessionID       string     `json:"session_id"`
	WorkspaceID     string     `json:"workspace_id,omitempty"`
	RootSessionID   string     `json:"root_session_id,omitempty"`
	ParentSessionID string     `json:"parent_session_id,omitempty"`
	SpawnDepth      int        `json:"spawn_depth"`
	Path            string     `json:"path"`
	Checksum        string     `json:"checksum"`
	CreatedAt       time.Time  `json:"created_at"`
	StoppedAt       *time.Time `json:"stopped_at,omitempty"`
}

// MemorySessionLedgerEntryPayload is one JSONL ledger event.
type MemorySessionLedgerEntryPayload struct {
	Sequence  int64          `json:"sequence"`
	EventType string         `json:"event_type"`
	EmittedAt time.Time      `json:"emitted_at"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// MemorySessionLedgerResponse wraps one materialized session ledger.
type MemorySessionLedgerResponse struct {
	Meta   MemorySessionLedgerMetaPayload    `json:"meta"`
	Events []MemorySessionLedgerEntryPayload `json:"events"`
}

// MemorySessionReplayRequest controls deterministic replay output.
type MemorySessionReplayRequest struct {
	IncludeToolEvents bool `json:"include_tool_events,omitempty"`
	IncludeMemory     bool `json:"include_memory,omitempty"`
}

// MemorySessionReplayResponse wraps replayable session ledger events.
type MemorySessionReplayResponse struct {
	SessionID string                            `json:"session_id"`
	Events    []MemorySessionLedgerEntryPayload `json:"events"`
}

// MemorySessionsPruneRequest asks the daemon to prune persisted ledger/session rows.
type MemorySessionsPruneRequest struct {
	OlderThanHours int  `json:"older_than_hours"`
	DryRun         bool `json:"dry_run,omitempty"`
}

// MemorySessionsPruneResponse reports ledger/session prune results.
type MemorySessionsPruneResponse struct {
	PrunedSessions int  `json:"pruned_sessions"`
	PrunedEvents   int  `json:"pruned_events"`
	DryRun         bool `json:"dry_run,omitempty"`
}

// MemorySessionsRepairResponse reports session ledger repair work.
type MemorySessionsRepairResponse struct {
	RepairedLedgers int       `json:"repaired_ledgers"`
	SkippedLedgers  int       `json:"skipped_ledgers"`
	CompletedAt     time.Time `json:"completed_at"`
}
