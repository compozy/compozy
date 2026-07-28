package task

import "time"

// ClearTaskBlockMutation captures an audited task-block clear.
type ClearTaskBlockMutation struct {
	TaskID    string        `json:"task_id"`
	BlockID   string        `json:"block_id"`
	ClearedBy ActorIdentity `json:"cleared_by"`
	ClearedAt time.Time     `json:"cleared_at"`
	ClearNote string        `json:"clear_note,omitempty"`
	Actor     ActorContext
}

// CreateTaskBlockMutation inserts one task block and evaluates breaker accounting atomically.
type CreateTaskBlockMutation struct {
	Block           TaskBlock `json:"block"`
	RecurrenceLimit int       `json:"recurrence_limit"`
	Actor           ActorContext
}

// BlockMutationResult is the durable result of a task-block creation mutation.
type BlockMutationResult struct {
	Block         TaskBlock       `json:"block"`
	Recurrence    BlockRecurrence `json:"recurrence"`
	EscalatedTask *Task           `json:"escalated_task,omitempty"`
}

// BlockRecurrence stores breaker accounting for one task and block kind.
type BlockRecurrence struct {
	TaskID    string    `json:"task_id"`
	Kind      BlockKind `json:"kind"`
	Count     int       `json:"count"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// NeedsAttention stores task-level escalation metadata.
type NeedsAttention struct {
	Reason string        `json:"reason"`
	At     time.Time     `json:"at"`
	By     ActorIdentity `json:"by"`
}

// NeedsAttentionMutation captures task-level escalation metadata.
type NeedsAttentionMutation struct {
	TaskID   string        `json:"task_id"`
	Reason   string        `json:"reason"`
	Actor    ActorIdentity `json:"actor"`
	MarkedAt time.Time     `json:"marked_at"`
	Origin   Origin        `json:"origin"`
}

// NeedsAttentionClearMutation clears task-level escalation metadata.
type NeedsAttentionClearMutation struct {
	TaskID    string       `json:"task_id"`
	Note      string       `json:"note,omitempty"`
	Actor     ActorContext `json:"actor"`
	ClearedAt time.Time    `json:"cleared_at"`
}

// NeedsAttentionClearResult is the task projection and exact audit record committed by one clear.
type NeedsAttentionClearResult struct {
	Task  Task        `json:"task"`
	Event EventRecord `json:"event"`
}

// WakeCreatorMutation captures the per-task creator-wake opt-in flag.
type WakeCreatorMutation struct {
	TaskID      string    `json:"task_id"`
	WakeCreator bool      `json:"wake_creator"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BlockTaskAndReleaseRunMutation inserts a block and atomically parks the active run.
type BlockTaskAndReleaseRunMutation struct {
	Block           TaskBlock `json:"block"`
	RunID           string    `json:"run_id"`
	ClaimToken      string    `json:"claim_token"`
	Now             time.Time `json:"now"`
	RecurrenceLimit int       `json:"recurrence_limit"`
	Actor           ActorContext
}

// BlockTaskAndReleaseRunResult is the durable result of the atomic block-and-release mutation.
type BlockTaskAndReleaseRunResult struct {
	Block          TaskBlock       `json:"block"`
	Run            Run             `json:"run"`
	Recurrence     BlockRecurrence `json:"recurrence"`
	EscalatedTask  *Task           `json:"escalated_task,omitempty"`
	ReleaseReason  string          `json:"release_reason"`
	PreviousRun    Run             `json:"previous_run"`
	ClaimTokenHash string          `json:"claim_token_hash,omitempty"`
}

// ExpireTaskBlocksMutation finalizes expired transient blocks as daemon-cleared rows.
type ExpireTaskBlocksMutation struct {
	Now       time.Time     `json:"now"`
	ClearedBy ActorIdentity `json:"cleared_by"`
}

// ExpireTaskBlocksResult describes expired transient blocks finalized by a sweep.
type ExpireTaskBlocksResult struct {
	Blocks []TaskBlock `json:"blocks"`
}
