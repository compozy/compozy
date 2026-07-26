package daemon

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/store"

	toolspkg "github.com/compozy/agh/internal/tools"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type nativeBoundSessionScope struct {
	sessionID       string
	workspaceID     string
	networkChannels map[string]struct{}
}

func (n *daemonNativeTools) nativeBoundSession(
	ctx context.Context,
	scope toolspkg.Scope,
) (*nativeBoundSessionScope, error) {
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return nil, nil
	}
	if n == nil || n.deps == nil || n.deps.Sessions == nil {
		return nil, errors.New("daemon: sessions are required")
	}
	info, err := n.deps.Sessions.Status(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	bound := &nativeBoundSessionScope{
		sessionID:       strings.TrimSpace(info.ID),
		workspaceID:     strings.TrimSpace(info.WorkspaceID),
		networkChannels: make(map[string]struct{}),
	}
	lineage := store.NormalizeSessionLineage(info.ID, info.Lineage)
	if lineage != nil {
		for _, channel := range lineage.PermissionPolicy.NetworkChannels {
			bound.networkChannels[channel] = struct{}{}
		}
	}
	return bound, nil
}

func nativeBoundWorkspaceRef(bound *nativeBoundSessionScope, workspaceRef string) string {
	if bound == nil {
		return workspaceRef
	}
	return bound.workspaceID
}

func nativeBoundSessionID(bound *nativeBoundSessionScope, values ...string) string {
	if bound == nil {
		return firstNonEmpty(values...)
	}
	return bound.sessionID
}

func nativeBoundSessionAllowsChannel(bound *nativeBoundSessionScope, channel string) bool {
	if bound == nil || len(bound.networkChannels) == 0 {
		return true
	}
	_, ok := bound.networkChannels[strings.TrimSpace(channel)]
	return ok
}

func nativeFilterNetworkChannelPayloads(
	bound *nativeBoundSessionScope,
	payload []contract.NetworkChannelPayload,
) []contract.NetworkChannelPayload {
	if bound == nil || len(bound.networkChannels) == 0 {
		return payload
	}
	filtered := make([]contract.NetworkChannelPayload, 0, len(payload))
	for _, channel := range payload {
		if nativeBoundSessionAllowsChannel(bound, channel.Channel) {
			filtered = append(filtered, channel)
		}
	}
	return filtered
}

func nativeFilterNetworkEnvelopes(
	bound *nativeBoundSessionScope,
	messages []network.Envelope,
) []network.Envelope {
	if bound == nil || len(bound.networkChannels) == 0 {
		return messages
	}
	filtered := make([]network.Envelope, 0, len(messages))
	for _, message := range messages {
		if nativeBoundSessionAllowsChannel(bound, message.Channel) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func nativeBoundChannelDenied(id toolspkg.ToolID, channel string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q denied for channel %q", id, strings.TrimSpace(channel)),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonSessionDenied,
	)
}

func (n *daemonNativeTools) nativeResolvedWorkspace(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceRef string,
	scope toolspkg.Scope,
) (workspacepkg.ResolvedWorkspace, error) {
	ref, err := nativeCallerWorkspaceInput(id, "workspace_id", workspaceRef, scope)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	if ref == "" {
		return workspacepkg.ResolvedWorkspace{}, nativeRequiredInputError(id, "workspace_id")
	}
	if n == nil || n.deps == nil || n.deps.Workspaces == nil {
		return workspacepkg.ResolvedWorkspace{}, nativeNetworkInputError(
			id,
			workspacepkg.ErrWorkspaceResolverUnavailable,
		)
	}
	resolved, err := n.deps.Workspaces.Resolve(ctx, ref)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, nativeNetworkInputError(id, err)
	}
	return resolved, nil
}

func nativeCallerWorkspaceInput(
	id toolspkg.ToolID,
	field string,
	value string,
	scope toolspkg.Scope,
) (string, error) {
	trusted := strings.TrimSpace(scope.WorkspaceID)
	current := strings.TrimSpace(value)
	if scope.Operator {
		if current == "" {
			return trusted, nil
		}
		return current, nil
	}
	if trusted == "" {
		return current, nil
	}
	if current == "" {
		return trusted, nil
	}
	if current != trusted {
		return "", nativeScopeMismatchError(id, field)
	}
	return current, nil
}

func nativeScopeMismatchError(id toolspkg.ToolID, field string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q call %s conflicts with caller scope", id, field),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonScopeMismatch,
	)
}

func (n *daemonNativeTools) nativeNetworkWorkspaceID(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceRef string,
	scope toolspkg.Scope,
) (string, error) {
	resolved, err := n.nativeResolvedWorkspace(ctx, id, workspaceRef, scope)
	if err != nil {
		return "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return "", nativeNetworkInputError(id, err)
	}
	return workspaceID, nil
}

func nativeResolvedNetworkWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) (string, error) {
	if resolved == nil {
		return "", errors.New("daemon: resolved workspace is required")
	}
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		return "", errors.New("daemon: resolved workspace_id is empty")
	}
	return workspaceID, nil
}

func nativeResolvedRegistryWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) (string, error) {
	if resolved == nil {
		return "", errors.New("daemon: resolved workspace is required")
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		return "", errors.New("daemon: resolved workspace registry id is empty")
	}
	return workspaceID, nil
}

func (n *daemonNativeTools) requireNativeSessionWorkspace(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceID string,
	sessionID string,
) error {
	_, err := n.nativeSessionInWorkspace(ctx, id, workspaceID, sessionID)
	return err
}

func (n *daemonNativeTools) nativeSessionInWorkspace(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nativeRequiredInputError(id, "session_id")
	}
	if n == nil || n.deps == nil || n.deps.Sessions == nil {
		return nil, errors.New("daemon: sessions are required")
	}
	info, err := n.deps.Sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if info == nil || strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return nil, fmt.Errorf("%w: session=%q workspace_id=%q", session.ErrSessionNotFound, sessionID, workspaceID)
	}
	return info, nil
}

func nativeNetworkChannel(id toolspkg.ToolID, value string) (string, error) {
	channel := strings.TrimSpace(value)
	if err := network.ValidateChannel(channel); err != nil {
		return "", nativeNetworkInputError(id, err)
	}
	return channel, nil
}

func trimNativeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if next := strings.TrimSpace(value); next != "" {
			trimmed = append(trimmed, next)
		}
	}
	return trimmed
}

func decodeSessionEventQueryInput(req toolspkg.CallRequest) (sessionEventQueryInput, store.EventQuery, error) {
	var input sessionEventQueryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return sessionEventQueryInput{}, store.EventQuery{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return sessionEventQueryInput{}, store.EventQuery{}, err
	}
	input.SessionID = sessionID
	query, err := input.eventQuery(req.ToolID)
	if err != nil {
		return sessionEventQueryInput{}, store.EventQuery{}, err
	}
	return input, query, nil
}
