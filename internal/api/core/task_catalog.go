package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
)

// ParseTaskListQuery parses the bounded task catalog query parameters.
func ParseTaskListQuery(c *gin.Context) (contract.TaskListQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return contract.TaskListQuery{}, NewTaskValidationError(err)
	}
	includeDrafts, err := ParseOptionalBool(c.Query("include_drafts"))
	if err != nil {
		return contract.TaskListQuery{}, NewTaskValidationError(err)
	}
	query := contract.TaskListQuery{
		Scope:                taskpkg.CatalogScope(c.Query("scope")).Normalize(),
		Workspace:            strings.TrimSpace(c.Query("workspace")),
		Status:               taskpkg.Status(c.Query("status")).Normalize(),
		Priority:             taskpkg.Priority(c.Query("priority")).Normalize(),
		IncludeDrafts:        includeDrafts,
		ApprovalState:        taskpkg.ApprovalState(c.Query("approval_state")).Normalize(),
		OwnerKind:            taskpkg.OwnerKind(c.Query("owner_kind")).Normalize(),
		OwnerRef:             strings.TrimSpace(c.Query("owner_ref")),
		ParentTaskID:         strings.TrimSpace(c.Query("parent_task_id")),
		ParticipationChannel: strings.TrimSpace(c.Query("participation_channel")),
		Query:                strings.TrimSpace(c.Query("query")),
		Sort:                 taskpkg.CatalogSort(c.Query("sort")).Normalize(),
		Cursor:               strings.TrimSpace(c.Query("cursor")),
		Limit:                limit,
	}
	if err := contract.ValidateTaskListQuery(query, "task_query"); err != nil {
		return contract.TaskListQuery{}, NewTaskValidationError(err)
	}
	return query, nil
}

// ParseTaskInboxQuery parses the bounded actor-scoped inbox query parameters.
func ParseTaskInboxQuery(c *gin.Context) (contract.TaskInboxQuery, error) {
	unread, err := parseOptionalBoolPointer(c.Query("unread"))
	if err != nil {
		return contract.TaskInboxQuery{}, NewTaskValidationError(err)
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return contract.TaskInboxQuery{}, NewTaskValidationError(err)
	}
	query := contract.TaskInboxQuery{
		Scope:     taskpkg.CatalogScope(c.Query("scope")).Normalize(),
		Workspace: strings.TrimSpace(c.Query("workspace")),
		OwnerKind: taskpkg.OwnerKind(c.Query("owner_kind")).Normalize(),
		OwnerRef:  strings.TrimSpace(c.Query("owner_ref")),
		Lane:      contract.TaskInboxLane(strings.ToLower(strings.TrimSpace(c.Query("lane")))),
		Status:    taskpkg.Status(c.Query("status")).Normalize(),
		Priority:  taskpkg.Priority(c.Query("priority")).Normalize(),
		Unread:    unread,
		Query:     strings.TrimSpace(c.Query("query")),
		Cursor:    strings.TrimSpace(c.Query("cursor")),
		Limit:     limit,
	}
	if err := contract.ValidateTaskInboxQuery(query, "task_inbox_query"); err != nil {
		return contract.TaskInboxQuery{}, NewTaskValidationError(err)
	}
	return query, nil
}

func (h *BaseHandlers) taskListDomainQuery(
	ctx context.Context,
	query contract.TaskListQuery,
) (taskpkg.CatalogQuery, error) {
	domainQuery := taskpkg.CatalogQuery{
		Scope:                query.Scope,
		Status:               query.Status,
		Priority:             query.Priority,
		IncludeDrafts:        query.IncludeDrafts,
		ApprovalState:        query.ApprovalState,
		OwnerKind:            query.OwnerKind,
		OwnerRef:             query.OwnerRef,
		ParentTaskID:         query.ParentTaskID,
		ParticipationChannel: query.ParticipationChannel,
		Search:               query.Query,
		Sort:                 query.Sort,
		Cursor:               query.Cursor,
		Limit:                query.Limit,
	}
	if err := h.resolveTaskCatalogWorkspace(
		ctx,
		query.Workspace,
		&domainQuery.Scope,
		&domainQuery.WorkspaceID,
	); err != nil {
		return taskpkg.CatalogQuery{}, err
	}
	if err := validateParticipationChannel(
		"task_query.participation_channel",
		domainQuery.ParticipationChannel,
	); err != nil {
		return taskpkg.CatalogQuery{}, err
	}
	return taskpkg.NormalizeCatalogQuery(domainQuery)
}

func (h *BaseHandlers) taskInboxDomainQuery(
	ctx context.Context,
	query contract.TaskInboxQuery,
) (observe.TaskInboxQuery, error) {
	domainQuery := observe.TaskInboxQuery{
		Scope:     query.Scope,
		OwnerKind: query.OwnerKind,
		OwnerRef:  query.OwnerRef,
		Lane:      observe.TaskInboxLane(query.Lane),
		Status:    query.Status,
		Priority:  query.Priority,
		Unread:    query.Unread,
		Search:    query.Query,
		Cursor:    query.Cursor,
		Limit:     query.Limit,
	}
	if err := h.resolveTaskCatalogWorkspace(
		ctx,
		query.Workspace,
		&domainQuery.Scope,
		&domainQuery.WorkspaceID,
	); err != nil {
		return observe.TaskInboxQuery{}, err
	}
	return domainQuery, domainQuery.Validate()
}

func (h *BaseHandlers) resolveTaskCatalogWorkspace(
	ctx context.Context,
	workspaceRef string,
	scope *taskpkg.CatalogScope,
	workspaceID *string,
) error {
	ref := strings.TrimSpace(workspaceRef)
	if ref == "" {
		return nil
	}
	if scope.Normalize() == taskpkg.CatalogScopeGlobal {
		return NewTaskValidationError(fmt.Errorf(
			"%w: task_query.workspace must be empty for global scope",
			taskpkg.ErrValidation,
		))
	}
	resolved, err := h.lookupWorkspaceID(ctx, ref)
	if err != nil {
		return err
	}
	*workspaceID = resolved
	return nil
}
