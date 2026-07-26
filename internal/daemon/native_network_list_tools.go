package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type networkThreadsInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	Query       string `json:"query,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Sort        string `json:"sort,omitempty"`
	HasWork     *bool  `json:"has_work,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	After       string `json:"after,omitempty"`
}

type networkDirectsInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	Query       string `json:"query,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Sort        string `json:"sort,omitempty"`
	HasWork     *bool  `json:"has_work,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	After       string `json:"after,omitempty"`
}

func (n *daemonNativeTools) networkThreads(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkThreadsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	channel, err := nativeNetworkChannel(req.ToolID, input.Channel)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query := store.NetworkThreadQuery{
		Search:    strings.TrimSpace(input.Query),
		SessionID: strings.TrimSpace(input.SessionID),
		Sort:      strings.TrimSpace(input.Sort),
		HasWork:   input.HasWork,
		Limit:     input.Limit,
		After:     strings.TrimSpace(input.After),
	}
	if err := query.Validate(); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	page, err := n.deps.NetworkStore.ListThreads(
		ctx,
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel},
		query,
	)
	if err != nil {
		if errors.Is(err, store.ErrNetworkCursorInvalid) {
			return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, err
	}
	payload := core.NetworkThreadSummaryPayloadsFromStore(page.Threads)
	return structuredNetworkResult(
		map[string]any{
			nativeListPageKey: contract.CountedCursorPagePayload{
				NextCursor: page.NextCursor,
				HasMore:    page.HasMore,
				Total:      page.Total,
				Limit:      page.Limit,
			},
			"threads": payload,
		},
		fmt.Sprintf("%d threads", len(payload)),
	)
}

func (n *daemonNativeTools) networkDirects(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkDirectsInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	channel, err := nativeNetworkChannel(req.ToolID, input.Channel)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query := store.NetworkDirectRoomQuery{
		Search:    strings.TrimSpace(input.Query),
		SessionID: strings.TrimSpace(input.SessionID),
		Sort:      strings.TrimSpace(input.Sort),
		HasWork:   input.HasWork,
		Limit:     input.Limit,
		After:     strings.TrimSpace(input.After),
	}
	if err := query.Validate(); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	page, err := n.deps.NetworkStore.ListDirectRooms(
		ctx,
		store.NetworkChannelRef{WorkspaceID: workspaceID, Channel: channel},
		query,
	)
	if err != nil {
		if errors.Is(err, store.ErrNetworkCursorInvalid) {
			return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
		}
		return toolspkg.ToolResult{}, err
	}
	payload := core.NetworkDirectRoomPayloadsFromStore(page.Directs)
	return structuredNetworkResult(
		map[string]any{
			"directs": payload,
			nativeListPageKey: contract.CountedCursorPagePayload{
				NextCursor: page.NextCursor,
				HasMore:    page.HasMore,
				Total:      page.Total,
				Limit:      page.Limit,
			},
		},
		fmt.Sprintf("%d directs", len(payload)),
	)
}
