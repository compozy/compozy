package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func taskProfileOwner(
	owners map[string]profileOwnerIdentity,
	itemID string,
	profileID string,
) (profileOwnerIdentity, error) {
	owner, found := owners[strings.TrimSpace(profileID)]
	if !found {
		return profileOwnerIdentity{}, fmt.Errorf("api: task %q profile owner %q not found", itemID, profileID)
	}
	return owner, nil
}

func setTaskProfileOwner(
	owners map[string]profileOwnerIdentity,
	itemID string,
	profileID, profileName, profileColor, profileIcon *string,
	useDefaultID bool,
) error {
	if profileID == nil || profileName == nil || profileColor == nil || profileIcon == nil {
		return fmt.Errorf("api: task %q profile owner fields are required", itemID)
	}
	owner, err := taskProfileOwner(owners, itemID, *profileID)
	if err != nil {
		return err
	}
	if *profileID == "" && useDefaultID {
		*profileID = owner.ID
	}
	*profileName, *profileColor, *profileIcon = owner.Name, owner.Color, owner.Icon
	return nil
}

func setTaskPayloadProfileOwner(
	owners map[string]profileOwnerIdentity,
	payload *contract.TaskPayload,
	useDefaultID bool,
) error {
	return setTaskProfileOwner(
		owners, payload.ID, &payload.ProfileID, &payload.ProfileName,
		&payload.ProfileColor, &payload.ProfileIcon, useDefaultID,
	)
}

func setTaskSummaryProfileOwner(
	owners map[string]profileOwnerIdentity,
	payload *contract.TaskSummaryPayload,
	useDefaultID bool,
) error {
	return setTaskProfileOwner(
		owners, payload.ID, &payload.ProfileID, &payload.ProfileName,
		&payload.ProfileColor, &payload.ProfileIcon, useDefaultID,
	)
}

func setTaskCatalogItemProfileOwner(
	owners map[string]profileOwnerIdentity,
	payload *contract.TaskCatalogItemPayload,
	useDefaultID bool,
) error {
	return setTaskProfileOwner(
		owners, payload.ID, &payload.ProfileID, &payload.ProfileName,
		&payload.ProfileColor, &payload.ProfileIcon, useDefaultID,
	)
}

func setTaskRunProfileOwner(
	owners map[string]profileOwnerIdentity,
	payload *contract.TaskRunPayload,
	useDefaultID bool,
) error {
	return setTaskProfileOwner(
		owners, payload.ID, &payload.ProfileID, &payload.ProfileName,
		&payload.ProfileColor, &payload.ProfileIcon, useDefaultID,
	)
}

func (h *BaseHandlers) decorateTaskOwner(ctx context.Context, payload *contract.TaskPayload) error {
	if payload == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	return setTaskPayloadProfileOwner(owners, payload, h == nil || h.Profiles == nil)
}

func (h *BaseHandlers) decorateTaskDetailOwners(ctx context.Context, payload *contract.TaskDetailPayload) error {
	if payload == nil {
		return nil
	}
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	useDefaultID := h == nil || h.Profiles == nil
	if err := setTaskSummaryProfileOwner(owners, &payload.Summary, useDefaultID); err != nil {
		return err
	}
	for index := range payload.Runs {
		if err := setTaskRunProfileOwner(owners, &payload.Runs[index], useDefaultID); err != nil {
			return err
		}
	}
	for index := range payload.Children {
		if err := setTaskSummaryProfileOwner(owners, &payload.Children[index], useDefaultID); err != nil {
			return err
		}
	}
	return setTaskPayloadProfileOwner(owners, &payload.Task, useDefaultID)
}

func (h *BaseHandlers) decorateTaskRunOwners(ctx context.Context, runs []contract.TaskRunPayload) error {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range runs {
		if err := setTaskRunProfileOwner(owners, &runs[index], h == nil || h.Profiles == nil); err != nil {
			return err
		}
	}
	return nil
}
