package core

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) decorateTaskDetailOwners(ctx context.Context, payload *contract.TaskDetailPayload) error {
	if payload == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	decorateSummary := func(summary *contract.TaskSummaryPayload) error {
		owner, found := owners[summary.ProfileID]
		if !found {
			return fmt.Errorf("api: task %q profile owner %q not found", summary.ID, summary.ProfileID)
		}
		if summary.ProfileID == "" && (h == nil || h.Profiles == nil) {
			summary.ProfileID = owner.ID
		}
		summary.ProfileName, summary.ProfileColor, summary.ProfileIcon = owner.Name, owner.Color, owner.Icon
		return nil
	}
	if err := decorateSummary(&payload.Summary); err != nil {
		return err
	}
	for index := range payload.Runs {
		owner, found := owners[payload.Runs[index].ProfileID]
		if !found {
			return fmt.Errorf(
				"api: task run %q profile owner %q not found",
				payload.Runs[index].ID,
				payload.Runs[index].ProfileID,
			)
		}
		payload.Runs[index].ProfileName = owner.Name
		payload.Runs[index].ProfileColor = owner.Color
		payload.Runs[index].ProfileIcon = owner.Icon
	}
	for index := range payload.Children {
		if err := decorateSummary(&payload.Children[index]); err != nil {
			return err
		}
	}
	owner, found := owners[payload.Task.ProfileID]
	if !found {
		return fmt.Errorf("api: task %q profile owner %q not found", payload.Task.ID, payload.Task.ProfileID)
	}
	if payload.Task.ProfileID == "" && (h == nil || h.Profiles == nil) {
		payload.Task.ProfileID = owner.ID
	}
	payload.Task.ProfileName, payload.Task.ProfileColor, payload.Task.ProfileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func (h *BaseHandlers) decorateTaskRunOwners(ctx context.Context, runs []contract.TaskRunPayload) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range runs {
		owner, found := owners[runs[index].ProfileID]
		if !found {
			return fmt.Errorf("api: task run %q profile owner %q not found", runs[index].ID, runs[index].ProfileID)
		}
		if runs[index].ProfileID == "" && (h == nil || h.Profiles == nil) {
			runs[index].ProfileID = owner.ID
		}
		runs[index].ProfileName = owner.Name
		runs[index].ProfileColor = owner.Color
		runs[index].ProfileIcon = owner.Icon
	}
	return nil
}
