package hooks

import (
	"encoding/json"
	"strings"

	"time"

	"github.com/compozy/compozy/internal/network/participation"
)

// TaskContext carries task-level identifiers shared across task lifecycle hooks.
type TaskContext struct {
	ProfileID                    string              `json:"profile_id,omitempty"`
	TaskID                       string              `json:"task_id,omitempty"`
	ParentTaskID                 string              `json:"parent_task_id,omitempty"`
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	WorkflowID                   string              `json:"workflow_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	AgentName                    string              `json:"agent_name,omitempty"`
	ActorKind                    string              `json:"actor_kind,omitempty"`
	ActorID                      string              `json:"actor_id,omitempty"`
	OriginKind                   string              `json:"origin_kind,omitempty"`
	OriginRef                    string              `json:"origin_ref,omitempty"`
	TaskStatus                   string              `json:"task_status,omitempty"`
	RunID                        string              `json:"run_id,omitempty"`
	ReleaseReason                string              `json:"release_reason,omitempty"`
	ClaimTokenHash               string              `json:"claim_token_hash,omitempty"`
}

// HookProfileID returns the durable owner used to isolate profile-scoped declarations.
func (c TaskContext) HookProfileID() string { return strings.TrimSpace(c.ProfileID) }

// TaskStatusChangedPayload is delivered after a task status transition is committed.
type TaskStatusChangedPayload struct {
	PayloadBase
	TaskContext
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}

// TaskBlockPayload is shared by task block and unblock hooks.
type TaskBlockPayload struct {
	PayloadBase
	TaskContext
	BlockID   string          `json:"block_id,omitempty"`
	Kind      string          `json:"kind,omitempty"`
	Reason    string          `json:"reason,omitempty"`
	Details   json.RawMessage `json:"details,omitempty"`
	ClearedAt time.Time       `json:"cleared_at,omitzero"`
	ClearNote string          `json:"clear_note,omitempty"`
}

// TaskBlockedPayload is delivered after a task block is committed.
type TaskBlockedPayload = TaskBlockPayload

// TaskUnblockedPayload is delivered after a task block clear is committed.
type TaskUnblockedPayload = TaskBlockPayload

// TaskAttentionPayload is shared by task-level needs-attention lifecycle hooks.
type TaskAttentionPayload struct {
	PayloadBase
	TaskContext
	Reason string    `json:"reason,omitempty"`
	Note   string    `json:"note,omitempty"`
	At     time.Time `json:"at,omitzero"`
}

// TaskNeedsAttentionPayload is delivered after a task escalates to needs_attention.
type TaskNeedsAttentionPayload = TaskAttentionPayload

// TaskRecoveredPayload is delivered after a task leaves needs_attention.
type TaskRecoveredPayload = TaskAttentionPayload

// TaskObservationPatch is the observation patch surface for committed task lifecycle hooks.
type TaskObservationPatch = AutonomyObservationPatch

// LoopContext carries identifiers shared by loop lifecycle hooks.
type LoopContext struct {
	ProfileID                    string              `json:"profile_id,omitempty"`
	LoopRunID                    string              `json:"loop_run_id,omitempty"`
	ParentLoopRunID              string              `json:"parent_loop_run_id,omitempty"`
	WorkspaceID                  string              `json:"workspace_id,omitempty"`
	LoopName                     string              `json:"loop_name,omitempty"`
	Generation                   int                 `json:"generation,omitempty"`
	TaskID                       string              `json:"task_id,omitempty"`
	RunID                        string              `json:"run_id,omitempty"`
	RunKind                      string              `json:"run_kind,omitempty"`
	NodeID                       string              `json:"node_id,omitempty"`
	WorkflowID                   string              `json:"workflow_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec `json:"resolved_network_participation,omitempty"`
	AgentName                    string              `json:"agent_name,omitempty"`
	SessionID                    string              `json:"session_id,omitempty"`
	ActorKind                    string              `json:"actor_kind,omitempty"`
	ActorID                      string              `json:"actor_id,omitempty"`
	OriginKind                   string              `json:"origin_kind,omitempty"`
	OriginRef                    string              `json:"origin_ref,omitempty"`
}

// HookProfileID returns the durable owner used to isolate profile-scoped declarations.
func (c LoopContext) HookProfileID() string { return strings.TrimSpace(c.ProfileID) }

// LoopLifecyclePayload is shared by loop started and terminal events.
type LoopLifecyclePayload struct {
	PayloadBase
	LoopContext
	Status     string          `json:"status,omitempty"`
	Cause      string          `json:"cause,omitempty"`
	ReasonCode string          `json:"reason_code,omitempty"`
	Details    json.RawMessage `json:"details,omitempty"`
}

// LoopStartedPayload is delivered after a loop_run is created.
type LoopStartedPayload = LoopLifecyclePayload

// LoopTerminalPayload is delivered after a loop_run reaches a terminal state.
type LoopTerminalPayload = LoopLifecyclePayload

// LoopGenerationOrigin is the closed provenance vocabulary for generation boundary hooks.
type LoopGenerationOrigin string

const (
	LoopGenerationOriginInitial            LoopGenerationOrigin = "initial"
	LoopGenerationOriginStopWhen           LoopGenerationOrigin = "stop_when"
	LoopGenerationOriginReattempt          LoopGenerationOrigin = "reattempt"
	LoopGenerationOriginGateRevise         LoopGenerationOrigin = "gate_revise"
	LoopGenerationOriginGateNextGeneration LoopGenerationOrigin = "gate_next_generation"
	LoopGenerationOriginDoDRetry           LoopGenerationOrigin = "dod_retry"
	LoopGenerationOriginRatchetRestore     LoopGenerationOrigin = "ratchet_restore"
)

// LoopGenerationOriginValues returns the closed wire vocabulary in declaration order.
func LoopGenerationOriginValues() []string {
	return []string{
		string(LoopGenerationOriginInitial),
		string(LoopGenerationOriginStopWhen),
		string(LoopGenerationOriginReattempt),
		string(LoopGenerationOriginGateRevise),
		string(LoopGenerationOriginGateNextGeneration),
		string(LoopGenerationOriginDoDRetry),
		string(LoopGenerationOriginRatchetRestore),
	}
}

// LoopGenerationPayload is shared by generation boundary hooks.
type LoopGenerationPayload struct {
	PayloadBase
	LoopContext
	// Origin is the closed loop-generation provenance value that explains why this generation exists.
	Origin           LoopGenerationOrigin `json:"origin"`
	ParentGeneration int64                `json:"parent_generation"`
	Status           string               `json:"status,omitempty"`
	ReasonCode       string               `json:"reason_code,omitempty"`
	Details          json.RawMessage      `json:"details,omitempty"`
	Denied           bool                 `json:"denied,omitempty"`
	DenyReason       string               `json:"deny_reason,omitempty"`
}

// LoopGenerationPrePayload is delivered before a loop generation is planned.
type LoopGenerationPrePayload = LoopGenerationPayload

// LoopGenerationPostPayload is delivered after a loop generation plan is produced.
type LoopGenerationPostPayload = LoopGenerationPayload

// LoopGatePayload is shared by gate decision hooks.
type LoopGatePayload struct {
	PayloadBase
	LoopContext
	GateID string `json:"gate_id,omitempty"`
	// Outcome is the machine result already computed when the hook observes the gate.
	Outcome string `json:"outcome,omitempty"`
	// Score is the computed metric score, when the observed gate has a metric criterion.
	Score *float64 `json:"score,omitempty"`
	// BestGeneration is the durable best generation known when the hook observes the result.
	BestGeneration *int64          `json:"best_generation,omitempty"`
	Status         string          `json:"status,omitempty"`
	ReasonCode     string          `json:"reason_code,omitempty"`
	Details        json.RawMessage `json:"details,omitempty"`
	Denied         bool            `json:"denied,omitempty"`
	DenyReason     string          `json:"deny_reason,omitempty"`
}

// LoopGatePrePayload is delivered before a loop gate decision is committed.
type LoopGatePrePayload = LoopGatePayload

// LoopGatePostPayload is delivered after a loop gate decision is committed.
type LoopGatePostPayload = LoopGatePayload

// LoopNodeTerminalPayload is delivered after a loop node task run reaches a terminal state.
type LoopNodeTerminalPayload struct {
	PayloadBase
	LoopContext
	TaskStatus   string          `json:"task_status,omitempty"`
	RunStatus    string          `json:"run_status,omitempty"`
	FailureClass string          `json:"failure_class,omitempty"`
	Disposition  string          `json:"disposition,omitempty"`
	Attempt      int             `json:"attempt,omitempty"`
	Target       string          `json:"target,omitempty"`
	Error        string          `json:"error,omitempty"`
	Details      json.RawMessage `json:"details,omitempty"`
}

// LoopControlPatch denies sync loop control hooks.
type LoopControlPatch struct {
	ControlPatch
}

// LoopGenerationPrePatch denies a generation before planning.
type LoopGenerationPrePatch = LoopControlPatch

// LoopGatePrePatch denies a gate before the decision commits.
type LoopGatePrePatch = LoopControlPatch

// LoopObservationPatch is the observation patch surface for committed loop hooks.
type LoopObservationPatch = AutonomyObservationPatch
