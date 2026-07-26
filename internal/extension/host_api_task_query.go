package extensionpkg

import (
	"context"

	"errors"
	"fmt"
	"strings"

	apicontract "github.com/compozy/agh/internal/api/contract"

	observepkg "github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (h *HostAPIHandler) taskManager() (hostAPITaskManager, error) {
	if h == nil {
		return nil, errors.New("extension: host api handler is required")
	}
	if h.tasks == nil {
		return nil, errors.New("extension: task manager is not configured")
	}
	return h.tasks, nil
}

func (h *HostAPIHandler) taskObserver() (hostAPIObserver, error) {
	if h == nil {
		return nil, errors.New("extension: host api handler is required")
	}
	if h.observer == nil {
		return nil, errors.New("extension: task observer is not configured")
	}
	return h.observer, nil
}

func (h *HostAPIHandler) taskManagerAndActor(ctx context.Context) (hostAPITaskManager, taskpkg.ActorContext, error) {
	manager, err := h.taskManager()
	if err != nil {
		return nil, taskpkg.ActorContext{}, fmt.Errorf("extension: resolve task manager: %w", err)
	}
	actor, err := h.taskActorContext(ctx)
	if err != nil {
		return nil, taskpkg.ActorContext{}, fmt.Errorf("extension: derive task actor context: %w", err)
	}
	return manager, actor, nil
}

func (h *HostAPIHandler) taskActorContext(ctx context.Context) (taskpkg.ActorContext, error) {
	extName := hostAPIExtensionNameFromContext(ctx)
	if extName == "" {
		return taskpkg.ActorContext{}, unavailableRPCError(errors.New("extension name is not available"))
	}

	actor, err := taskpkg.DeriveExtensionActorContext(extName, "")
	if err != nil {
		return taskpkg.ActorContext{}, invalidParamsRPCError(err)
	}
	return actor, nil
}

func (h *HostAPIHandler) taskQueryFromParams(
	ctx context.Context,
	params hostAPITasksParams,
) (taskpkg.CatalogQuery, error) {
	if err := apicontract.ValidateTaskListQuery(params, "task_query"); err != nil {
		return taskpkg.CatalogQuery{}, invalidParamsRPCError(err)
	}
	query := taskpkg.CatalogQuery{
		Scope:         params.Scope.Normalize(),
		Status:        params.Status.Normalize(),
		Priority:      params.Priority.Normalize(),
		IncludeDrafts: params.IncludeDrafts,
		ApprovalState: params.ApprovalState.Normalize(),
		OwnerKind:     params.OwnerKind.Normalize(),
		OwnerRef:      strings.TrimSpace(params.OwnerRef),
		ParentTaskID:  strings.TrimSpace(params.ParentTaskID),
		Search:        strings.TrimSpace(params.Query),
		Sort:          params.Sort.Normalize(),
		Cursor:        strings.TrimSpace(params.Cursor),
		Limit:         params.Limit,
	}
	if workspaceRef := strings.TrimSpace(params.Workspace); workspaceRef != "" {
		if query.Scope.Normalize() == taskpkg.CatalogScopeGlobal {
			return taskpkg.CatalogQuery{}, invalidParamsRPCError(fmt.Errorf(
				"%w: task_query.workspace must be empty for global scope",
				taskpkg.ErrValidation,
			))
		}
		workspaceID, err := h.resolveTaskWorkspaceID(ctx, workspaceRef)
		if err != nil {
			return taskpkg.CatalogQuery{}, err
		}
		query.WorkspaceID = workspaceID
	}
	if err := validateTaskChannel("task_query.participation_channel", params.ParticipationChannel); err != nil {
		return taskpkg.CatalogQuery{}, err
	}
	query.ParticipationChannel = strings.TrimSpace(params.ParticipationChannel)
	normalized, err := taskpkg.NormalizeCatalogQuery(query)
	if err != nil {
		return taskpkg.CatalogQuery{}, invalidParamsRPCError(err)
	}
	return normalized, nil
}

func taskTimelineQueryFromParams(params apicontract.TaskTimelineQuery) (taskpkg.TimelineQuery, error) {
	query := taskpkg.TimelineQuery{
		AfterSequence: params.AfterSequence,
		Limit:         params.Limit,
	}
	if err := query.Validate("task_timeline_query"); err != nil {
		return taskpkg.TimelineQuery{}, invalidParamsRPCError(err)
	}
	return query, nil
}

func taskRunQueryFromParams(params apicontract.TaskRunListQuery) (taskpkg.RunQuery, error) {
	query := taskpkg.RunQuery{
		Status:               params.Status.Normalize(),
		SessionID:            strings.TrimSpace(params.SessionID),
		ParticipationChannel: strings.TrimSpace(params.ParticipationChannel),
		Limit:                params.Limit,
	}
	if err := query.Validate("task_run_query"); err != nil {
		return taskpkg.RunQuery{}, invalidParamsRPCError(err)
	}
	return query, nil
}

func (h *HostAPIHandler) taskDashboardQueryFromParams(
	ctx context.Context,
	params apicontract.TaskDashboardQuery,
) (observepkg.TaskDashboardQuery, error) {
	query := observepkg.TaskDashboardQuery{
		Scope:                params.Scope.Normalize(),
		OwnerKind:            params.OwnerKind.Normalize(),
		OwnerRef:             strings.TrimSpace(params.OwnerRef),
		ParticipationChannel: strings.TrimSpace(params.ParticipationChannel),
		OriginKind:           params.OriginKind.Normalize(),
	}
	if query.Scope.Normalize() != "" {
		if err := query.Scope.Validate("task_dashboard_query.scope"); err != nil {
			return observepkg.TaskDashboardQuery{}, invalidParamsRPCError(err)
		}
	}
	if workspaceRef := strings.TrimSpace(params.Workspace); workspaceRef != "" {
		if query.Scope.Normalize() == taskpkg.ScopeGlobal {
			if err := taskpkg.ValidateScopeBinding(
				query.Scope,
				workspaceRef,
				"task_dashboard_query",
				"workspace",
			); err != nil {
				return observepkg.TaskDashboardQuery{}, invalidParamsRPCError(err)
			}
		}
		workspaceID, err := h.resolveTaskWorkspaceID(ctx, workspaceRef)
		if err != nil {
			return observepkg.TaskDashboardQuery{}, err
		}
		query.WorkspaceID = workspaceID
	}
	if err := validateTaskChannel(
		"task_dashboard_query.participation_channel",
		query.ParticipationChannel,
	); err != nil {
		return observepkg.TaskDashboardQuery{}, err
	}
	if err := query.Validate(); err != nil {
		return observepkg.TaskDashboardQuery{}, invalidParamsRPCError(err)
	}
	return query, nil
}

func (h *HostAPIHandler) taskInboxQueryFromParams(
	ctx context.Context,
	params apicontract.TaskInboxQuery,
) (observepkg.TaskInboxQuery, error) {
	if err := apicontract.ValidateTaskInboxQuery(params, "task_inbox_query"); err != nil {
		return observepkg.TaskInboxQuery{}, invalidParamsRPCError(err)
	}
	query := observepkg.TaskInboxQuery{
		Scope:     params.Scope.Normalize(),
		OwnerKind: params.OwnerKind.Normalize(),
		OwnerRef:  strings.TrimSpace(params.OwnerRef),
		Lane:      observepkg.TaskInboxLane(params.Lane).Normalize(),
		Status:    params.Status.Normalize(),
		Priority:  params.Priority.Normalize(),
		Unread:    params.Unread,
		Search:    strings.TrimSpace(params.Query),
		Cursor:    strings.TrimSpace(params.Cursor),
		Limit:     params.Limit,
	}
	if workspaceRef := strings.TrimSpace(params.Workspace); workspaceRef != "" {
		if query.Scope.Normalize() == taskpkg.CatalogScopeGlobal {
			return observepkg.TaskInboxQuery{}, invalidParamsRPCError(fmt.Errorf(
				"%w: task_inbox_query.workspace must be empty for global scope",
				taskpkg.ErrValidation,
			))
		}
		workspaceID, err := h.resolveTaskWorkspaceID(ctx, workspaceRef)
		if err != nil {
			return observepkg.TaskInboxQuery{}, err
		}
		query.WorkspaceID = workspaceID
	}
	if err := query.Validate(); err != nil {
		return observepkg.TaskInboxQuery{}, invalidParamsRPCError(err)
	}
	return query, nil
}
