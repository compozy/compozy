package daemon

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type compactTaskCatalog struct {
	ID                   string                          `json:"id"`
	ProfileID            string                          `json:"profile_id"`
	Identifier           string                          `json:"identifier,omitempty"`
	Scope                taskpkg.Scope                   `json:"scope"`
	WorkspaceID          string                          `json:"workspace_id,omitempty"`
	ParentTaskID         string                          `json:"parent_task_id,omitempty"`
	Title                string                          `json:"title"`
	Priority             taskpkg.Priority                `json:"priority,omitempty"`
	MaxAttempts          int                             `json:"max_attempts,omitempty"`
	Status               taskpkg.Status                  `json:"status"`
	ApprovalPolicy       taskpkg.ApprovalPolicy          `json:"approval_policy,omitempty"`
	ApprovalState        taskpkg.ApprovalState           `json:"approval_state,omitempty"`
	Draft                bool                            `json:"draft,omitempty"`
	Owner                *taskpkg.Ownership              `json:"owner,omitempty"`
	CurrentRunID         string                          `json:"current_run_id,omitempty"`
	LatestEventSeq       int64                           `json:"latest_event_seq"`
	NeedsAttention       bool                            `json:"needs_attention,omitempty"`
	NeedsAttentionReason string                          `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt     *time.Time                      `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy     *taskpkg.ActorIdentity          `json:"needs_attention_by,omitempty"`
	CreatedAt            time.Time                       `json:"created_at"`
	UpdatedAt            time.Time                       `json:"updated_at"`
	ClosedAt             *time.Time                      `json:"closed_at,omitempty"`
	ChildCount           int                             `json:"child_count,omitempty"`
	DependencyCount      int                             `json:"dependency_count,omitempty"`
	ActiveRun            *contract.TaskCatalogRunPayload `json:"active_run,omitempty"`
	LastActivityAt       *time.Time                      `json:"last_activity_at,omitempty"`
	Loop                 *contract.LoopProvenance        `json:"loop,omitempty"`
}

func projectCompactTaskCatalog(p *contract.TaskCatalogItemPayload) *compactTaskCatalog {
	if p == nil {
		return nil
	}
	result := &compactTaskCatalog{
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
		NeedsAttention:       p.NeedsAttention,
		NeedsAttentionReason: p.NeedsAttentionReason,
		NeedsAttentionAt:     p.NeedsAttentionAt,
		NeedsAttentionBy:     p.NeedsAttentionBy,
		CreatedAt:            p.CreatedAt,
		UpdatedAt:            p.UpdatedAt,
		ClosedAt:             p.ClosedAt,
		ChildCount:           p.ChildCount,
		DependencyCount:      p.DependencyCount,
		ActiveRun:            p.ActiveRun,
		LastActivityAt:       p.LastActivityAt,
		Loop:                 p.Loop,
	}
	return result
}

type compactTaskSummary struct {
	ID                   string                                    `json:"id"`
	ProfileID            string                                    `json:"profile_id"`
	Identifier           string                                    `json:"identifier,omitempty"`
	Scope                taskpkg.Scope                             `json:"scope"`
	WorkspaceID          string                                    `json:"workspace_id,omitempty"`
	ParentTaskID         string                                    `json:"parent_task_id,omitempty"`
	Title                string                                    `json:"title"`
	Priority             taskpkg.Priority                          `json:"priority,omitempty"`
	MaxAttempts          int                                       `json:"max_attempts,omitempty"`
	Status               taskpkg.Status                            `json:"status"`
	ApprovalPolicy       taskpkg.ApprovalPolicy                    `json:"approval_policy,omitempty"`
	ApprovalState        taskpkg.ApprovalState                     `json:"approval_state,omitempty"`
	Draft                bool                                      `json:"draft,omitempty"`
	Owner                *taskpkg.Ownership                        `json:"owner,omitempty"`
	CurrentRunID         string                                    `json:"current_run_id,omitempty"`
	LatestEventSeq       int64                                     `json:"latest_event_seq"`
	Paused               bool                                      `json:"paused,omitempty"`
	PausedBy             string                                    `json:"paused_by,omitempty"`
	PausedAt             *time.Time                                `json:"paused_at,omitempty"`
	PausedReason         string                                    `json:"paused_reason,omitempty"`
	EffectivePaused      bool                                      `json:"effective_paused,omitempty"`
	PausedByTaskID       string                                    `json:"paused_by_task_id,omitempty"`
	BlockedReasons       []taskpkg.BlockedReason                   `json:"blocked_reasons,omitempty"`
	NeedsAttention       bool                                      `json:"needs_attention,omitempty"`
	NeedsAttentionReason string                                    `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt     *time.Time                                `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy     *taskpkg.ActorIdentity                    `json:"needs_attention_by,omitempty"`
	CreatedAt            time.Time                                 `json:"created_at"`
	UpdatedAt            time.Time                                 `json:"updated_at"`
	ClosedAt             *time.Time                                `json:"closed_at,omitempty"`
	ChildCount           int                                       `json:"child_count,omitempty"`
	DependencyCount      int                                       `json:"dependency_count,omitempty"`
	Dependencies         []contract.TaskDependencyReferencePayload `json:"dependencies,omitempty"`
	ActiveRun            *contract.TaskRunSummaryPayload           `json:"active_run,omitempty"`
	LastActivityAt       *time.Time                                `json:"last_activity_at,omitempty"`
}

func projectCompactTaskSummary(p *contract.TaskSummaryPayload) *compactTaskSummary {
	if p == nil {
		return nil
	}
	result := &compactTaskSummary{
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
		ChildCount:           p.ChildCount,
		DependencyCount:      p.DependencyCount,
		Dependencies:         p.Dependencies,
		ActiveRun:            p.ActiveRun,
		LastActivityAt:       p.LastActivityAt,
	}
	return result
}

type compactTaskDetail struct {
	Summary              compactTaskSummary                        `json:"summary"`
	Task                 compactTaskPayload                        `json:"task"`
	Children             []compactTaskSummary                      `json:"children,omitempty"`
	Dependencies         []contract.TaskDependencyPayload          `json:"dependencies,omitempty"`
	DependencyReferences []contract.TaskDependencyReferencePayload `json:"dependency_references,omitempty"`
	Runs                 []compactTaskRunPayload                   `json:"runs,omitempty"`
}

func projectCompactTaskDetail(p *contract.TaskDetailPayload) *compactTaskDetail {
	if p == nil {
		return nil
	}
	result := &compactTaskDetail{
		Dependencies:         p.Dependencies,
		DependencyReferences: p.DependencyReferences,
	}
	result.Summary = *projectCompactTaskSummary(&p.Summary)
	result.Task = *projectCompactTaskPayload(&p.Task)
	if p.Children != nil {
		result.Children = make([]compactTaskSummary, len(p.Children))
		for i := range p.Children {
			result.Children[i] = *projectCompactTaskSummary(&p.Children[i])
		}
	}
	if p.Runs != nil {
		result.Runs = make([]compactTaskRunPayload, len(p.Runs))
		for i := range p.Runs {
			result.Runs[i] = *projectCompactTaskRunPayload(&p.Runs[i])
		}
	}
	return result
}

type compactTaskList struct {
	Tasks  []compactTaskCatalog              `json:"tasks"`
	Page   contract.CountedCursorPagePayload `json:"page"`
	Facets contract.TaskCatalogFacetsPayload `json:"facets"`
}

func projectCompactTaskList(p *contract.TasksResponse) *compactTaskList {
	if p == nil {
		return nil
	}
	result := &compactTaskList{
		Page:   p.Page,
		Facets: p.Facets,
	}
	if p.Tasks != nil {
		result.Tasks = make([]compactTaskCatalog, len(p.Tasks))
		for i := range p.Tasks {
			result.Tasks[i] = *projectCompactTaskCatalog(&p.Tasks[i])
		}
	}
	return result
}

func viewTaskList(p contract.TasksResponse) any { return projectCompactTaskList(&p) }

type compactTaskRead struct {
	Task compactTaskDetail `json:"task"`
}

func projectCompactTaskRead(p *taskReadResult) *compactTaskRead {
	if p == nil {
		return nil
	}
	result := &compactTaskRead{}
	result.Task = *projectCompactTaskDetail(&p.Task)
	return result
}

func viewTaskRead(p *taskReadResult) any { return projectCompactTaskRead(p) }

type taskReadResult struct {
	Task contract.TaskDetailPayload `json:"task"`
}
