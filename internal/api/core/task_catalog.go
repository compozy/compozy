package core

import (
	"context"
	"fmt"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/observe"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// ParseTaskListQuery parses the bounded task catalog query parameters.
func ParseTaskListQuery(c *gin.Context, workspaces ...WorkspaceGetter) (contract.TaskListQuery, error) {
	breadth, err := parseWorkspaceBreadth(
		c,
		c.Param("workspace_id"),
		breadthEncoding{Selector: workspaceQueryKey, OmissionIsAll: true},
		workspaces...)
	if err != nil {
		return contract.TaskListQuery{}, err
	}
	explicitAll, err := ParseOptionalBool(c.Query("all_workspaces"))
	if err != nil {
		return contract.TaskListQuery{}, err
	}
	// Implicit archive exclusion belongs to the catalog predicate, not the caller's authority.
	if !explicitAll {
		breadth.WorkspaceIDs = nil
	}
	scope := taskpkg.CatalogScope(c.Query("scope")).Normalize()
	if explicitAll && scope != "" && scope != taskpkg.CatalogScopeAll {
		return contract.TaskListQuery{}, invalidWorkspaceBreadth("all_workspaces conflicts with scope")
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return contract.TaskListQuery{}, NewTaskValidationError(err)
	}
	includeDrafts, err := ParseOptionalBool(c.Query("include_drafts"))
	if err != nil {
		return contract.TaskListQuery{}, NewTaskValidationError(err)
	}
	includeLoop, err := ParseOptionalBool(c.Query("include_loop"))
	if err != nil {
		return contract.TaskListQuery{}, &invalidTaskQueryFieldError{
			field: "include_loop",
			cause: err,
		}
	}
	query := contract.TaskListQuery{
		AllWorkspaces: explicitAll,
		WorkspaceIDs:  breadth.WorkspaceIDs,
		Scope:         taskpkg.CatalogScope(c.Query("scope")).Normalize(),
		Workspace:     strings.TrimSpace(c.Query("workspace")),
		Status:        taskpkg.Status(c.Query("status")).Normalize(),
		Priority:      taskpkg.Priority(c.Query("priority")).Normalize(),
		IncludeDrafts: includeDrafts,
		IncludeLoop:   includeLoop,
		LoopRunID:     strings.TrimSpace(c.Query("loop_run_id")),
		ApprovalState: taskpkg.ApprovalState(c.Query("approval_state")).Normalize(),
		OwnerKind:     taskpkg.OwnerKind(c.Query("owner_kind")).Normalize(),
		OwnerRef:      strings.TrimSpace(c.Query("owner_ref")),
		ParentTaskID:  strings.TrimSpace(c.Query("parent_task_id")),
		Worktree:      strings.TrimSpace(c.Query("worktree")),
		Query:         strings.TrimSpace(c.Query("query")),
		Sort:          taskpkg.CatalogSort(c.Query("sort")).Normalize(),
		Cursor:        strings.TrimSpace(c.Query("cursor")),
		Limit:         limit,
	}
	if err := contract.ValidateTaskListQuery(query, "task_query"); err != nil {
		return contract.TaskListQuery{}, NewTaskValidationError(err)
	}
	return query, nil
}

// ParseTaskInboxQuery parses the bounded actor-scoped inbox query parameters.
func ParseTaskInboxQuery(c *gin.Context, workspaces ...WorkspaceGetter) (contract.TaskInboxQuery, error) {
	unread, err := parseOptionalBoolPointer(c.Query("unread"))
	if err != nil {
		return contract.TaskInboxQuery{}, NewTaskValidationError(err)
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return contract.TaskInboxQuery{}, NewTaskValidationError(err)
	}
	workspaceIDs, err := parseTaskInboxWorkspaceIDs(c)
	if err != nil {
		return contract.TaskInboxQuery{}, err
	}
	allWorkspaces, err := ParseOptionalBool(c.Query("all_workspaces"))
	if err != nil {
		return contract.TaskInboxQuery{}, invalidWorkspaceBreadth("all_workspaces must be a boolean")
	}
	if shouldFilterArchivedWorkspaces(c, allWorkspaces, c.Query("workspace"), true) && len(workspaces) > 0 {
		workspaceIDs, err = defaultActiveWorkspaceIDs(c.Request.Context(), workspaces[0], workspaceIDs)
		if err != nil {
			return contract.TaskInboxQuery{}, err
		}
	}
	query := contract.TaskInboxQuery{
		WorkspaceIDs: workspaceIDs,
		Scope:        taskpkg.CatalogScope(c.Query("scope")).Normalize(),
		Workspace:    strings.TrimSpace(c.Query("workspace")),
		Worktree:     strings.TrimSpace(c.Query("worktree")),
		OwnerKind:    taskpkg.OwnerKind(c.Query("owner_kind")).Normalize(),
		OwnerRef:     strings.TrimSpace(c.Query("owner_ref")),
		Lane:         contract.TaskInboxLane(strings.ToLower(strings.TrimSpace(c.Query("lane")))),
		Status:       taskpkg.Status(c.Query("status")).Normalize(),
		Priority:     taskpkg.Priority(c.Query("priority")).Normalize(),
		Unread:       unread,
		Query:        strings.TrimSpace(c.Query("query")),
		Cursor:       strings.TrimSpace(c.Query("cursor")),
		Limit:        limit,
	}
	if err := contract.ValidateTaskInboxQuery(query, "task_inbox_query"); err != nil {
		return contract.TaskInboxQuery{}, NewTaskValidationError(err)
	}
	return query, nil
}

// parseTaskInboxWorkspaceIDs reads the All projects narrowing. The inbox's omission already
// reads every task, so all_workspaces only gates workspace_ids (AP BR-12).
func parseTaskInboxWorkspaceIDs(c *gin.Context) ([]string, error) {
	all, err := ParseOptionalBool(c.Query("all_workspaces"))
	if err != nil {
		return nil, invalidWorkspaceBreadth("all_workspaces must be a boolean")
	}
	ids, hasIDs := c.Request.URL.Query()["workspace_ids"]
	if hasIDs && !all {
		return nil, invalidWorkspaceBreadth("workspace_ids requires all_workspaces=true")
	}
	if all && (strings.TrimSpace(c.Query("workspace")) != "" || strings.TrimSpace(c.Query("scope")) != "") {
		return nil, invalidWorkspaceBreadth("all_workspaces conflicts with scope or workspace")
	}
	normalized := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, invalidWorkspaceBreadth("workspace_ids must not be empty; use @user")
		}
		normalized = append(normalized, id)
	}
	return normalized, nil
}

func (h *BaseHandlers) taskListDomainQuery(
	ctx context.Context,
	readScope store.ReadScope,
	query contract.TaskListQuery,
) (taskpkg.CatalogQuery, error) {
	domainQuery := taskpkg.CatalogQuery{
		ReadScope:     readScope,
		AllWorkspaces: query.AllWorkspaces,
		WorkspaceIDs:  query.WorkspaceIDs,
		Scope:         query.Scope,
		Status:        query.Status,
		Priority:      query.Priority,
		IncludeDrafts: query.IncludeDrafts,
		ApprovalState: query.ApprovalState,
		OwnerKind:     query.OwnerKind,
		OwnerRef:      query.OwnerRef,
		ParentTaskID:  query.ParentTaskID,
		WorktreeID:    query.Worktree,
		Search:        query.Query,
		Sort:          query.Sort,
		Cursor:        query.Cursor,
		Limit:         query.Limit,
	}
	ApplyTaskLoopCatalogFilters(&domainQuery, query.IncludeLoop, query.LoopRunID)
	if err := h.resolveTaskCatalogWorkspace(
		ctx,
		query.Workspace,
		&domainQuery.Scope,
		&domainQuery.WorkspaceID,
	); err != nil {
		return taskpkg.CatalogQuery{}, err
	}
	return taskpkg.NormalizeCatalogQuery(domainQuery)
}

func (h *BaseHandlers) taskInboxDomainQuery(
	ctx context.Context,
	readScope store.ReadScope,
	query contract.TaskInboxQuery,
) (observe.TaskInboxQuery, error) {
	domainQuery := observe.TaskInboxQuery{
		ReadScope:        readScope,
		Scope:            query.Scope,
		WorkspaceIDs:     query.WorkspaceIDs,
		WorktreeID:       query.Worktree,
		OwnerKind:        query.OwnerKind,
		OwnerRef:         query.OwnerRef,
		Lane:             observe.TaskInboxLane(query.Lane),
		Status:           query.Status,
		Priority:         query.Priority,
		Unread:           query.Unread,
		Search:           query.Query,
		Cursor:           query.Cursor,
		Limit:            query.Limit,
		ExcludeCreatedBy: defaultTaskLoopExclusions(),
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
	if workspacepkg.ParseRef(ref).Kind == workspacepkg.RefUser {
		// User tasks are stored as global-scope tasks without a workspace (USW BR-1, BR-17).
		*scope = taskpkg.CatalogScopeGlobal
		*workspaceID = ""
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
