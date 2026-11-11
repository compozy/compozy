package daemon

import (
	"context"
	"errors"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/store"
)

func (n *daemonNativeTools) nativeAutomationJobForWorkspace(
	ctx context.Context,
	workspaceID string,
	jobID string,
	readScope store.ReadScope,
) (automationpkg.Job, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	jobID = strings.TrimSpace(jobID)
	job, err := n.automationManager().GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrJobNotFound) {
			return automationpkg.Job{}, automationpkg.ErrJobNotFound
		}
		return automationpkg.Job{}, err
	}
	if !readScope.Matches(job.ProfileID) {
		return automationpkg.Job{}, automationpkg.ErrJobNotFound
	}
	if workspaceID == "" {
		return job, nil
	}
	if job.Scope != automationpkg.AutomationScopeWorkspace ||
		strings.TrimSpace(job.WorkspaceID) != workspaceID {
		return automationpkg.Job{}, automationpkg.ErrJobNotFound
	}
	return job, nil
}

func (n *daemonNativeTools) nativeAutomationTriggerForWorkspace(
	ctx context.Context,
	workspaceID string,
	triggerID string,
	readScope store.ReadScope,
) (automationpkg.Trigger, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	triggerID = strings.TrimSpace(triggerID)
	trigger, err := n.automationManager().GetTrigger(ctx, triggerID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrTriggerNotFound) {
			return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
		}
		return automationpkg.Trigger{}, err
	}
	if !readScope.Matches(trigger.ProfileID) {
		return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
	}
	if workspaceID == "" {
		return trigger, nil
	}
	if trigger.Scope != automationpkg.AutomationScopeWorkspace ||
		strings.TrimSpace(trigger.WorkspaceID) != workspaceID {
		return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
	}
	return trigger, nil
}

func (n *daemonNativeTools) nativeAutomationRunForWorkspace(
	ctx context.Context,
	workspaceID string,
	runID string,
	readScope store.ReadScope,
) (automationpkg.Run, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	runID = strings.TrimSpace(runID)
	run, err := n.automationManager().GetRun(ctx, runID)
	if err != nil {
		if errors.Is(err, automationpkg.ErrRunNotFound) {
			return automationpkg.Run{}, automationpkg.ErrRunNotFound
		}
		return automationpkg.Run{}, err
	}
	if !readScope.Matches(run.ProfileID) {
		return automationpkg.Run{}, automationpkg.ErrRunNotFound
	}
	hasTarget := false
	if jobID := strings.TrimSpace(run.JobID); jobID != "" {
		hasTarget = true
		job, jobErr := n.nativeAutomationJobForWorkspace(ctx, workspaceID, jobID, readScope)
		if jobErr != nil {
			if errors.Is(jobErr, automationpkg.ErrJobNotFound) {
				return automationpkg.Run{}, automationpkg.ErrRunNotFound
			}
			return automationpkg.Run{}, jobErr
		}
		run.ProfileID = job.ProfileID
	}
	if triggerID := strings.TrimSpace(run.TriggerID); triggerID != "" {
		hasTarget = true
		trigger, triggerErr := n.nativeAutomationTriggerForWorkspace(ctx, workspaceID, triggerID, readScope)
		if triggerErr != nil {
			if errors.Is(triggerErr, automationpkg.ErrTriggerNotFound) {
				return automationpkg.Run{}, automationpkg.ErrRunNotFound
			}
			return automationpkg.Run{}, triggerErr
		}
		run.ProfileID = trigger.ProfileID
	}
	if !hasTarget {
		return automationpkg.Run{}, automationpkg.ErrRunNotFound
	}
	return run, nil
}

func (n *daemonNativeTools) validateNativeAutomationRunTargets(
	ctx context.Context,
	workspaceID string,
	query automationpkg.RunQuery,
) error {
	if strings.TrimSpace(query.JobID) != "" {
		if _, err := n.nativeAutomationJobForWorkspace(ctx, workspaceID, query.JobID, query.ReadScope); err != nil {
			return err
		}
	}
	if strings.TrimSpace(query.TriggerID) != "" {
		if _, err := n.nativeAutomationTriggerForWorkspace(
			ctx,
			workspaceID,
			query.TriggerID,
			query.ReadScope,
		); err != nil {
			return err
		}
	}
	return nil
}
