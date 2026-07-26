package contract

import (
	"encoding/json"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskRunPayload is the shared task-run response payload.
type TaskRunPayload struct {
	ID                           string                         `json:"id"`
	TaskID                       string                         `json:"task_id"`
	Status                       taskpkg.RunStatus              `json:"status"`
	Attempt                      int                            `json:"attempt"`
	RecoveryCount                int                            `json:"recovery_count"`
	PreviousRunID                string                         `json:"previous_run_id,omitempty"`
	FailureKind                  string                         `json:"failure_kind,omitempty"`
	ClaimedBy                    *taskpkg.ActorIdentity         `json:"claimed_by,omitempty"`
	SessionID                    string                         `json:"session_id,omitempty"`
	Origin                       taskpkg.Origin                 `json:"origin"`
	IdempotencyKey               string                         `json:"idempotency_key,omitempty"`
	ResolvedNetworkParticipation *participation.Spec            `json:"resolved_network_participation,omitempty"`
	DesignationGroupID           string                         `json:"designation_group_id,omitempty"`
	ClaimTokenHash               string                         `json:"claim_token_hash,omitempty"`
	LeaseUntil                   *time.Time                     `json:"lease_until,omitempty"`
	HeartbeatAt                  *time.Time                     `json:"heartbeat_at,omitempty"`
	CoordinationChannel          *CoordinationChannelPayload    `json:"coordination_channel,omitempty"`
	Designation                  *taskpkg.RunDesignationSummary `json:"designation,omitempty"`
	QueuedAt                     time.Time                      `json:"queued_at"`
	ClaimedAt                    *time.Time                     `json:"claimed_at,omitempty"`
	StartedAt                    *time.Time                     `json:"started_at,omitempty"`
	EndedAt                      *time.Time                     `json:"ended_at,omitempty"`
	Error                        string                         `json:"error,omitempty"`
	Metadata                     json.RawMessage                `json:"metadata,omitempty"`
	Result                       json.RawMessage                `json:"result,omitempty"`
}

// TaskRunSummaryPayload is the shared run-chip payload reused by enriched task reads.
type TaskRunSummaryPayload struct {
	ID                           string                         `json:"id"`
	TaskID                       string                         `json:"task_id"`
	Status                       taskpkg.RunStatus              `json:"status"`
	Attempt                      int                            `json:"attempt"`
	RecoveryCount                int                            `json:"recovery_count"`
	PreviousRunID                string                         `json:"previous_run_id,omitempty"`
	FailureKind                  string                         `json:"failure_kind,omitempty"`
	MaxAttempts                  int                            `json:"max_attempts"`
	SessionID                    string                         `json:"session_id,omitempty"`
	ClaimedBy                    *taskpkg.ActorIdentity         `json:"claimed_by,omitempty"`
	ClaimTokenHash               string                         `json:"claim_token_hash,omitempty"`
	LeaseUntil                   *time.Time                     `json:"lease_until,omitempty"`
	HeartbeatAt                  *time.Time                     `json:"heartbeat_at,omitempty"`
	ResolvedNetworkParticipation *participation.Spec            `json:"resolved_network_participation,omitempty"`
	CoordinationChannel          *CoordinationChannelPayload    `json:"coordination_channel,omitempty"`
	DesignationGroupID           string                         `json:"designation_group_id,omitempty"`
	Designation                  *taskpkg.RunDesignationSummary `json:"designation,omitempty"`
	QueuedAt                     time.Time                      `json:"queued_at"`
	ClaimedAt                    *time.Time                     `json:"claimed_at,omitempty"`
	StartedAt                    *time.Time                     `json:"started_at,omitempty"`
	EndedAt                      *time.Time                     `json:"ended_at,omitempty"`
	Error                        string                         `json:"error,omitempty"`
}
