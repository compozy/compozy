package extensionpkg

import (
	"context"
	"errors"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
)

func (h *HostAPIHandler) automationJobForBoundSession(
	ctx context.Context,
	automation HostAPIAutomationManager,
	jobID string,
) (automationpkg.Job, error) {
	boundWorkspaceID, bound, err := hostAPIBoundWorkspaceID(ctx)
	if err != nil {
		return automationpkg.Job{}, err
	}
	jobID = strings.TrimSpace(jobID)
	if !bound {
		return automation.GetJob(ctx, jobID)
	}

	query := automationpkg.JobListQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: strings.TrimSpace(boundWorkspaceID),
		Limit:       automationpkg.MaxListLimit,
	}
	for {
		page, listErr := automation.ListJobs(ctx, query)
		if listErr != nil {
			return automationpkg.Job{}, listErr
		}
		for _, job := range page.Jobs {
			if strings.TrimSpace(job.ID) == jobID {
				return job, nil
			}
		}
		if !page.HasMore || strings.TrimSpace(page.NextCursor) == "" {
			return automationpkg.Job{}, automationpkg.ErrJobNotFound
		}
		query.Cursor = page.NextCursor
	}
}

func (h *HostAPIHandler) automationTriggerForBoundSession(
	ctx context.Context,
	automation HostAPIAutomationManager,
	triggerID string,
) (automationpkg.Trigger, error) {
	boundWorkspaceID, bound, err := hostAPIBoundWorkspaceID(ctx)
	if err != nil {
		return automationpkg.Trigger{}, err
	}
	triggerID = strings.TrimSpace(triggerID)
	if !bound {
		return automation.GetTrigger(ctx, triggerID)
	}

	query := automationpkg.TriggerListQuery{
		Scope:       automationpkg.AutomationScopeWorkspace,
		WorkspaceID: strings.TrimSpace(boundWorkspaceID),
		Limit:       automationpkg.MaxListLimit,
	}
	for {
		page, listErr := automation.ListTriggers(ctx, query)
		if listErr != nil {
			return automationpkg.Trigger{}, listErr
		}
		for _, trigger := range page.Triggers {
			if strings.TrimSpace(trigger.ID) == triggerID {
				return trigger, nil
			}
		}
		if !page.HasMore || strings.TrimSpace(page.NextCursor) == "" {
			return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
		}
		query.Cursor = page.NextCursor
	}
}

func (h *HostAPIHandler) requireBoundAutomationRunFilter(
	ctx context.Context,
	automation HostAPIAutomationManager,
	jobID string,
	triggerID string,
) error {
	_, bound, err := hostAPIBoundWorkspaceID(ctx)
	if err != nil || !bound {
		return err
	}
	jobID = strings.TrimSpace(jobID)
	triggerID = strings.TrimSpace(triggerID)
	if jobID == "" && triggerID == "" {
		return invalidParamsRPCError(errors.New(
			"workspace-bound automation runs require job_id or trigger_id",
		))
	}
	if jobID != "" {
		if _, err := h.automationJobForBoundSession(ctx, automation, jobID); err != nil {
			return err
		}
	}
	if triggerID != "" {
		if _, err := h.automationTriggerForBoundSession(ctx, automation, triggerID); err != nil {
			return err
		}
	}
	return nil
}
