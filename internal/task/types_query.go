package task

import "time"

// RunStarvation is the durable per-run escalation budget the scheduler advances each cycle a
// claimable run stays queued past the starvation threshold. It survives daemon restart so the
// tier ladder resumes from the persisted count rather than restarting on every Rebuild.
type RunStarvation struct {
	RunID            string     `json:"run_id"`
	WakeCount        int        `json:"wake_count"`
	FirstStarvedAt   time.Time  `json:"first_starved_at"`
	LastWakeAt       time.Time  `json:"last_wake_at,omitzero"`
	EscalationTier   int        `json:"escalation_tier"`
	SpawnRequestedAt *time.Time `json:"spawn_requested_at,omitempty"`
	StarvedEventAt   *time.Time `json:"starved_event_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// RunStarvationMutation captures one upsert of a run's escalation budget.
type RunStarvationMutation struct {
	RunID            string     `json:"run_id"`
	WakeCount        int        `json:"wake_count"`
	FirstStarvedAt   time.Time  `json:"first_starved_at"`
	LastWakeAt       time.Time  `json:"last_wake_at,omitzero"`
	EscalationTier   int        `json:"escalation_tier"`
	SpawnRequestedAt *time.Time `json:"spawn_requested_at,omitempty"`
	StarvedEventAt   *time.Time `json:"starved_event_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Query captures the supported list filters for task reads.
type Query struct {
	Scope         Scope         `json:"scope,omitempty"`
	WorkspaceID   string        `json:"workspace_id,omitempty"`
	Status        Status        `json:"status,omitempty"`
	Priority      Priority      `json:"priority,omitempty"`
	ApprovalState ApprovalState `json:"approval_state,omitempty"`
	OwnerKind     OwnerKind     `json:"owner_kind,omitempty"`
	OwnerRef      string        `json:"owner_ref,omitempty"`
	ParentTaskID  string        `json:"parent_task_id,omitempty"`
	Search        string        `json:"search,omitempty"`
	CreatedByKind ActorKind     `json:"created_by_kind,omitempty"`
	CreatedByRef  string        `json:"created_by_ref,omitempty"`
	Limit         int           `json:"limit,omitempty"`
}

// RunQuery captures the supported list filters for task-run reads.
type RunQuery struct {
	TaskID               string    `json:"task_id,omitempty"`
	Status               RunStatus `json:"status,omitempty"`
	SessionID            string    `json:"session_id,omitempty"`
	DesignationGroupID   string    `json:"designation_group_id,omitempty"`
	ParticipationChannel string    `json:"participation_channel,omitempty"`
	Limit                int       `json:"limit,omitempty"`
}

// EventQuery captures the supported list filters for task-event reads.
type EventQuery struct {
	TaskID    string `json:"task_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// StartTaskSession captures the task and run context needed to allocate a dedicated session.
type StartTaskSession struct {
	Task             Task              `json:"task"`
	Run              Run               `json:"run"`
	ExecutionProfile *ExecutionProfile `json:"execution_profile,omitempty"`
	Actor            ActorContext      `json:"actor"`
}

// SessionRef is the task-domain view of a runtime session binding.
type SessionRef struct {
	SessionID   string    `json:"session_id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	StartedAt   time.Time `json:"started_at"`
}

// RunBootRecovery captures one daemon-owned recovery decision for an in-flight
// run discovered during boot reconciliation.
type RunBootRecovery struct {
	Action         RunBootRecoveryAction `json:"action"`
	Reason         string                `json:"reason,omitempty"`
	SessionState   string                `json:"session_state,omitempty"`
	Classification string                `json:"classification,omitempty"`
	Detail         string                `json:"detail,omitempty"`
}
