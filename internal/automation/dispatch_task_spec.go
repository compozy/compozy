package automation

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func directTaskSpec(job *Job, prompt string) taskpkg.CreateTask {
	if job == nil || job.Task == nil {
		return taskpkg.CreateTask{}
	}

	title := strings.TrimSpace(job.Task.Title)
	if title == "" {
		title = strings.TrimSpace(job.Name)
	}
	description := strings.TrimSpace(job.Task.Description)
	if description == "" {
		description = strings.TrimSpace(prompt)
	}
	if description == "" {
		description = strings.TrimSpace(job.Prompt)
	}

	return taskpkg.CreateTask{
		Scope:                taskScopeForAutomationScope(job.Scope),
		WorkspaceID:          strings.TrimSpace(job.WorkspaceID),
		Title:                title,
		Description:          description,
		Owner:                cloneTaskOwnership(job.Task.Owner),
		NetworkParticipation: cloneParticipationRequest(job.Task.NetworkParticipation),
	}
}

func (r DispatchRequest) networkParticipation() *participation.Request {
	var request *participation.Request
	switch {
	case r.Job != nil && r.Job.Task != nil:
		request = r.Job.Task.NetworkParticipation
	case r.loopTarget() != nil:
		request = r.loopTarget().NetworkParticipation
	default:
		return nil
	}
	if request == nil {
		return nil
	}
	normalized, err := participation.NormalizeIntent(*request)
	if err != nil {
		return cloneParticipationRequest(request)
	}
	return &normalized
}

func taskScopeForAutomationScope(scope Scope) taskpkg.Scope {
	switch scope {
	case AutomationScopeWorkspace:
		return taskpkg.ScopeWorkspace
	default:
		return taskpkg.ScopeGlobal
	}
}

func cloneTaskOwnership(owner *taskpkg.Ownership) *taskpkg.Ownership {
	if owner == nil {
		return nil
	}
	cloned := *owner
	return &cloned
}
