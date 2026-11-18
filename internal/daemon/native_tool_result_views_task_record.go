package daemon

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type compactTaskRecord struct {
	ID             string                  `json:"id"`
	ProfileID      string                  `json:"profile_id"`
	Identifier     string                  `json:"identifier,omitempty"`
	Scope          taskpkg.Scope           `json:"scope"`
	WorkspaceID    string                  `json:"workspace_id,omitempty"`
	ParentTaskID   string                  `json:"parent_task_id,omitempty"`
	Title          string                  `json:"title"`
	Priority       taskpkg.Priority        `json:"priority,omitempty"`
	MaxAttempts    int                     `json:"max_attempts,omitzero"`
	Status         taskpkg.Status          `json:"status"`
	ApprovalPolicy taskpkg.ApprovalPolicy  `json:"approval_policy,omitempty"`
	ApprovalState  taskpkg.ApprovalState   `json:"approval_state,omitempty"`
	Owner          *taskpkg.Ownership      `json:"owner,omitempty"`
	CurrentRunID   string                  `json:"current_run_id,omitempty"`
	LatestEventSeq int64                   `json:"latest_event_seq"`
	Paused         bool                    `json:"paused,omitzero"`
	PausedBy       string                  `json:"paused_by,omitempty"`
	PausedAt       time.Time               `json:"paused_at,omitzero"`
	PausedReason   string                  `json:"paused_reason,omitempty"`
	NeedsAttention *taskpkg.NeedsAttention `json:"needs_attention,omitzero"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
	ClosedAt       time.Time               `json:"closed_at"`
}

func projectCompactTaskRecord(p *taskpkg.Task) *compactTaskRecord {
	if p == nil {
		return nil
	}
	result := &compactTaskRecord{
		ID:             p.ID,
		ProfileID:      p.ProfileID,
		Identifier:     p.Identifier,
		Scope:          p.Scope,
		WorkspaceID:    p.WorkspaceID,
		ParentTaskID:   p.ParentTaskID,
		Title:          p.Title,
		Priority:       p.Priority,
		MaxAttempts:    p.MaxAttempts,
		Status:         p.Status,
		ApprovalPolicy: p.ApprovalPolicy,
		ApprovalState:  p.ApprovalState,
		Owner:          p.Owner,
		CurrentRunID:   p.CurrentRunID,
		LatestEventSeq: p.LatestEventSeq,
		Paused:         p.Paused,
		PausedBy:       p.PausedBy,
		PausedAt:       p.PausedAt,
		PausedReason:   p.PausedReason,
		NeedsAttention: p.NeedsAttention,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		ClosedAt:       p.ClosedAt,
	}
	return result
}

type compactTaskPayload struct {
	ID                   string                   `json:"id"`
	ProfileID            string                   `json:"profile_id"`
	Identifier           string                   `json:"identifier,omitempty"`
	Scope                taskpkg.Scope            `json:"scope"`
	WorkspaceID          string                   `json:"workspace_id,omitempty"`
	ParentTaskID         string                   `json:"parent_task_id,omitempty"`
	Title                string                   `json:"title"`
	Description          string                   `json:"description,omitempty"`
	Priority             taskpkg.Priority         `json:"priority,omitempty"`
	MaxAttempts          int                      `json:"max_attempts,omitempty"`
	Status               taskpkg.Status           `json:"status"`
	ApprovalPolicy       taskpkg.ApprovalPolicy   `json:"approval_policy,omitempty"`
	ApprovalState        taskpkg.ApprovalState    `json:"approval_state,omitempty"`
	Draft                bool                     `json:"draft,omitempty"`
	Owner                *taskpkg.Ownership       `json:"owner,omitempty"`
	CurrentRunID         string                   `json:"current_run_id,omitempty"`
	LatestEventSeq       int64                    `json:"latest_event_seq"`
	Paused               bool                     `json:"paused,omitempty"`
	PausedBy             string                   `json:"paused_by,omitempty"`
	PausedAt             *time.Time               `json:"paused_at,omitempty"`
	PausedReason         string                   `json:"paused_reason,omitempty"`
	EffectivePaused      bool                     `json:"effective_paused,omitempty"`
	PausedByTaskID       string                   `json:"paused_by_task_id,omitempty"`
	BlockedReasons       []taskpkg.BlockedReason  `json:"blocked_reasons,omitempty"`
	NeedsAttention       bool                     `json:"needs_attention,omitempty"`
	NeedsAttentionReason string                   `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt     *time.Time               `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy     *taskpkg.ActorIdentity   `json:"needs_attention_by,omitempty"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
	ClosedAt             *time.Time               `json:"closed_at,omitempty"`
	Metadata             json.RawMessage          `json:"metadata,omitempty"`
	Loop                 *contract.LoopProvenance `json:"loop,omitempty"`
}

func projectCompactTaskPayload(p *contract.TaskPayload) *compactTaskPayload {
	if p == nil {
		return nil
	}
	result := &compactTaskPayload{
		ID:                   p.ID,
		ProfileID:            p.ProfileID,
		Identifier:           p.Identifier,
		Scope:                p.Scope,
		WorkspaceID:          p.WorkspaceID,
		ParentTaskID:         p.ParentTaskID,
		Title:                p.Title,
		Description:          p.Description,
		Priority:             p.Priority,
		MaxAttempts:          p.MaxAttempts,
		Status:               p.Status,
		ApprovalPolicy:       p.ApprovalPolicy,
		ApprovalState:        p.ApprovalState,
		Draft:                p.Draft,
		Owner:                p.Owner,
		CurrentRunID:         p.CurrentRunID,
		LatestEventSeq:       p.LatestEventSeq,
		Paused:               p.Paused,
		PausedBy:             p.PausedBy,
		PausedAt:             p.PausedAt,
		PausedReason:         p.PausedReason,
		EffectivePaused:      p.EffectivePaused,
		PausedByTaskID:       p.PausedByTaskID,
		BlockedReasons:       p.BlockedReasons,
		NeedsAttention:       p.NeedsAttention,
		NeedsAttentionReason: p.NeedsAttentionReason,
		NeedsAttentionAt:     p.NeedsAttentionAt,
		NeedsAttentionBy:     p.NeedsAttentionBy,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
		ClosedAt:             p.ClosedAt,
		Metadata:             p.Metadata,
		Loop:                 p.Loop,
	}
	return result
}

type compactTaskRecoverRecord struct {
	ID                   string                   `json:"id"`
	ProfileID            string                   `json:"profile_id"`
	Identifier           string                   `json:"identifier,omitempty"`
	Scope                taskpkg.Scope            `json:"scope"`
	WorkspaceID          string                   `json:"workspace_id,omitempty"`
	ParentTaskID         string                   `json:"parent_task_id,omitempty"`
	Title                string                   `json:"title"`
	Priority             taskpkg.Priority         `json:"priority,omitempty"`
	MaxAttempts          int                      `json:"max_attempts,omitempty"`
	Status               taskpkg.Status           `json:"status"`
	ApprovalPolicy       taskpkg.ApprovalPolicy   `json:"approval_policy,omitempty"`
	ApprovalState        taskpkg.ApprovalState    `json:"approval_state,omitempty"`
	Draft                bool                     `json:"draft,omitempty"`
	Owner                *taskpkg.Ownership       `json:"owner,omitempty"`
	CurrentRunID         string                   `json:"current_run_id,omitempty"`
	LatestEventSeq       int64                    `json:"latest_event_seq"`
	Paused               bool                     `json:"paused,omitempty"`
	PausedBy             string                   `json:"paused_by,omitempty"`
	PausedAt             *time.Time               `json:"paused_at,omitempty"`
	PausedReason         string                   `json:"paused_reason,omitempty"`
	EffectivePaused      bool                     `json:"effective_paused,omitempty"`
	PausedByTaskID       string                   `json:"paused_by_task_id,omitempty"`
	BlockedReasons       []taskpkg.BlockedReason  `json:"blocked_reasons,omitempty"`
	NeedsAttention       bool                     `json:"needs_attention,omitempty"`
	NeedsAttentionReason string                   `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt     *time.Time               `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy     *taskpkg.ActorIdentity   `json:"needs_attention_by,omitempty"`
	CreatedAt            time.Time                `json:"created_at"`
	UpdatedAt            time.Time                `json:"updated_at"`
	ClosedAt             *time.Time               `json:"closed_at,omitempty"`
	Loop                 *contract.LoopProvenance `json:"loop,omitempty"`
}

func projectCompactTaskRecoverRecord(p *contract.TaskPayload) *compactTaskRecoverRecord {
	if p == nil {
		return nil
	}
	result := &compactTaskRecoverRecord{
		ID:                   p.ID,
		ProfileID:            p.ProfileID,
		Identifier:           p.Identifier,
		Scope:                p.Scope,
		WorkspaceID:          p.WorkspaceID,
		ParentTaskID:         p.ParentTaskID,
		Title:                p.Title,
		Priority:             p.Priority,
		MaxAttempts:          p.MaxAttempts,
		Status:               p.Status,
		ApprovalPolicy:       p.ApprovalPolicy,
		ApprovalState:        p.ApprovalState,
		Draft:                p.Draft,
		Owner:                p.Owner,
		CurrentRunID:         p.CurrentRunID,
		LatestEventSeq:       p.LatestEventSeq,
		Paused:               p.Paused,
		PausedBy:             p.PausedBy,
		PausedAt:             p.PausedAt,
		PausedReason:         p.PausedReason,
		EffectivePaused:      p.EffectivePaused,
		PausedByTaskID:       p.PausedByTaskID,
		BlockedReasons:       p.BlockedReasons,
		NeedsAttention:       p.NeedsAttention,
		NeedsAttentionReason: p.NeedsAttentionReason,
		NeedsAttentionAt:     p.NeedsAttentionAt,
		NeedsAttentionBy:     p.NeedsAttentionBy,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
		ClosedAt:             p.ClosedAt,
		Loop:                 p.Loop,
	}
	return result
}

type taskRecordResult struct {
	Task *taskpkg.Task `json:"task"`
}

type compactTaskRecordResult struct {
	Task *compactTaskRecord `json:"task"`
}

func projectCompactTaskRecordResult(p *taskRecordResult) *compactTaskRecordResult {
	if p == nil {
		return nil
	}
	result := &compactTaskRecordResult{}
	result.Task = projectCompactTaskRecord(p.Task)
	return result
}

func viewTaskRecordResult(p taskRecordResult) any { return projectCompactTaskRecordResult(&p) }

type compactTaskRecover struct {
	Task compactTaskRecoverRecord `json:"task"`
}

func projectCompactTaskRecover(p *taskRecoverResult) *compactTaskRecover {
	if p == nil {
		return nil
	}
	result := &compactTaskRecover{}
	result.Task = *projectCompactTaskRecoverRecord(&p.Task)
	return result
}

func viewTaskRecover(p *taskRecoverResult) any { return projectCompactTaskRecover(p) }

type taskRecoverResult struct {
	Task contract.TaskPayload `json:"task"`
}
