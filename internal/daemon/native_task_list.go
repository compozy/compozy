package daemon

import (
	"context"
	"fmt"
	"strings"

	core "github.com/compozy/agh/internal/api/core"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type taskListInput struct {
	Scope                string `json:"scope,omitempty"`
	WorkspaceID          string `json:"workspace_id,omitempty"`
	Status               string `json:"status,omitempty"`
	Priority             string `json:"priority,omitempty"`
	IncludeDrafts        bool   `json:"include_drafts,omitempty"`
	ApprovalState        string `json:"approval_state,omitempty"`
	OwnerKind            string `json:"owner_kind,omitempty"`
	OwnerRef             string `json:"owner_ref,omitempty"`
	ParentTaskID         string `json:"parent_task_id,omitempty"`
	ParticipationChannel string `json:"participation_channel,omitempty"`
	Search               string `json:"search,omitempty"`
	Sort                 string `json:"sort,omitempty"`
	Cursor               string `json:"cursor,omitempty"`
	Limit                int    `json:"limit,omitempty"`
}

func (n *daemonNativeTools) taskList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input taskListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	actor, err := actorContextFromScope(scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query := input.query()
	if strings.TrimSpace(scope.WorkspaceID) != "" {
		workspaceID, workspaceErr := nativeCallerWorkspaceInput(
			req.ToolID,
			"workspace_id",
			input.WorkspaceID,
			scope,
		)
		if workspaceErr != nil {
			return toolspkg.ToolResult{}, workspaceErr
		}
		query.Scope = taskpkg.CatalogScopeWorkspace
		query.WorkspaceID = workspaceID
	}
	page, err := n.deps.Tasks.ListTaskCatalog(ctx, query, actor)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	response := core.TaskCatalogResponseFromPage(page)
	return structuredResult(response, fmt.Sprintf("%d of %d tasks", len(response.Tasks), response.Page.Total))
}

func (i taskListInput) query() taskpkg.CatalogQuery {
	return taskpkg.CatalogQuery{
		Scope:                taskpkg.CatalogScope(strings.TrimSpace(i.Scope)),
		WorkspaceID:          strings.TrimSpace(i.WorkspaceID),
		Status:               taskpkg.Status(strings.TrimSpace(i.Status)),
		Priority:             taskpkg.Priority(strings.TrimSpace(i.Priority)),
		IncludeDrafts:        i.IncludeDrafts,
		ApprovalState:        taskpkg.ApprovalState(strings.TrimSpace(i.ApprovalState)),
		OwnerKind:            taskpkg.OwnerKind(strings.TrimSpace(i.OwnerKind)),
		OwnerRef:             strings.TrimSpace(i.OwnerRef),
		ParentTaskID:         strings.TrimSpace(i.ParentTaskID),
		ParticipationChannel: strings.TrimSpace(i.ParticipationChannel),
		Search:               strings.TrimSpace(i.Search),
		Sort:                 taskpkg.CatalogSort(strings.TrimSpace(i.Sort)),
		Cursor:               strings.TrimSpace(i.Cursor),
		Limit:                i.Limit,
	}
}
