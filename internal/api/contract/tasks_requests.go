package contract

import (
	"encoding/json"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskRunListQuery captures the shared task-run list filters.
type TaskRunListQuery struct {
	Status               taskpkg.RunStatus `json:"status,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
	ParticipationChannel string            `json:"participation_channel,omitempty"`
	Limit                int               `json:"limit,omitempty"`
}

// TaskTimelineQuery captures the shared task timeline filters.
type TaskTimelineQuery struct {
	AfterSequence int64 `json:"after_sequence,omitempty"`
	Limit         int   `json:"limit,omitempty"`
}

// TaskStreamQuery captures the shared task stream replay filters.
type TaskStreamQuery struct {
	AfterSequence int64 `json:"after_sequence,omitempty"`
}

// TaskDashboardQuery captures the shared observer-backed task dashboard filters.
type TaskDashboardQuery struct {
	Scope                taskpkg.Scope      `json:"scope,omitempty"`
	Workspace            string             `json:"workspace,omitempty"`
	OwnerKind            taskpkg.OwnerKind  `json:"owner_kind,omitempty"`
	OwnerRef             string             `json:"owner_ref,omitempty"`
	ParticipationChannel string             `json:"participation_channel,omitempty"`
	OriginKind           taskpkg.OriginKind `json:"origin_kind,omitempty"`
}

// CreateTaskRequest is the shared task-create request payload.
type CreateTaskRequest struct {
	ID                   string                 `json:"id,omitempty"`
	Identifier           string                 `json:"identifier,omitempty"`
	Scope                taskpkg.Scope          `json:"scope"`
	Workspace            string                 `json:"workspace,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description,omitempty"`
	Priority             taskpkg.Priority       `json:"priority,omitempty"`
	MaxAttempts          *int                   `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady   bool                   `json:"auto_enqueue_on_ready,omitempty"`
	Draft                bool                   `json:"draft,omitempty"`
	ApprovalPolicy       taskpkg.ApprovalPolicy `json:"approval_policy,omitempty"`
	Owner                *taskpkg.Ownership     `json:"owner,omitempty"`
	WakeCreator          *bool                  `json:"wake_creator,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

// CreateTaskChildRequest is the shared child-task create payload.
type CreateTaskChildRequest struct {
	ID                   string                 `json:"id,omitempty"`
	Identifier           string                 `json:"identifier,omitempty"`
	Scope                taskpkg.Scope          `json:"scope"`
	Workspace            string                 `json:"workspace,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Title                string                 `json:"title"`
	Description          string                 `json:"description,omitempty"`
	Priority             taskpkg.Priority       `json:"priority,omitempty"`
	MaxAttempts          *int                   `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady   bool                   `json:"auto_enqueue_on_ready,omitempty"`
	Draft                bool                   `json:"draft,omitempty"`
	ApprovalPolicy       taskpkg.ApprovalPolicy `json:"approval_policy,omitempty"`
	Owner                *taskpkg.Ownership     `json:"owner,omitempty"`
	WakeCreator          *bool                  `json:"wake_creator,omitempty"`
	Metadata             json.RawMessage        `json:"metadata,omitempty"`
}

// UpdateTaskRequest is the shared task patch payload.
type UpdateTaskRequest struct {
	Title                *string                 `json:"title,omitempty"`
	Description          *string                 `json:"description,omitempty"`
	Priority             *taskpkg.Priority       `json:"priority,omitempty"`
	MaxAttempts          *int                    `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady   *bool                   `json:"auto_enqueue_on_ready,omitempty"`
	ApprovalPolicy       *taskpkg.ApprovalPolicy `json:"approval_policy,omitempty"`
	Metadata             *json.RawMessage        `json:"metadata,omitempty"`
	NetworkParticipation *participation.Request  `json:"network_participation,omitempty"`
	Owner                *taskpkg.Ownership      `json:"owner,omitempty"`
	ClearOwner           bool                    `json:"clear_owner,omitempty"`
}

// HasChanges reports whether the patch includes any mutable task field.
func (r UpdateTaskRequest) HasChanges() bool {
	return r.Title != nil ||
		r.Description != nil ||
		r.Priority != nil ||
		r.MaxAttempts != nil ||
		r.AutoEnqueueOnReady != nil ||
		r.ApprovalPolicy != nil ||
		r.Metadata != nil ||
		r.NetworkParticipation != nil ||
		r.Owner != nil ||
		r.ClearOwner
}
