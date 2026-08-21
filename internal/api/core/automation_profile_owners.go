package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

func (h *BaseHandlers) decorateAutomationJobOwners(ctx context.Context, jobs []contract.JobPayload) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range jobs {
		if err := decorateAutomationJobOwnerWithOwners(&jobs[index], owners); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateAutomationJobOwner(ctx context.Context, job *contract.JobPayload) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	return decorateAutomationJobOwnerWithOwners(job, owners)
}

func (h *BaseHandlers) decorateAutomationTriggerOwners(
	ctx context.Context,
	triggers []contract.TriggerPayload,
) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range triggers {
		if err := decorateAutomationTriggerOwnerWithOwners(&triggers[index], owners); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateAutomationTriggerOwner(
	ctx context.Context,
	trigger *contract.TriggerPayload,
) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	return decorateAutomationTriggerOwnerWithOwners(trigger, owners)
}

func (h *BaseHandlers) decorateAutomationRunOwners(ctx context.Context, runs []contract.RunPayload) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range runs {
		if err := decorateAutomationRunOwnerWithOwners(&runs[index], owners); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateAutomationRunOwner(ctx context.Context, run *contract.RunPayload) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	return decorateAutomationRunOwnerWithOwners(run, owners)
}

func decorateAutomationJobOwnerWithOwners(job *contract.JobPayload, owners map[string]profileOwnerIdentity) error {
	if job == nil {
		return nil
	}
	owner, found := owners[strings.TrimSpace(job.ProfileID)]
	if !found {
		return fmt.Errorf("api: automation %q profile owner %q not found", job.ID, job.ProfileID)
	}
	job.ProfileID = owner.ID
	job.ProfileName, job.ProfileColor, job.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func decorateAutomationTriggerOwnerWithOwners(
	trigger *contract.TriggerPayload,
	owners map[string]profileOwnerIdentity,
) error {
	if trigger == nil {
		return nil
	}
	owner, found := owners[strings.TrimSpace(trigger.ProfileID)]
	if !found {
		return fmt.Errorf("api: automation %q profile owner %q not found", trigger.ID, trigger.ProfileID)
	}
	trigger.ProfileID = owner.ID
	trigger.ProfileName, trigger.ProfileColor, trigger.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func decorateAutomationRunOwnerWithOwners(run *contract.RunPayload, owners map[string]profileOwnerIdentity) error {
	if run == nil {
		return nil
	}
	owner, found := owners[strings.TrimSpace(run.ProfileID)]
	if !found {
		return fmt.Errorf("api: automation %q profile owner %q not found", run.ID, run.ProfileID)
	}
	run.ProfileID = owner.ID
	run.ProfileName, run.ProfileColor, run.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func (h *BaseHandlers) populateAutomationRunProfileID(
	ctx context.Context,
	manager AutomationManager,
	run *automationpkg.Run,
) error {
	if run == nil || strings.TrimSpace(run.ProfileID) != "" {
		return nil
	}
	switch {
	case strings.TrimSpace(run.JobID) != "":
		job, err := manager.GetJob(ctx, run.JobID)
		if err != nil {
			return err
		}
		run.ProfileID = strings.TrimSpace(job.ProfileID)
	case strings.TrimSpace(run.TriggerID) != "":
		trigger, err := manager.GetTrigger(ctx, run.TriggerID)
		if err != nil {
			return err
		}
		run.ProfileID = strings.TrimSpace(trigger.ProfileID)
	default:
		return fmt.Errorf("api: automation run %q has no owning definition", run.ID)
	}
	return nil
}
