package contract

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskListQuery captures the shared bounded task catalog filters.
type TaskListQuery struct {
	Scope                taskpkg.CatalogScope  `json:"scope,omitempty"`
	Workspace            string                `json:"workspace,omitempty"`
	Status               taskpkg.Status        `json:"status,omitempty"`
	Priority             taskpkg.Priority      `json:"priority,omitempty"`
	IncludeDrafts        bool                  `json:"include_drafts,omitempty"`
	ApprovalState        taskpkg.ApprovalState `json:"approval_state,omitempty"`
	OwnerKind            taskpkg.OwnerKind     `json:"owner_kind,omitempty"`
	OwnerRef             string                `json:"owner_ref,omitempty"`
	ParentTaskID         string                `json:"parent_task_id,omitempty"`
	ParticipationChannel string                `json:"participation_channel,omitempty"`
	Query                string                `json:"query,omitempty"`
	Sort                 taskpkg.CatalogSort   `json:"sort,omitempty"`
	Cursor               string                `json:"cursor,omitempty"`
	Limit                int                   `json:"limit,omitempty"`
}

// TaskCatalogStatusFacetPayload is one exact status count after all filters.
type TaskCatalogStatusFacetPayload struct {
	Status taskpkg.Status `json:"status"`
	Count  int            `json:"count"`
}

// TaskCatalogOwnerFacetPayload is one exact owner count after all filters.
type TaskCatalogOwnerFacetPayload struct {
	Owner taskpkg.Ownership `json:"owner"`
	Count int               `json:"count"`
}

// TaskCatalogFacetsPayload contains exact counts computed before the cursor cut.
type TaskCatalogFacetsPayload struct {
	Statuses []TaskCatalogStatusFacetPayload `json:"statuses"`
	Owners   []TaskCatalogOwnerFacetPayload  `json:"owners"`
}

// TaskCatalogRunPayload is the bounded run summary embedded in catalog and inbox rows.
type TaskCatalogRunPayload struct {
	ID                           string                 `json:"id"`
	TaskID                       string                 `json:"task_id"`
	Status                       taskpkg.RunStatus      `json:"status"`
	Attempt                      int                    `json:"attempt"`
	RecoveryCount                int                    `json:"recovery_count"`
	PreviousRunID                string                 `json:"previous_run_id,omitempty"`
	FailureKind                  string                 `json:"failure_kind,omitempty"`
	MaxAttempts                  int                    `json:"max_attempts"`
	SessionID                    string                 `json:"session_id,omitempty"`
	ClaimedBy                    *taskpkg.ActorIdentity `json:"claimed_by,omitempty"`
	LeaseUntil                   *time.Time             `json:"lease_until,omitempty"`
	HeartbeatAt                  *time.Time             `json:"heartbeat_at,omitempty"`
	ResolvedNetworkParticipation *participation.Spec    `json:"resolved_network_participation,omitempty"`
	QueuedAt                     time.Time              `json:"queued_at"`
	ClaimedAt                    *time.Time             `json:"claimed_at,omitempty"`
	StartedAt                    *time.Time             `json:"started_at,omitempty"`
	EndedAt                      *time.Time             `json:"ended_at,omitempty"`
	Error                        string                 `json:"error,omitempty"`
}

// TaskCatalogItemPayload is one lean task catalog row.
type TaskCatalogItemPayload struct {
	ID                           string                 `json:"id"`
	Identifier                   string                 `json:"identifier,omitempty"`
	Scope                        taskpkg.Scope          `json:"scope"`
	WorkspaceID                  string                 `json:"workspace_id,omitempty"`
	ParentTaskID                 string                 `json:"parent_task_id,omitempty"`
	ResolvedNetworkParticipation *participation.Spec    `json:"resolved_network_participation,omitempty"`
	Title                        string                 `json:"title"`
	Priority                     taskpkg.Priority       `json:"priority,omitempty"`
	MaxAttempts                  int                    `json:"max_attempts,omitempty"`
	AutoEnqueueOnReady           bool                   `json:"auto_enqueue_on_ready,omitempty"`
	Status                       taskpkg.Status         `json:"status"`
	ApprovalPolicy               taskpkg.ApprovalPolicy `json:"approval_policy,omitempty"`
	ApprovalState                taskpkg.ApprovalState  `json:"approval_state,omitempty"`
	Draft                        bool                   `json:"draft,omitempty"`
	Owner                        *taskpkg.Ownership     `json:"owner,omitempty"`
	CurrentRunID                 string                 `json:"current_run_id,omitempty"`
	LatestEventSeq               int64                  `json:"latest_event_seq"`
	NeedsAttention               bool                   `json:"needs_attention,omitempty"`
	NeedsAttentionReason         string                 `json:"needs_attention_reason,omitempty"`
	NeedsAttentionAt             *time.Time             `json:"needs_attention_at,omitempty"`
	NeedsAttentionBy             *taskpkg.ActorIdentity `json:"needs_attention_by,omitempty"`
	WakeCreator                  bool                   `json:"wake_creator"`
	CreatedBy                    taskpkg.ActorIdentity  `json:"created_by"`
	Origin                       taskpkg.Origin         `json:"origin"`
	CreatedAt                    time.Time              `json:"created_at"`
	UpdatedAt                    time.Time              `json:"updated_at"`
	ClosedAt                     *time.Time             `json:"closed_at,omitempty"`
	ChildCount                   int                    `json:"child_count,omitempty"`
	DependencyCount              int                    `json:"dependency_count,omitempty"`
	ActiveRun                    *TaskCatalogRunPayload `json:"active_run,omitempty"`
	LastActivityAt               *time.Time             `json:"last_activity_at,omitempty"`
}

// TaskInboxTaskPayload is the lean task reference embedded in inbox rows.
type TaskInboxTaskPayload struct {
	ID             string             `json:"id"`
	Identifier     string             `json:"identifier,omitempty"`
	Title          string             `json:"title"`
	Status         taskpkg.Status     `json:"status"`
	Priority       taskpkg.Priority   `json:"priority,omitempty"`
	Owner          *taskpkg.Ownership `json:"owner,omitempty"`
	Scope          taskpkg.Scope      `json:"scope"`
	WorkspaceID    string             `json:"workspace_id,omitempty"`
	LatestEventSeq int64              `json:"latest_event_seq"`
}

// TaskCatalogItemPayloadFromSummary converts one bounded domain catalog row.
func TaskCatalogItemPayloadFromSummary(record *taskpkg.Summary) TaskCatalogItemPayload {
	needsAttention := record.NeedsAttention != nil ||
		record.Status.Normalize() == taskpkg.TaskStatusNeedsAttention
	return TaskCatalogItemPayload{
		ID:                           record.ID,
		Identifier:                   record.Identifier,
		Scope:                        record.Scope,
		WorkspaceID:                  record.WorkspaceID,
		ParentTaskID:                 record.ParentTaskID,
		ResolvedNetworkParticipation: taskCatalogResolvedParticipation(record.ActiveRun),
		Title:                        taskpkg.RedactClaimTokens(strings.TrimSpace(record.Title)),
		Priority:                     record.Priority,
		MaxAttempts:                  record.MaxAttempts,
		AutoEnqueueOnReady:           record.AutoEnqueueOnReady,
		Status:                       record.Status,
		ApprovalPolicy:               record.ApprovalPolicy,
		ApprovalState:                record.ApprovalState,
		Draft:                        record.Draft,
		Owner:                        taskCatalogCloneOwnership(record.Owner),
		CurrentRunID:                 record.CurrentRunID,
		LatestEventSeq:               record.LatestEventSeq,
		NeedsAttention:               needsAttention,
		NeedsAttentionReason:         taskCatalogNeedsAttentionReason(record.NeedsAttention),
		NeedsAttentionAt:             taskCatalogNeedsAttentionAt(record.NeedsAttention),
		NeedsAttentionBy:             taskCatalogNeedsAttentionBy(record.NeedsAttention),
		WakeCreator:                  record.WakeCreator,
		CreatedBy:                    record.CreatedBy,
		Origin:                       record.Origin,
		CreatedAt:                    record.CreatedAt,
		UpdatedAt:                    record.UpdatedAt,
		ClosedAt:                     taskCatalogOptionalTime(record.ClosedAt),
		ChildCount:                   int(record.ChildCount),
		DependencyCount:              int(record.DependencyCount),
		ActiveRun:                    TaskCatalogRunPayloadFromSummary(record.ActiveRun),
		LastActivityAt:               taskCatalogOptionalTime(record.LastActivityAt),
	}
}

// TaskInboxTaskPayloadFromReference converts one bounded inbox task reference.
func TaskInboxTaskPayloadFromReference(record taskpkg.Reference) TaskInboxTaskPayload {
	return TaskInboxTaskPayload{
		ID:             record.ID,
		Identifier:     record.Identifier,
		Title:          taskpkg.RedactClaimTokens(strings.TrimSpace(record.Title)),
		Status:         record.Status,
		Priority:       record.Priority,
		Owner:          taskCatalogCloneOwnership(record.Owner),
		Scope:          record.Scope,
		WorkspaceID:    record.WorkspaceID,
		LatestEventSeq: record.LatestEventSeq,
	}
}

// TaskCatalogRunPayloadFromSummary converts one bounded catalog run summary.
func TaskCatalogRunPayloadFromSummary(summary *taskpkg.RunSummary) *TaskCatalogRunPayload {
	if summary == nil {
		return nil
	}
	return &TaskCatalogRunPayload{
		ID:                           summary.ID,
		TaskID:                       summary.TaskID,
		Status:                       summary.Status,
		Attempt:                      summary.Attempt,
		RecoveryCount:                summary.RecoveryCount,
		PreviousRunID:                summary.PreviousRunID,
		FailureKind:                  summary.FailureKind,
		MaxAttempts:                  summary.MaxAttempts,
		SessionID:                    summary.SessionID,
		ClaimedBy:                    taskCatalogCloneActorIdentity(summary.ClaimedBy),
		LeaseUntil:                   taskCatalogOptionalTime(summary.LeaseUntil),
		HeartbeatAt:                  taskCatalogOptionalTime(summary.HeartbeatAt),
		ResolvedNetworkParticipation: taskCatalogResolvedParticipation(summary),
		QueuedAt:                     summary.QueuedAt,
		ClaimedAt:                    taskCatalogOptionalTime(summary.ClaimedAt),
		StartedAt:                    taskCatalogOptionalTime(summary.StartedAt),
		EndedAt:                      taskCatalogOptionalTime(summary.EndedAt),
		Error:                        taskpkg.RedactClaimTokens(strings.TrimSpace(summary.Error)),
	}
}

func taskCatalogResolvedParticipation(summary *taskpkg.RunSummary) *participation.Spec {
	if summary == nil || summary.ResolvedNetworkParticipation == nil {
		return nil
	}
	return participation.CloneSpec(*summary.ResolvedNetworkParticipation)
}

func taskCatalogCloneOwnership(source *taskpkg.Ownership) *taskpkg.Ownership {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}

func taskCatalogCloneActorIdentity(source *taskpkg.ActorIdentity) *taskpkg.ActorIdentity {
	if source == nil {
		return nil
	}
	cloned := *source
	return &cloned
}

func taskCatalogOptionalTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	cloned := value
	return &cloned
}

func taskCatalogNeedsAttentionReason(attention *taskpkg.NeedsAttention) string {
	if attention == nil {
		return ""
	}
	return taskpkg.RedactClaimTokens(strings.TrimSpace(attention.Reason))
}

func taskCatalogNeedsAttentionAt(attention *taskpkg.NeedsAttention) *time.Time {
	if attention == nil {
		return nil
	}
	return taskCatalogOptionalTime(attention.At)
}

func taskCatalogNeedsAttentionBy(attention *taskpkg.NeedsAttention) *taskpkg.ActorIdentity {
	if attention == nil || attention.By.IsZero() {
		return nil
	}
	actor := attention.By
	return &actor
}

// TaskInboxLane identifies one inbox grouping lane.
type TaskInboxLane string

const (
	TaskInboxLaneMyWork     TaskInboxLane = "my_work"
	TaskInboxLaneApprovals  TaskInboxLane = "approvals"
	TaskInboxLaneFailedRuns TaskInboxLane = "failed_runs"
	TaskInboxLaneBlocked    TaskInboxLane = "blocked"
	TaskInboxLaneArchived   TaskInboxLane = "archived"
)

// TaskInboxQuery captures the shared bounded actor-scoped inbox filters.
type TaskInboxQuery struct {
	Scope     taskpkg.CatalogScope `json:"scope,omitempty"`
	Workspace string               `json:"workspace,omitempty"`
	OwnerKind taskpkg.OwnerKind    `json:"owner_kind,omitempty"`
	OwnerRef  string               `json:"owner_ref,omitempty"`
	Lane      TaskInboxLane        `json:"lane,omitempty"`
	Status    taskpkg.Status       `json:"status,omitempty"`
	Priority  taskpkg.Priority     `json:"priority,omitempty"`
	Unread    *bool                `json:"unread,omitempty"`
	Query     string               `json:"query,omitempty"`
	Cursor    string               `json:"cursor,omitempty"`
	Limit     int                  `json:"limit,omitempty"`
}

// TaskInboxItemPayload is one task inbox item with action-ready metadata.
type TaskInboxItemPayload struct {
	Task             TaskInboxTaskPayload   `json:"task"`
	Lane             TaskInboxLane          `json:"lane"`
	ApprovalPolicy   taskpkg.ApprovalPolicy `json:"approval_policy,omitempty"`
	ApprovalState    taskpkg.ApprovalState  `json:"approval_state,omitempty"`
	BlockingReason   string                 `json:"blocking_reason,omitempty"`
	LatestActivityAt time.Time              `json:"latest_activity_at"`
	Run              *TaskCatalogRunPayload `json:"run,omitempty"`
	Triage           TaskTriageStatePayload `json:"triage"`
}

// TaskInboxLaneGroupPayload groups page items and exact filtered lane counts.
type TaskInboxLaneGroupPayload struct {
	Lane        TaskInboxLane          `json:"lane"`
	Count       int                    `json:"count"`
	UnreadCount int                    `json:"unread_count"`
	Items       []TaskInboxItemPayload `json:"items,omitempty"`
}

// TaskInboxStatusFacetPayload is one exact inbox status count.
type TaskInboxStatusFacetPayload struct {
	Status taskpkg.Status `json:"status"`
	Count  int            `json:"count"`
}

// TaskInboxPriorityFacetPayload is one exact inbox priority count.
type TaskInboxPriorityFacetPayload struct {
	Priority taskpkg.Priority `json:"priority"`
	Count    int              `json:"count"`
}

// TaskInboxFacetsPayload contains exact inbox counts before the cursor cut.
type TaskInboxFacetsPayload struct {
	Statuses   []TaskInboxStatusFacetPayload   `json:"statuses"`
	Priorities []TaskInboxPriorityFacetPayload `json:"priorities"`
}

// TaskInboxPayload is the actor-scoped bounded task inbox response payload.
type TaskInboxPayload struct {
	UnreadTotal   int                         `json:"unread_total"`
	ArchivedTotal int                         `json:"archived_total"`
	Groups        []TaskInboxLaneGroupPayload `json:"groups"`
	Page          CountedCursorPagePayload    `json:"page"`
	Facets        TaskInboxFacetsPayload      `json:"facets"`
}
