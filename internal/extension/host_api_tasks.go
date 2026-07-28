package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func (h *HostAPIHandler) handleTasks(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITasksParams
	if err := decodeHostAPIParamsStrict(raw, &params); err != nil {
		return nil, err
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	query, err := h.taskQueryFromParams(ctx, params)
	if err != nil {
		return nil, err
	}
	page, err := manager.ListTaskCatalog(ctx, query, actor)
	if err != nil {
		return nil, mapTaskRPCError("", err)
	}
	return taskCatalogResponseFromPage(page), nil
}

func (h *HostAPIHandler) handleTasksGet(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.ID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}

	view, err := manager.GetTask(ctx, taskID, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskDetailPayloadFromView(view), nil
}

func (h *HostAPIHandler) handleTasksTimeline(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskTimelineParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.ID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	query, err := taskTimelineQueryFromParams(params.TaskTimelineQuery)
	if err != nil {
		return nil, err
	}

	items, err := manager.Timeline(ctx, taskID, query, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskTimelineItemPayloadsFromItems(items), nil
}

func (h *HostAPIHandler) handleTasksTree(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskTreeParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.ID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}

	view, err := manager.Tree(ctx, taskID, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskTreePayloadFromView(view), nil
}

func (h *HostAPIHandler) handleTasksDashboard(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskDashboardParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	observer, err := h.taskObserver()
	if err != nil {
		return nil, err
	}
	query, err := h.taskDashboardQueryFromParams(ctx, params)
	if err != nil {
		return nil, err
	}

	view, err := observer.QueryTaskDashboard(ctx, query)
	if err != nil {
		return nil, mapTaskRPCError("", err)
	}
	return taskDashboardPayloadFromView(&view), nil
}

func (h *HostAPIHandler) handleTasksInbox(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskInboxParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	observer, err := h.taskObserver()
	if err != nil {
		return nil, err
	}
	actor, err := h.taskActorContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("extension: derive task actor context: %w", err)
	}
	query, err := h.taskInboxQueryFromParams(ctx, params)
	if err != nil {
		return nil, err
	}

	view, err := observer.QueryTaskInbox(ctx, query, actor.Actor)
	if err != nil {
		return nil, mapTaskRPCError("", err)
	}
	return taskInboxPayloadFromView(view), nil
}

func (h *HostAPIHandler) handleTasksCreate(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskCreateParams
	if err := decodeHostAPIParamsStrict(raw, &params); err != nil {
		return nil, err
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	spec, err := h.createTaskSpecFromRequest(ctx, params)
	if err != nil {
		return nil, err
	}

	record, err := manager.CreateTask(ctx, spec, actor)
	if err != nil {
		return nil, mapTaskRPCError(spec.ID, err)
	}
	return taskPayloadFromTask(record), nil
}

func (h *HostAPIHandler) handleTasksUpdate(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskUpdateParams
	if err := decodeHostAPIParamsStrict(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.ID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}
	if !params.HasChanges() {
		return nil, invalidParamsRPCError(errors.New("task update must include at least one mutable field"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	patch, err := taskPatchFromRequest(params.UpdateTaskRequest)
	if err != nil {
		return nil, err
	}

	record, err := manager.UpdateTask(ctx, taskID, patch, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskPayloadFromTask(record), nil
}

func (h *HostAPIHandler) handleTasksCancel(ctx context.Context, raw json.RawMessage) (any, error) {
	var params hostAPITaskCancelParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	taskID := strings.TrimSpace(params.ID)
	if taskID == "" {
		return nil, invalidParamsRPCError(errors.New("id is required"))
	}

	manager, actor, err := h.taskManagerAndActor(ctx)
	if err != nil {
		return nil, err
	}
	cancelReq, err := cancelTaskFromRequest(params.CancelTaskRequest)
	if err != nil {
		return nil, err
	}

	record, err := manager.CancelTask(ctx, taskID, cancelReq, actor)
	if err != nil {
		return nil, mapTaskRPCError(taskID, err)
	}
	return taskPayloadFromTask(record), nil
}
