package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type hostAPIProfileIdentity struct {
	name  string
	color string
	icon  string
}

func (h *HostAPIHandler) taskCatalogResponse(
	ctx context.Context,
	page taskpkg.CatalogPage,
) (apicontract.TasksResponse, error) {
	payload := taskCatalogResponseFromPage(page)
	if err := h.decorateTaskCatalog(ctx, &payload); err != nil {
		return apicontract.TasksResponse{}, err
	}
	return payload, nil
}

func (h *HostAPIHandler) taskDetailResponse(
	ctx context.Context,
	view *taskpkg.View,
) (apicontract.TaskDetailPayload, error) {
	payload := taskDetailPayloadFromView(view)
	if err := h.decorateTaskDetail(ctx, &payload); err != nil {
		return apicontract.TaskDetailPayload{}, err
	}
	return payload, nil
}

func (h *HostAPIHandler) taskPayloadResponse(
	ctx context.Context,
	record *taskpkg.Task,
) (apicontract.TaskPayload, error) {
	payload := taskPayloadFromTask(record)
	if err := h.decorateTaskPayload(ctx, &payload); err != nil {
		return apicontract.TaskPayload{}, err
	}
	return payload, nil
}

func (h *HostAPIHandler) taskRunPayloadResponse(
	ctx context.Context,
	manager hostAPITaskManager,
	actor taskpkg.ActorContext,
	run *taskpkg.Run,
) (apicontract.TaskRunPayload, error) {
	if err := ensureHostAPITaskRunProfileID(ctx, manager, actor, run); err != nil {
		return apicontract.TaskRunPayload{}, err
	}
	payloads := []apicontract.TaskRunPayload{taskRunPayloadFromRun(run)}
	if err := h.decorateTaskRuns(ctx, payloads); err != nil {
		return apicontract.TaskRunPayload{}, err
	}
	return payloads[0], nil
}

func (h *HostAPIHandler) taskRunPayloadResponses(
	ctx context.Context,
	manager hostAPITaskManager,
	actor taskpkg.ActorContext,
	runs []taskpkg.Run,
) ([]apicontract.TaskRunPayload, error) {
	for index := range runs {
		if err := ensureHostAPITaskRunProfileID(ctx, manager, actor, &runs[index]); err != nil {
			return nil, err
		}
	}
	payloads := taskRunPayloadsFromRuns(runs)
	if err := h.decorateTaskRuns(ctx, payloads); err != nil {
		return nil, err
	}
	return payloads, nil
}

func ensureHostAPITaskRunProfileID(
	ctx context.Context,
	manager hostAPITaskManager,
	actor taskpkg.ActorContext,
	run *taskpkg.Run,
) error {
	if run == nil || strings.TrimSpace(run.ProfileID) != "" {
		return nil
	}
	if manager == nil {
		return errors.New("extension: task manager is required to resolve task run profile")
	}
	record, err := manager.GetTask(ctx, run.TaskID, actor)
	if err != nil {
		return mapTaskRPCError(run.TaskID, err)
	}
	run.ProfileID = strings.TrimSpace(record.Task.ProfileID)
	if run.ProfileID == "" {
		return fmt.Errorf("extension: task run %q has no profile owner", run.ID)
	}
	return nil
}

func (h *HostAPIHandler) taskProfileIdentities(ctx context.Context) (map[string]hostAPIProfileIdentity, error) {
	if h == nil || h.profiles == nil {
		return nil, errors.New("extension: profile reader is required for task payloads")
	}
	profiles, err := h.profiles.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("extension: list task profile owners: %w", err)
	}
	owners := make(map[string]hostAPIProfileIdentity, len(profiles))
	for _, item := range profiles {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		owners[id] = hostAPIProfileIdentity{
			name:  strings.TrimSpace(item.Name),
			color: strings.TrimSpace(item.Color),
			icon:  strings.TrimSpace(item.Icon),
		}
	}
	return owners, nil
}

func setHostAPITaskProfileOwner(
	owners map[string]hostAPIProfileIdentity,
	itemID string,
	profileID, profileName, profileColor, profileIcon *string,
) error {
	if profileID == nil || profileName == nil || profileColor == nil || profileIcon == nil {
		return fmt.Errorf("extension: task %q profile owner fields are required", itemID)
	}
	id := strings.TrimSpace(*profileID)
	owner, found := owners[id]
	if !found {
		return fmt.Errorf("extension: task %q profile owner %q not found", itemID, id)
	}
	*profileName, *profileColor, *profileIcon = owner.name, owner.color, owner.icon
	return nil
}

func (h *HostAPIHandler) decorateTaskPayload(ctx context.Context, payload *apicontract.TaskPayload) error {
	if payload == nil {
		return nil
	}
	owners, err := h.taskProfileIdentities(ctx)
	if err != nil {
		return err
	}
	return setHostAPITaskProfileOwner(
		owners,
		payload.ID,
		&payload.ProfileID,
		&payload.ProfileName,
		&payload.ProfileColor,
		&payload.ProfileIcon,
	)
}

func (h *HostAPIHandler) decorateTaskCatalog(
	ctx context.Context,
	payload *apicontract.TasksResponse,
) error {
	if payload == nil {
		return nil
	}
	owners, err := h.taskProfileIdentities(ctx)
	if err != nil {
		return err
	}
	for index := range payload.Tasks {
		item := &payload.Tasks[index]
		if err := setHostAPITaskProfileOwner(
			owners,
			item.ID,
			&item.ProfileID,
			&item.ProfileName,
			&item.ProfileColor,
			&item.ProfileIcon,
		); err != nil {
			return err
		}
	}
	return nil
}

func (h *HostAPIHandler) decorateTaskDetail(
	ctx context.Context,
	payload *apicontract.TaskDetailPayload,
) error {
	if payload == nil {
		return nil
	}
	owners, err := h.taskProfileIdentities(ctx)
	if err != nil {
		return err
	}
	decorateSummary := func(summary *apicontract.TaskSummaryPayload) error {
		return setHostAPITaskProfileOwner(
			owners,
			summary.ID,
			&summary.ProfileID,
			&summary.ProfileName,
			&summary.ProfileColor,
			&summary.ProfileIcon,
		)
	}
	if err := decorateSummary(&payload.Summary); err != nil {
		return err
	}
	for index := range payload.Children {
		if err := decorateSummary(&payload.Children[index]); err != nil {
			return err
		}
	}
	if err := setHostAPITaskProfileOwner(
		owners,
		payload.Task.ID,
		&payload.Task.ProfileID,
		&payload.Task.ProfileName,
		&payload.Task.ProfileColor,
		&payload.Task.ProfileIcon,
	); err != nil {
		return err
	}
	return decorateHostAPITaskRuns(owners, payload.Runs)
}

func (h *HostAPIHandler) decorateTaskRuns(ctx context.Context, payloads []apicontract.TaskRunPayload) error {
	owners, err := h.taskProfileIdentities(ctx)
	if err != nil {
		return err
	}
	return decorateHostAPITaskRuns(owners, payloads)
}

func decorateHostAPITaskRuns(
	owners map[string]hostAPIProfileIdentity,
	payloads []apicontract.TaskRunPayload,
) error {
	for index := range payloads {
		payload := &payloads[index]
		if err := setHostAPITaskProfileOwner(
			owners,
			payload.ID,
			&payload.ProfileID,
			&payload.ProfileName,
			&payload.ProfileColor,
			&payload.ProfileIcon,
		); err != nil {
			return err
		}
	}
	return nil
}
