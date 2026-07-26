package task

import (
	"encoding/json"
	"time"
)

// ActorIdentity is the immutable server-derived actor identity attached to task and run writes.
type ActorIdentity struct {
	Kind ActorKind `json:"kind"`
	Ref  string    `json:"ref"`
}

// TaskBlock is a durable runtime-declared block row for one task.
//
//nolint:revive // TaskBlock is the approved TechSpec contract name.
type TaskBlock struct {
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	TaskID      string          `json:"task_id"`
	Kind        BlockKind       `json:"kind"`
	Reason      string          `json:"reason"`
	Details     json.RawMessage `json:"details,omitempty"`
	CreatedBy   ActorIdentity   `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	ExpiresAt   time.Time       `json:"expires_at,omitzero"`
	ClearedAt   time.Time       `json:"cleared_at,omitzero"`
	ClearedBy   ActorIdentity   `json:"cleared_by,omitzero"`
	ClearNote   string          `json:"clear_note,omitempty"`
}

// BlockRequest captures the service-level request to create a task block.
type BlockRequest struct {
	TaskID     string          `json:"task_id"`
	Kind       BlockKind       `json:"kind"`
	Reason     string          `json:"reason"`
	Details    json.RawMessage `json:"details,omitempty"`
	ExpiresAt  time.Time       `json:"expires_at,omitzero"`
	RunID      string          `json:"run_id,omitempty"`
	ClaimToken string          `json:"claim_token,omitempty"`
}

// BlockedReason is a derived read projection; it is never persisted.
type BlockedReason struct {
	Source           BlockedSource `json:"source"`
	Kind             BlockKind     `json:"kind,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	BlockID          string        `json:"block_id,omitempty"`
	DependsOnTaskIDs []string      `json:"depends_on_task_ids,omitempty"`
}

// Ownership is the optional mutable operational assignee attached to a task.
type Ownership struct {
	Kind OwnerKind `json:"kind"`
	Ref  string    `json:"ref"`
}

// OwnerKindForActor maps an actor identity kind to its operational owner kind so
// actor-scoped reads can filter tasks by the same ownership the inbox applies.
func OwnerKindForActor(kind ActorKind) OwnerKind {
	switch kind.Normalize() {
	case ActorKindHuman:
		return OwnerKindHuman
	case ActorKindAgentSession:
		return OwnerKindAgentSession
	case ActorKindAutomation:
		return OwnerKindAutomation
	case ActorKindExtension:
		return OwnerKindExtension
	case ActorKindNetworkPeer:
		return OwnerKindNetworkPeer
	default:
		return ""
	}
}

// Origin is the immutable technical ingress context attached to task and run writes.
type Origin struct {
	Kind OriginKind `json:"kind"`
	Ref  string     `json:"ref"`
}

// Authority captures the task-domain permissions resolved for one authenticated principal.
type Authority struct {
	Read            bool `json:"read"`
	Write           bool `json:"write"`
	CreateGlobal    bool `json:"create_global"`
	CreateWorkspace bool `json:"create_workspace"`
}

// CallerScope carries daemon-derived caller ownership for scoped operations.
type CallerScope struct {
	SessionID   string `json:"session_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Operator    bool   `json:"operator,omitempty"`
}

// ActorContext carries the authenticated principal, ingress origin, resolved task authority, and caller scope.
type ActorContext struct {
	Actor     ActorIdentity `json:"actor"`
	Origin    Origin        `json:"origin"`
	Authority Authority     `json:"authority"`
	Scope     CallerScope   `json:"scope"`
}

// Task is the durable coordination record owned by the task domain.
type Task struct {
	ID             string         `json:"id"`
	Identifier     string         `json:"identifier,omitempty"`
	Scope          Scope          `json:"scope"`
	WorkspaceID    string         `json:"workspace_id,omitempty"`
	ParentTaskID   string         `json:"parent_task_id,omitempty"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	Priority       Priority       `json:"priority,omitempty"`
	MaxAttempts    int            `json:"max_attempts,omitempty"`
	Status         Status         `json:"status"`
	ApprovalPolicy ApprovalPolicy `json:"approval_policy,omitempty"`
	ApprovalState  ApprovalState  `json:"approval_state,omitempty"`
	Owner          *Ownership     `json:"owner,omitempty"`
	CurrentRunID   string         `json:"current_run_id,omitempty"`
	LatestEventSeq int64          `json:"latest_event_seq"`
	// AutoEnqueueOnReady, when true, makes the runtime enqueue this task's run
	// automatically once its blocking dependencies complete (opt-in; default
	// false preserves the explicit-execution-boundary contract). Clustered with
	// Paused to keep Task within the 512-byte gocritic copy threshold.
	AutoEnqueueOnReady bool            `json:"auto_enqueue_on_ready,omitempty"`
	Paused             bool            `json:"paused,omitempty"`
	WakeCreator        bool            `json:"wake_creator"`
	PausedBy           string          `json:"paused_by,omitempty"`
	PausedAt           time.Time       `json:"paused_at,omitzero"`
	PausedReason       string          `json:"paused_reason,omitempty"`
	NeedsAttention     *NeedsAttention `json:"needs_attention,omitempty"`
	CreatedBy          ActorIdentity   `json:"created_by"`
	Origin             Origin          `json:"origin"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	ClosedAt           time.Time       `json:"closed_at"`
	Metadata           json.RawMessage `json:"metadata,omitempty"`
}
