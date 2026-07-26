package contract

import (
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// MemoryDreamPayload is one dreaming runtime record.
type MemoryDreamPayload struct {
	ID             string                `json:"id"`
	Status         MemoryDreamState      `json:"status"`
	Scope          memcontract.Scope     `json:"scope"`
	WorkspaceID    string                `json:"workspace_id,omitempty"`
	AgentName      string                `json:"agent_name,omitempty"`
	AgentTier      memcontract.AgentTier `json:"agent_tier,omitempty"`
	CandidateCount int                   `json:"candidate_count"`
	PromotedCount  int                   `json:"promoted_count"`
	ArtifactPaths  []string              `json:"artifact_paths,omitempty"`
	FailurePath    string                `json:"failure_path,omitempty"`
	FailureReason  string                `json:"failure_reason,omitempty"`
	LockUntil      *time.Time            `json:"lock_until,omitempty"`
	StartedAt      time.Time             `json:"started_at"`
	CompletedAt    *time.Time            `json:"completed_at,omitempty"`
}

// MemoryDreamListResponse wraps dreaming runtime records.
type MemoryDreamListResponse struct {
	Dreams []MemoryDreamPayload `json:"dreams"`
}

// MemoryDreamResponse wraps one dreaming runtime record.
type MemoryDreamResponse struct {
	Dream MemoryDreamPayload `json:"dream"`
}

// MemoryDreamTriggerRequest asks the daemon to run dreaming immediately.
type MemoryDreamTriggerRequest struct {
	Scope       memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	AgentName   string                `json:"agent_name,omitempty"`
	AgentTier   memcontract.AgentTier `json:"agent_tier,omitempty"`
	Force       bool                  `json:"force,omitempty"`
}

// MemoryDreamTriggerResponse reports the requested dreaming run.
type MemoryDreamTriggerResponse struct {
	Dream     MemoryDreamPayload `json:"dream"`
	Triggered bool               `json:"triggered"`
	Reason    string             `json:"reason,omitempty"`
}

// MemoryDreamRetryRequest asks the daemon to retry a failed dreaming run.
type MemoryDreamRetryRequest struct {
	FailureID string `json:"failure_id,omitempty"`
	Force     bool   `json:"force,omitempty"`
}

// MemoryDreamRetryResponse reports the retried dreaming run.
type MemoryDreamRetryResponse struct {
	Dream   MemoryDreamPayload `json:"dream"`
	Retried bool               `json:"retried"`
}

// MemoryDailyLogPayload describes one daily memory log artifact.
type MemoryDailyLogPayload struct {
	Date           string                `json:"date"`
	Scope          memcontract.Scope     `json:"scope"`
	WorkspaceID    string                `json:"workspace_id,omitempty"`
	AgentName      string                `json:"agent_name,omitempty"`
	AgentTier      memcontract.AgentTier `json:"agent_tier,omitempty"`
	Path           string                `json:"path"`
	OperationCount int                   `json:"operation_count"`
}

// MemoryDailyLogListResponse wraps daily memory log artifacts.
type MemoryDailyLogListResponse struct {
	Logs []MemoryDailyLogPayload `json:"logs"`
}

// MemoryExtractorStatusPayload reports extractor queue/runtime status.
type MemoryExtractorStatusPayload struct {
	Status                 MemoryExtractorState `json:"status"`
	QueuedSessions         int                  `json:"queued_sessions"`
	InFlightSessions       int                  `json:"in_flight_sessions"`
	ActiveProviderSessions int                  `json:"active_provider_sessions"`
	DroppedTurns           int                  `json:"dropped_turns"`
	CoalescedTurns         int                  `json:"coalesced_turns"`
	SkippedTurns           int                  `json:"skipped_turns"`
	BackpressuredSessions  int                  `json:"backpressured_sessions"`
	FailureCount           int                  `json:"failure_count"`
}

// MemoryExtractorStatusResponse wraps extractor queue/runtime status.
type MemoryExtractorStatusResponse struct {
	Extractor MemoryExtractorStatusPayload `json:"extractor"`
}

// MemoryExtractorFailurePayload is one redaction-safe extractor DLQ record.
type MemoryExtractorFailurePayload struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	AgentName   string    `json:"agent_name,omitempty"`
	Reason      string    `json:"reason"`
	Path        string    `json:"path"`
	CreatedAt   time.Time `json:"created_at"`
}

// MemoryExtractorFailuresResponse wraps extractor DLQ records.
type MemoryExtractorFailuresResponse struct {
	Failures []MemoryExtractorFailurePayload `json:"failures"`
}

// MemoryExtractorRetryRequest asks the daemon to retry extractor DLQ records.
type MemoryExtractorRetryRequest struct {
	FailureID string `json:"failure_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// MemoryExtractorRetryResponse reports extractor retry results.
type MemoryExtractorRetryResponse struct {
	Retried int `json:"retried"`
	Failed  int `json:"failed"`
}

// MemoryExtractorDrainResponse reports extractor drain completion.
type MemoryExtractorDrainResponse struct {
	DrainedAt time.Time `json:"drained_at"`
	Remaining int       `json:"remaining"`
}
