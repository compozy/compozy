package daemon

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) networkSubscriptions(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkSubscriptionsInput
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
	query := store.NetworkSubscriptionQuery{
		WorkspaceID: workspaceID,
		Channel:     channel,
		ThreadID:    strings.TrimSpace(input.ThreadID),
		SessionID:   strings.TrimSpace(input.SessionID),
		Limit:       input.Limit,
	}
	if query.Limit == 0 {
		query.Limit = 100
	}
	if err := query.Validate(); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	subscriptions, err := n.deps.NetworkStore.ListNetworkSubscriptions(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	payload := core.NetworkSubscriptionPayloadsFromStore(subscriptions)
	return structuredNetworkResult(
		map[string]any{"subscriptions": payload},
		fmt.Sprintf("%d subscriptions", len(payload)),
	)
}

func (n *daemonNativeTools) networkSubscribe(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.networkSetSubscription(ctx, scope, req, string(store.NetworkSubscriptionModeFull))
}

func (n *daemonNativeTools) networkMute(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.networkSetSubscription(ctx, scope, req, string(store.NetworkSubscriptionModeMute))
}

func (n *daemonNativeTools) networkSetSubscription(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	mode string,
) (toolspkg.ToolResult, error) {
	var input networkSubscriptionInput
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
	now := time.Now().UTC()
	entry := store.NetworkSubscriptionEntry{
		WorkspaceID: workspaceID,
		Channel:     channel,
		ThreadID:    strings.TrimSpace(input.ThreadID),
		SessionID:   strings.TrimSpace(input.SessionID),
		Mode:        mode,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := entry.Validate(); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	channelEntry, err := nativeNetworkSubscriptionChannel(scope, entry)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if err := n.deps.NetworkStore.PutNetworkSubscriptionWithChannel(ctx, channelEntry, entry); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	payload := core.NetworkSubscriptionPayloadFromStore(entry)
	return structuredNetworkResult(map[string]any{"subscription": payload}, payload.Mode)
}

func nativeNetworkSubscriptionChannel(
	scope toolspkg.Scope,
	entry store.NetworkSubscriptionEntry,
) (store.NetworkChannelEntry, error) {
	ref := store.NetworkChannelRef{
		WorkspaceID: strings.TrimSpace(entry.WorkspaceID),
		Channel:     strings.TrimSpace(entry.Channel),
	}
	if err := ref.Validate(); err != nil {
		return store.NetworkChannelEntry{}, err
	}
	now := time.Now().UTC()
	return store.NetworkChannelEntry{
		WorkspaceID:  ref.WorkspaceID,
		Channel:      ref.Channel,
		Purpose:      "network_channel",
		FanoutPolicy: store.NetworkFanoutPolicyCapabilityMatch,
		CreatedBy:    strings.TrimSpace(scope.AgentName),
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func (n *daemonNativeTools) networkUnmute(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkSubscriptionDeleteInput
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
	ref := store.NetworkSubscriptionRef{
		WorkspaceID: workspaceID,
		Channel:     channel,
		ThreadID:    strings.TrimSpace(input.ThreadID),
		SessionID:   strings.TrimSpace(input.SessionID),
	}
	if err := ref.Validate(); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	if err := n.deps.NetworkStore.DeleteNetworkSubscription(ctx, ref); err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	return structuredNetworkResult(map[string]any{"deleted": true}, "deleted")
}
