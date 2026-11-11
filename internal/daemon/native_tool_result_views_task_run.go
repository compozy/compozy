package daemon

import (
	"encoding/json"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type compactTaskRun struct {
	ID            string                 `json:"id"`
	ProfileID     string                 `json:"profile_id,omitempty"`
	TaskID        string                 `json:"task_id"`
	WorkspaceID   string                 `json:"workspace_id,omitempty"`
	Attempt       int32                  `json:"attempt"`
	RecoveryCount int32                  `json:"recovery_count"`
	RunKind       taskpkg.RunKind        `json:"run_kind,omitzero"`
	Status        taskpkg.RunStatus      `json:"status"`
	LoopRunID     string                 `json:"loop_run_id,omitempty"`
	PreviousRunID string                 `json:"previous_run_id,omitempty"`
	FailureKind   string                 `json:"failure_kind,omitempty"`
	ClaimedBy     *taskpkg.ActorIdentity `json:"claimed_by,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	*taskpkg.RunWorktreeState
	DesignationGroupID string                    `json:"designation_group_id,omitempty"`
	LeaseUntil         time.Time                 `json:"lease_until"`
	HeartbeatAt        time.Time                 `json:"heartbeat_at"`
	Review             *taskpkg.RunReviewLineage `json:"review,omitzero"`
	QueuedAt           time.Time                 `json:"queued_at"`
	ClaimedAt          time.Time                 `json:"claimed_at"`
	StartedAt          time.Time                 `json:"started_at"`
	EndedAt            time.Time                 `json:"ended_at"`
	TokensUsed         int64                     `json:"tokens_used,omitzero"`
	Error              string                    `json:"error,omitempty"`
	*taskpkg.RunResultState
}

func projectCompactTaskRun(p *taskpkg.Run) *compactTaskRun {
	if p == nil {
		return nil
	}
	result := &compactTaskRun{
		ID:                 p.ID,
		ProfileID:          p.ProfileID,
		TaskID:             p.TaskID,
		WorkspaceID:        p.WorkspaceID,
		Attempt:            p.Attempt,
		RecoveryCount:      p.RecoveryCount,
		RunKind:            p.RunKind,
		Status:             p.Status,
		LoopRunID:          p.LoopRunID,
		PreviousRunID:      p.PreviousRunID,
		FailureKind:        p.FailureKind,
		ClaimedBy:          p.ClaimedBy,
		SessionID:          p.SessionID,
		RunWorktreeState:   p.RunWorktreeState,
		DesignationGroupID: p.DesignationGroupID,
		LeaseUntil:         p.LeaseUntil,
		HeartbeatAt:        p.HeartbeatAt,
		Review:             p.Review,
		QueuedAt:           p.QueuedAt,
		ClaimedAt:          p.ClaimedAt,
		StartedAt:          p.StartedAt,
		EndedAt:            p.EndedAt,
		TokensUsed:         p.TokensUsed,
		Error:              p.Error,
		RunResultState:     p.RunResultState,
	}
	return result
}

type compactTaskRunPayload struct {
	ID                   string                        `json:"id"`
	ProfileID            string                        `json:"profile_id,omitempty"`
	TaskID               string                        `json:"task_id"`
	Status               taskpkg.RunStatus             `json:"status"`
	Attempt              int                           `json:"attempt"`
	RecoveryCount        int                           `json:"recovery_count"`
	PreviousRunID        string                        `json:"previous_run_id,omitempty"`
	FailureKind          string                        `json:"failure_kind,omitempty"`
	ClaimedBy            *taskpkg.ActorIdentity        `json:"claimed_by,omitempty"`
	SessionID            string                        `json:"session_id,omitempty"`
	WorktreeID           string                        `json:"worktree_id,omitempty"`
	ResolvedWorktreeMode contract.ResolvedWorktreeMode `json:"resolved_worktree_mode"`
	ResolvedWorktreeRef  string                        `json:"resolved_worktree_ref,omitempty"`
	DesignationGroupID   string                        `json:"designation_group_id,omitempty"`
	LeaseUntil           *time.Time                    `json:"lease_until,omitempty"`
	HeartbeatAt          *time.Time                    `json:"heartbeat_at,omitempty"`
	QueuedAt             time.Time                     `json:"queued_at"`
	ClaimedAt            *time.Time                    `json:"claimed_at,omitempty"`
	StartedAt            *time.Time                    `json:"started_at,omitempty"`
	EndedAt              *time.Time                    `json:"ended_at,omitempty"`
	Error                string                        `json:"error,omitempty"`
	Result               json.RawMessage               `json:"result,omitempty"`
	ResultRef            string                        `json:"result_ref,omitempty"`
	ResultBytes          int64                         `json:"result_bytes,omitempty"`
}

func projectCompactTaskRunPayload(p *contract.TaskRunPayload) *compactTaskRunPayload {
	if p == nil {
		return nil
	}
	result := &compactTaskRunPayload{
		ID:                   p.ID,
		ProfileID:            p.ProfileID,
		TaskID:               p.TaskID,
		Status:               p.Status,
		Attempt:              p.Attempt,
		RecoveryCount:        p.RecoveryCount,
		PreviousRunID:        p.PreviousRunID,
		FailureKind:          p.FailureKind,
		ClaimedBy:            p.ClaimedBy,
		SessionID:            p.SessionID,
		WorktreeID:           p.WorktreeID,
		ResolvedWorktreeMode: p.ResolvedWorktreeMode,
		ResolvedWorktreeRef:  p.ResolvedWorktreeRef,
		DesignationGroupID:   p.DesignationGroupID,
		LeaseUntil:           p.LeaseUntil,
		HeartbeatAt:          p.HeartbeatAt,
		QueuedAt:             p.QueuedAt,
		ClaimedAt:            p.ClaimedAt,
		StartedAt:            p.StartedAt,
		EndedAt:              p.EndedAt,
		Error:                p.Error,
		Result:               p.Result,
		ResultRef:            p.ResultRef,
		ResultBytes:          p.ResultBytes,
	}
	return result
}

type compactTaskRuns struct {
	Runs []compactTaskRun `json:"runs"`
}

func projectCompactTaskRuns(p *taskRunsResult) *compactTaskRuns {
	if p == nil {
		return nil
	}
	result := &compactTaskRuns{}
	if p.Runs != nil {
		result.Runs = make([]compactTaskRun, len(p.Runs))
		for i := range p.Runs {
			result.Runs[i] = *projectCompactTaskRun(&p.Runs[i])
		}
	}
	return result
}

func viewTaskRuns(p taskRunsResult) any { return projectCompactTaskRuns(&p) }

type taskRunsResult struct {
	Runs []taskpkg.Run `json:"runs"`
}

type compactTaskFanout struct {
	DesignationGroupID string           `json:"designation_group_id"`
	Runs               []compactTaskRun `json:"runs"`
}

func projectCompactTaskFanout(p *taskFanoutResult) *compactTaskFanout {
	if p == nil {
		return nil
	}
	result := &compactTaskFanout{
		DesignationGroupID: p.DesignationGroupID,
	}
	if p.Runs != nil {
		result.Runs = make([]compactTaskRun, len(p.Runs))
		for i := range p.Runs {
			result.Runs[i] = *projectCompactTaskRun(&p.Runs[i])
		}
	}
	return result
}

func viewTaskFanout(p taskFanoutResult) any { return projectCompactTaskFanout(&p) }

type taskFanoutResult struct {
	DesignationGroupID string        `json:"designation_group_id"`
	Runs               []taskpkg.Run `json:"runs"`
}
