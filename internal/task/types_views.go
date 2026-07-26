package task

import (
	"time"

	"github.com/compozy/agh/internal/network/participation"
)

const maxSummaryCount = int32(1<<31 - 1)

// ClampSummaryCount converts an internal count to the compact Summary count representation.
func ClampSummaryCount(count int) int32 {
	switch {
	case count <= 0:
		return 0
	case count > int(maxSummaryCount):
		return maxSummaryCount
	default:
		return int32(count)
	}
}

// Reference is the human-meaningful task identity used in enriched read models.
type Reference struct {
	ID              string     `json:"id"`
	Identifier      string     `json:"identifier,omitempty"`
	Title           string     `json:"title"`
	Status          Status     `json:"status"`
	Priority        Priority   `json:"priority,omitempty"`
	Owner           *Ownership `json:"owner,omitempty"`
	Scope           Scope      `json:"scope"`
	WorkspaceID     string     `json:"workspace_id,omitempty"`
	LatestEventSeq  int64      `json:"latest_event_seq"`
	Paused          bool       `json:"paused,omitempty"`
	EffectivePaused bool       `json:"effective_paused,omitempty"`
	PausedByTaskID  string     `json:"paused_by_task_id,omitempty"`
}

// DependencyReference enriches one dependency edge with the referenced blocker identity.
type DependencyReference struct {
	TaskID          string         `json:"task_id"`
	DependsOnTaskID string         `json:"depends_on_task_id"`
	Kind            DependencyKind `json:"kind"`
	CreatedAt       time.Time      `json:"created_at"`
	DependsOn       Reference      `json:"depends_on"`
}

// RunSummary captures the operator-facing run chip data used by enriched task cards.
type RunSummary struct {
	ID                           string                 `json:"id"`
	TaskID                       string                 `json:"task_id"`
	RunKind                      RunKind                `json:"run_kind,omitempty"`
	LoopRunID                    string                 `json:"loop_run_id,omitempty"`
	Status                       RunStatus              `json:"status"`
	Attempt                      int                    `json:"attempt"`
	RecoveryCount                int                    `json:"recovery_count"`
	PreviousRunID                string                 `json:"previous_run_id,omitempty"`
	FailureKind                  string                 `json:"failure_kind,omitempty"`
	MaxAttempts                  int                    `json:"max_attempts"`
	SessionID                    string                 `json:"session_id,omitempty"`
	ClaimedBy                    *ActorIdentity         `json:"claimed_by,omitempty"`
	ClaimTokenHash               string                 `json:"claim_token_hash,omitempty"`
	LeaseUntil                   time.Time              `json:"lease_until"`
	HeartbeatAt                  time.Time              `json:"heartbeat_at"`
	ResolvedNetworkParticipation *participation.Spec    `json:"resolved_network_participation"`
	DesignationGroupID           string                 `json:"designation_group_id,omitempty"`
	Designation                  *RunDesignationSummary `json:"designation,omitempty"`
	QueuedAt                     time.Time              `json:"queued_at"`
	ClaimedAt                    time.Time              `json:"claimed_at"`
	StartedAt                    time.Time              `json:"started_at"`
	EndedAt                      time.Time              `json:"ended_at"`
	TokensUsed                   int64                  `json:"tokens_used,omitempty"`
	Error                        string                 `json:"error,omitempty"`
}

// View is the expanded read model returned from single-task lookups.
type View struct {
	Summary              Summary               `json:"summary"`
	Task                 Task                  `json:"task"`
	Children             []Summary             `json:"children,omitempty"`
	Dependencies         []Dependency          `json:"dependencies,omitempty"`
	DependencyReferences []DependencyReference `json:"dependency_references,omitempty"`
	Runs                 []Run                 `json:"runs,omitempty"`
	Events               []Event               `json:"events,omitempty"`
}
