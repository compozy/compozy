package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionListInput struct {
	BoardColumn   []string `json:"board_column,omitempty"`
	Children      string   `json:"children,omitempty"`
	Workspace     string   `json:"workspace,omitempty"`
	Worktree      string   `json:"worktree,omitempty"`
	State         string   `json:"state,omitempty"`
	Type          string   `json:"type,omitempty"`
	Agent         string   `json:"agent,omitempty"`
	Parent        string   `json:"parent,omitempty"`
	Root          string   `json:"root,omitempty"`
	Query         string   `json:"q,omitempty"`
	Resumable     bool     `json:"resumable,omitzero"`
	Archive       string   `json:"archive,omitempty"`
	Later         string   `json:"later,omitempty"`
	IncludeHealth bool     `json:"include_health,omitzero"`
	Sort          string   `json:"sort,omitempty"`
	Cursor        string   `json:"cursor,omitempty"`
	Limit         int      `json:"limit,omitzero"`
}

// sessionList returns the authorized session page with current profile labels and placements.
func (n *daemonNativeTools) sessionList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceRef := nativeCallerWorkspaceInput(input.Workspace, scope)
	workspaceID := ""
	if workspaceRef == workspaceaccess.UserSelector {
		workspaceID = workspaceaccess.UserSelector
	} else if workspaceRef != "" {
		resolvedWorkspaceID, err := n.workspaceID(ctx, workspaceRef)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		workspaceID = resolvedWorkspaceID
	}
	pager, ok := n.deps.Sessions.(core.SessionPageManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: paged session catalog is required")
	}
	readScope, err := n.nativeProfileReadScope(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	columns, err := session.ParseBoardColumns(input.BoardColumn)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	hideChildren, err := session.ParseChildrenFilter(input.Children)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	page, err := pager.ListPage(ctx, session.ListQuery{
		Columns:         columns,
		HideChildren:    hideChildren,
		ReadScope:       readScope,
		WorkspaceID:     workspaceID,
		WorktreeID:      strings.TrimSpace(input.Worktree),
		State:           strings.TrimSpace(input.State),
		SessionType:     session.Type(strings.TrimSpace(input.Type)),
		AgentName:       strings.TrimSpace(input.Agent),
		ParentSessionID: strings.TrimSpace(input.Parent),
		RootSessionID:   strings.TrimSpace(input.Root),
		Search:          strings.TrimSpace(input.Query),
		Resumable:       input.Resumable,
		Archive:         store.SessionArchiveFilter(strings.TrimSpace(input.Archive)),
		Later:           store.SessionLaterFilter(strings.TrimSpace(input.Later)),
		Sort:            strings.TrimSpace(input.Sort),
		Cursor:          strings.TrimSpace(input.Cursor),
		Limit:           input.Limit,
	})
	if err != nil {
		if errors.Is(err, session.ErrListQueryInvalid) || errors.Is(err, session.ErrListCursorInvalid) {
			return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, err
	}
	return n.sessionListPayload(ctx, req, input.IncludeHealth, page)
}

func (n *daemonNativeTools) sessionListPayload(
	ctx context.Context,
	req toolspkg.CallRequest,
	includeHealth bool,
	page session.ListPage,
) (toolspkg.ToolResult, error) {
	var err error
	payload := core.SessionPayloadsFromInfos(page.Sessions)
	if includeHealth {
		pageReader, ok := n.deps.Sessions.(core.SessionHealthPageReader)
		if !ok {
			return toolspkg.ToolResult{}, nativeUnavailableError(
				req.ToolID,
				"session health page capability is unavailable",
			)
		}
		payload, err = core.SessionPayloadsWithPageHealth(ctx, page.Sessions, pageReader)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	if err := core.DecorateSessionPullRequests(ctx, n.deps.Sessions, page.Sessions, payload); err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err = core.SessionPayloadsWithProfileOwners(ctx, payload, n.deps.Profiles)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	core.ApplySessionPlacements(payload, page.Placements)
	response := contract.SessionCatalogResponse{
		Sessions: payload,
		Page: contract.SessionCatalogPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      &page.Total,
			Limit:      page.Limit,
		},
	}
	return structuredResult(response, fmt.Sprintf("%d of %d sessions", len(payload), page.Total))
}
