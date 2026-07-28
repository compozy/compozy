package daemon

import (
	"context"
	"errors"
	"sort"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
)

func (n *daemonNativeTools) nativeAutomationJobForWorkspace(
	ctx context.Context,
	workspaceID string,
	jobID string,
) (automationpkg.Job, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	jobID = strings.TrimSpace(jobID)
	if workspaceID == "" {
		return n.automationManager().GetJob(ctx, jobID)
	}
	job, err := n.automationManager().GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrJobNotFound) {
			return automationpkg.Job{}, automationpkg.ErrJobNotFound
		}
		return automationpkg.Job{}, err
	}
	if job.Scope != automationpkg.AutomationScopeWorkspace ||
		strings.TrimSpace(job.WorkspaceID) != workspaceID {
		return automationpkg.Job{}, automationpkg.ErrJobNotFound
	}
	return job, nil
}

func (n *daemonNativeTools) nativeAutomationJobsForWorkspace(
	ctx context.Context,
	workspaceID string,
) ([]automationpkg.Job, error) {
	query := automationpkg.JobListQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: strings.TrimSpace(workspaceID),
		Limit:       automationpkg.MaxListLimit,
	}
	jobs := make([]automationpkg.Job, 0)
	for {
		page, err := n.automationManager().ListJobs(ctx, query)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, page.Jobs...)
		nextCursor := strings.TrimSpace(page.NextCursor)
		if !page.HasMore || nextCursor == "" || nextCursor == strings.TrimSpace(query.Cursor) {
			return jobs, nil
		}
		query.Cursor = nextCursor
	}
}

func (n *daemonNativeTools) nativeAutomationTriggerForWorkspace(
	ctx context.Context,
	workspaceID string,
	triggerID string,
) (automationpkg.Trigger, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	triggerID = strings.TrimSpace(triggerID)
	if workspaceID == "" {
		return n.automationManager().GetTrigger(ctx, triggerID)
	}
	trigger, err := n.automationManager().GetTrigger(ctx, triggerID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrTriggerNotFound) {
			return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
		}
		return automationpkg.Trigger{}, err
	}
	if trigger.Scope != automationpkg.AutomationScopeWorkspace ||
		strings.TrimSpace(trigger.WorkspaceID) != workspaceID {
		return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
	}
	return trigger, nil
}

func (n *daemonNativeTools) nativeAutomationTriggersForWorkspace(
	ctx context.Context,
	workspaceID string,
) ([]automationpkg.Trigger, error) {
	query := automationpkg.TriggerListQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: strings.TrimSpace(workspaceID),
		Limit:       automationpkg.MaxListLimit,
	}
	triggers := make([]automationpkg.Trigger, 0)
	for {
		page, err := n.automationManager().ListTriggers(ctx, query)
		if err != nil {
			return nil, err
		}
		triggers = append(triggers, page.Triggers...)
		nextCursor := strings.TrimSpace(page.NextCursor)
		if !page.HasMore || nextCursor == "" || nextCursor == strings.TrimSpace(query.Cursor) {
			return triggers, nil
		}
		query.Cursor = nextCursor
	}
}

func (n *daemonNativeTools) nativeAutomationRunForWorkspace(
	ctx context.Context,
	workspaceID string,
	runID string,
) (automationpkg.Run, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	runID = strings.TrimSpace(runID)
	if workspaceID == "" {
		return n.automationManager().GetRun(ctx, runID)
	}
	run, err := n.automationManager().GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrRunNotFound) {
			return automationpkg.Run{}, automationpkg.ErrRunNotFound
		}
		return automationpkg.Run{}, err
	}
	hasOwnedTarget := false
	if jobID := strings.TrimSpace(run.JobID); jobID != "" {
		hasOwnedTarget = true
		if _, err := n.nativeAutomationJobForWorkspace(ctx, workspaceID, jobID); err != nil {
			if errors.Is(err, automationpkg.ErrJobNotFound) {
				return automationpkg.Run{}, automationpkg.ErrRunNotFound
			}
			return automationpkg.Run{}, err
		}
	}
	if triggerID := strings.TrimSpace(run.TriggerID); triggerID != "" {
		hasOwnedTarget = true
		if _, err := n.nativeAutomationTriggerForWorkspace(ctx, workspaceID, triggerID); err != nil {
			if errors.Is(err, automationpkg.ErrTriggerNotFound) {
				return automationpkg.Run{}, automationpkg.ErrRunNotFound
			}
			return automationpkg.Run{}, err
		}
	}
	if !hasOwnedTarget {
		return automationpkg.Run{}, automationpkg.ErrRunNotFound
	}
	return run, nil
}

func (n *daemonNativeTools) nativeAutomationRunsForWorkspace(
	ctx context.Context,
	workspaceID string,
	query automationpkg.RunQuery,
) ([]automationpkg.Run, error) {
	if err := n.validateNativeAutomationRunTargets(ctx, workspaceID, query); err != nil {
		return nil, err
	}
	if strings.TrimSpace(query.JobID) != "" || strings.TrimSpace(query.TriggerID) != "" {
		return n.automationManager().ListRuns(ctx, query)
	}

	return n.nativeAutomationRunsAcrossWorkspaceTargets(ctx, workspaceID, query)
}

func (n *daemonNativeTools) validateNativeAutomationRunTargets(
	ctx context.Context,
	workspaceID string,
	query automationpkg.RunQuery,
) error {
	if strings.TrimSpace(query.JobID) != "" {
		if _, err := n.nativeAutomationJobForWorkspace(ctx, workspaceID, query.JobID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(query.TriggerID) != "" {
		if _, err := n.nativeAutomationTriggerForWorkspace(ctx, workspaceID, query.TriggerID); err != nil {
			return err
		}
	}
	return nil
}

func (n *daemonNativeTools) nativeAutomationRunsAcrossWorkspaceTargets(
	ctx context.Context,
	workspaceID string,
	query automationpkg.RunQuery,
) ([]automationpkg.Run, error) {
	jobs, err := n.nativeAutomationJobsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	triggers, err := n.nativeAutomationTriggersForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	limit := query.Limit
	query.Limit = 0
	runsByID := make(map[string]automationpkg.Run)
	for _, job := range jobs {
		jobQuery := query
		jobQuery.JobID = job.ID
		runs, listErr := n.automationManager().ListRuns(ctx, jobQuery)
		if listErr != nil {
			return nil, listErr
		}
		addNativeAutomationRuns(runsByID, runs)
	}
	for _, trigger := range triggers {
		triggerQuery := query
		triggerQuery.TriggerID = trigger.ID
		runs, listErr := n.automationManager().ListRuns(ctx, triggerQuery)
		if listErr != nil {
			return nil, listErr
		}
		addNativeAutomationRuns(runsByID, runs)
	}
	return sortedNativeAutomationRuns(runsByID, limit), nil
}

func sortedNativeAutomationRuns(runsByID map[string]automationpkg.Run, limit int) []automationpkg.Run {
	runs := make([]automationpkg.Run, 0, len(runsByID))
	for _, run := range runsByID {
		runs = append(runs, run)
	}
	sort.Slice(runs, func(left, right int) bool {
		leftStarted := runs[left].StartedAt
		rightStarted := runs[right].StartedAt
		switch {
		case leftStarted != nil && rightStarted == nil:
			return true
		case leftStarted == nil && rightStarted != nil:
			return false
		case leftStarted != nil && rightStarted != nil && !leftStarted.Equal(*rightStarted):
			return leftStarted.After(*rightStarted)
		default:
			return runs[left].ID > runs[right].ID
		}
	})
	if limit > 0 && len(runs) > limit {
		runs = runs[:limit]
	}
	return runs
}

func addNativeAutomationRuns(target map[string]automationpkg.Run, runs []automationpkg.Run) {
	for _, run := range runs {
		target[strings.TrimSpace(run.ID)] = run
	}
}
