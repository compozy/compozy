package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionListInput struct {
	Workspace     string `json:"workspace,omitempty"`
	Worktree      string `json:"worktree,omitempty"`
	State         string `json:"state,omitempty"`
	Type          string `json:"type,omitempty"`
	Agent         string `json:"agent,omitempty"`
	Parent        string `json:"parent,omitempty"`
	Root          string `json:"root,omitempty"`
	Query         string `json:"q,omitempty"`
	Resumable     bool   `json:"resumable,omitempty"`
	Archive       string `json:"archive,omitempty"`
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
	workspaceRef := nativeCallerWorkspaceInput(input.Workspace, scope)
	workspaceID := ""
	if workspaceRef != "" {
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
	page, err := pager.ListPage(ctx, session.ListQuery{
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
		Sort:            strings.TrimSpace(input.Sort),
		Cursor:          strings.TrimSpace(input.Cursor),
		Limit:           input.Limit,
	})
	if err != nil {
		if errors.Is(err, session.ErrListQueryInvalid) || errors.Is(err, session.ErrListCursorInvalid) {
			return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, err
	}
	payload, err := n.nativeSessionListPayload(ctx, req.ToolID, page.Sessions, input.IncludeHealth)
	if err != nil {
		return toolspkg.ToolResult{}, err
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

func (n *daemonNativeTools) nativeSessionListPayload(
	ctx context.Context,
	toolID toolspkg.ToolID,
	infos []*session.Info,
	includeHealth bool,
) ([]contract.SessionPayload, error) {
	payload := core.SessionPayloadsForAgentFromInfos(infos)
	if !includeHealth {
		return payload, nil
	}
	pageReader, ok := n.deps.Sessions.(core.SessionHealthPageReader)
	if !ok {
		return nil, nativeUnavailableError(toolID, "session health page capability is unavailable")
	}
	healthPayload, err := core.SessionPayloadsWithPageHealth(ctx, infos, pageReader)
	if err != nil {
		return nil, err
	}
	if len(healthPayload) != len(payload) {
		return nil, errors.New("daemon: session health projection changed page cardinality")
	}
	for index := range payload {
		if payload[index].ID != healthPayload[index].ID {
			return nil, errors.New("daemon: session health projection changed page order")
		}
		payload[index].Badge = healthPayload[index].Badge
		payload[index].Health = healthPayload[index].Health
	}
	return payload, nil
}
