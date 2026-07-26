package contract

import (
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

// SchedulerStatusPayload exposes scheduler-wide pause state and queue pressure.
type SchedulerStatusPayload struct {
	Paused                 bool       `json:"paused"`
	PausedBy               string     `json:"paused_by,omitempty"`
	PausedAt               *time.Time `json:"paused_at,omitempty"`
	PausedReason           string     `json:"paused_reason,omitempty"`
	ActiveClaimCount       int        `json:"active_claim_count"`
	QueuedRunCount         int        `json:"queued_run_count"`
	PausedTaskCount        int        `json:"paused_task_count"`
	StarvedRunCount        int        `json:"starved_run_count"`
	NeedsAttentionRunCount int        `json:"needs_attention_run_count"`
	AsOf                   time.Time  `json:"as_of"`
}

// SchedulerStatusResponse wraps scheduler status.
type SchedulerStatusResponse struct {
	Scheduler SchedulerStatusPayload `json:"scheduler"`
}

// SchedulerPauseRequest captures scheduler-wide pause input.
type SchedulerPauseRequest struct {
	Reason string `json:"reason,omitempty"`
}

// SchedulerResumeRequest captures scheduler-wide resume input.
type SchedulerResumeRequest struct {
	Reason string `json:"reason,omitempty"`
}

// SchedulerDrainRequest captures scheduler drain input.
type SchedulerDrainRequest struct {
	Reason         string `json:"reason,omitempty"`
	TimeoutSeconds *int   `json:"timeout_seconds,omitempty"`
}

// SchedulerDrainResponse wraps the final drain result.
type SchedulerDrainResponse struct {
	Scheduler       SchedulerStatusPayload `json:"scheduler"`
	Completed       bool                   `json:"completed"`
	TimedOut        bool                   `json:"timed_out,omitempty"`
	RemainingClaims int                    `json:"remaining_claims"`
	StartedAt       time.Time              `json:"started_at"`
	CompletedAt     time.Time              `json:"completed_at"`
}

// SchedulerBacklogRunPayload exposes one queued run with task identity.
type SchedulerBacklogRunPayload struct {
	Task TaskSummaryPayload `json:"task"`
	Run  TaskRunPayload     `json:"run"`
}

// SchedulerBacklogPayload reports queued scheduler backlog rows.
type SchedulerBacklogPayload struct {
	Runs  []SchedulerBacklogRunPayload `json:"runs"`
	Total int                          `json:"total"`
}

// SchedulerBacklogResponse wraps scheduler backlog.
type SchedulerBacklogResponse struct {
	Backlog SchedulerBacklogPayload `json:"backlog"`
}

// SchedulerBacklogQuery captures transport query filters.
type SchedulerBacklogQuery struct {
	Limit         int           `form:"limit"          json:"limit,omitempty"`
	Scope         taskpkg.Scope `form:"scope"          json:"scope,omitempty"`
	WorkspaceID   string        `form:"workspace"      json:"workspace,omitempty"`
	IncludePaused bool          `form:"include_paused" json:"include_paused,omitempty"`
}

// SchedulerBacklogDomainQuery converts API filters into task-domain filters.
func SchedulerBacklogDomainQuery(query SchedulerBacklogQuery) taskpkg.SchedulerBacklogQuery {
	return taskpkg.SchedulerBacklogQuery{
		Limit:         query.Limit,
		Scope:         query.Scope,
		WorkspaceID:   query.WorkspaceID,
		IncludePaused: query.IncludePaused,
	}
}
