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

	job, err := automation.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrJobNotFound) {
			return automationpkg.Job{}, automationpkg.ErrJobNotFound
		}
		return automationpkg.Job{}, err
	}
	if job.Scope != automationpkg.AutomationScopeWorkspace ||
		strings.TrimSpace(job.WorkspaceID) != strings.TrimSpace(boundWorkspaceID) {
		return automationpkg.Job{}, automationpkg.ErrJobNotFound
	}
	return job, nil
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

	trigger, err := automation.GetTrigger(ctx, triggerID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrTriggerNotFound) {
			return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
		}
		return automationpkg.Trigger{}, err
	}
	if trigger.Scope != automationpkg.AutomationScopeWorkspace ||
		strings.TrimSpace(trigger.WorkspaceID) != strings.TrimSpace(boundWorkspaceID) {
		return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
	}
	return trigger, nil
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
