package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

func (h *BaseHandlers) decorateAutomationJobOwners(ctx context.Context, jobs []contract.JobPayload) error {
	for index := range jobs {
		if err := h.decorateAutomationJobOwner(ctx, &jobs[index]); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateAutomationJobOwner(ctx context.Context, job *contract.JobPayload) error {
	if job == nil {
		return nil
	}
	owner, err := h.automationProfileOwner(ctx, job.ID, job.ProfileID)
	if err != nil {
		return err
	}
	job.ProfileID = owner.ID
	job.ProfileName, job.ProfileColor, job.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func (h *BaseHandlers) decorateAutomationTriggerOwners(
	ctx context.Context,
	triggers []contract.TriggerPayload,
) error {
	for index := range triggers {
		if err := h.decorateAutomationTriggerOwner(ctx, &triggers[index]); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateAutomationTriggerOwner(
	ctx context.Context,
	trigger *contract.TriggerPayload,
) error {
	if trigger == nil {
		return nil
	}
	owner, err := h.automationProfileOwner(ctx, trigger.ID, trigger.ProfileID)
	if err != nil {
		return err
	}
	trigger.ProfileID = owner.ID
	trigger.ProfileName, trigger.ProfileColor, trigger.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func (h *BaseHandlers) decorateAutomationRunOwners(ctx context.Context, runs []contract.RunPayload) error {
	for index := range runs {
		if err := h.decorateAutomationRunOwner(ctx, &runs[index]); err != nil {
			return err
		}
	}
	return nil
}

func (h *BaseHandlers) decorateAutomationRunOwner(ctx context.Context, run *contract.RunPayload) error {
	if run == nil {
		return nil
	}
	owner, err := h.automationProfileOwner(ctx, run.ID, run.ProfileID)
	if err != nil {
		return err
	}
	run.ProfileID = owner.ID
	run.ProfileName, run.ProfileColor, run.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func (h *BaseHandlers) automationProfileOwner(
	ctx context.Context,
	itemID string,
	profileID string,
) (profileOwnerIdentity, error) {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return profileOwnerIdentity{}, err
	}
	owner, found := owners[strings.TrimSpace(profileID)]
	if !found {
		return profileOwnerIdentity{}, fmt.Errorf(
			"api: automation %q profile owner %q not found",
			itemID,
			profileID,
		)
	}
	return owner, nil
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
