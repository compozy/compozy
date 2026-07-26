package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/session"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type sessionListInput struct {
	Workspace     string `json:"workspace,omitempty"`
	State         string `json:"state,omitempty"`
	Type          string `json:"type,omitempty"`
	Agent         string `json:"agent,omitempty"`
	Query         string `json:"q,omitempty"`
	Resumable     bool   `json:"resumable,omitempty"`
	IncludeHealth bool   `json:"include_health,omitempty"`
	Sort          string `json:"sort,omitempty"`
	Cursor        string `json:"cursor,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

func (n *daemonNativeTools) sessionList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceRef, err := nativeCallerWorkspaceInput(req.ToolID, "workspace", input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID := ""
	if workspaceRef != "" {
		workspaceID, err = n.workspaceID(ctx, workspaceRef)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
	}
	pager, ok := n.deps.Sessions.(core.SessionPageManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: paged session catalog is required")
	}
	page, err := pager.ListPage(ctx, session.ListQuery{
		WorkspaceID: workspaceID,
		State:       strings.TrimSpace(input.State),
		SessionType: session.Type(strings.TrimSpace(input.Type)),
		AgentName:   strings.TrimSpace(input.Agent),
		Search:      strings.TrimSpace(input.Query),
		Resumable:   input.Resumable,
		Sort:        strings.TrimSpace(input.Sort),
		Cursor:      strings.TrimSpace(input.Cursor),
		Limit:       input.Limit,
	})
	if err != nil {
		if errors.Is(err, session.ErrListQueryInvalid) || errors.Is(err, session.ErrListCursorInvalid) {
			return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadsFromInfos(page.Sessions)
	if input.IncludeHealth {
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
	response := contract.SessionCatalogResponse{
		Sessions: payload,
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}
	return structuredResult(response, fmt.Sprintf("%d of %d sessions", len(payload), page.Total))
}
