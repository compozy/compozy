package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type networkThreadMessagesInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	ThreadID    string `json:"thread_id"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	Kind        string `json:"kind,omitempty"`
	WorkID      string `json:"work_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type networkDirectMessagesInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	DirectID    string `json:"direct_id"`
	Before      string `json:"before,omitempty"`
	After       string `json:"after,omitempty"`
	Kind        string `json:"kind,omitempty"`
	WorkID      string `json:"work_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type networkConversationMessageQueryInput struct {
	Before string
	After  string
	Kind   string
	WorkID string
	Limit  int
}

type networkConversationMessagePage struct {
	Messages []contract.NetworkConversationMessagePayload
	Page     contract.CursorPagePayload
}

func (n *daemonNativeTools) networkThreadMessages(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkThreadMessagesInput
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
	page, err := n.networkConversationMessages(ctx, req.ToolID, store.NetworkConversationRef{
		WorkspaceID: workspaceID,
		Channel:     channel,
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    strings.TrimSpace(input.ThreadID),
	}, networkConversationMessageQueryInput{
		Before: strings.TrimSpace(input.Before),
		After:  strings.TrimSpace(input.After),
		Kind:   strings.TrimSpace(input.Kind),
		WorkID: strings.TrimSpace(input.WorkID),
		Limit:  input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(
		map[string]any{nativeToolsMessagesKey: page.Messages, nativeListPageKey: page.Page},
		fmt.Sprintf("%d messages", len(page.Messages)),
	)
}

func (n *daemonNativeTools) networkDirectMessages(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkDirectMessagesInput
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
	page, err := n.networkConversationMessages(ctx, req.ToolID, store.NetworkConversationRef{
		WorkspaceID: workspaceID,
		Channel:     channel,
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    strings.TrimSpace(input.DirectID),
	}, networkConversationMessageQueryInput{
		Before: strings.TrimSpace(input.Before),
		After:  strings.TrimSpace(input.After),
		Kind:   strings.TrimSpace(input.Kind),
		WorkID: strings.TrimSpace(input.WorkID),
		Limit:  input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredNetworkResult(
		map[string]any{nativeToolsMessagesKey: page.Messages, nativeListPageKey: page.Page},
		fmt.Sprintf("%d messages", len(page.Messages)),
	)
}

func (n *daemonNativeTools) networkConversationMessages(
	ctx context.Context,
	id toolspkg.ToolID,
	ref store.NetworkConversationRef,
	input networkConversationMessageQueryInput,
) (networkConversationMessagePage, error) {
	if err := ref.Validate(); err != nil {
		return networkConversationMessagePage{}, nativeNetworkInputError(id, err)
	}
	limit := store.NormalizeNetworkMessageLimit(input.Limit)
	query := store.NetworkConversationMessageQuery{
		BeforeMessageID: strings.TrimSpace(input.Before),
		AfterMessageID:  strings.TrimSpace(input.After),
		Kind:            strings.TrimSpace(input.Kind),
		WorkID:          strings.TrimSpace(input.WorkID),
		Limit:           limit + 1,
	}
	if err := query.Validate(); err != nil {
		return networkConversationMessagePage{}, nativeNetworkInputError(id, err)
	}
	messages, err := n.deps.NetworkStore.ListConversationMessages(ctx, ref, query)
	if err != nil {
		return networkConversationMessagePage{}, err
	}
	payload := core.NetworkConversationMessagePayloadsFromStore(messages)
	payload, page := trimNativeNetworkMessagePage(payload, input.After, limit)
	return networkConversationMessagePage{Messages: payload, Page: page}, nil
}

func trimNativeNetworkMessagePage(
	messages []contract.NetworkConversationMessagePayload,
	after string,
	limit int,
) ([]contract.NetworkConversationMessagePayload, contract.CursorPagePayload) {
	page := contract.CursorPagePayload{Limit: limit}
	if len(messages) <= limit {
		return messages, page
	}
	page.HasMore = true
	if strings.TrimSpace(after) != "" {
		messages = messages[:limit]
		page.NextCursor = strings.TrimSpace(messages[len(messages)-1].MessageID)
		return messages, page
	}
	messages = messages[len(messages)-limit:]
	page.NextCursor = strings.TrimSpace(messages[0].MessageID)
	return messages, page
}
