package automation

import (
	"strings"

	taskpkg "github.com/compozy/compozy/internal/task"
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
		ProfileID:   strings.TrimSpace(job.ProfileID),
		Scope:       taskScopeForAutomationScope(job.Scope),
		WorkspaceID: strings.TrimSpace(job.WorkspaceID),
		Title:       title,
		Description: description,
		Owner:       cloneTaskOwnership(job.Task.Owner),
	}
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
