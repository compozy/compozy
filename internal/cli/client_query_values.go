package cli

import (
	"errors"
	"fmt"

	"net/url"

	"strconv"
	"strings"
)

func sessionRepairValues(query SessionRepairQuery) url.Values {
	values := url.Values{}
	if query.DryRun {
		values.Set("dry_run", "true")
	}
	if query.Force {
		values.Set("force", "true")
	}
	return values
}

func sessionInspectValues(query SessionInspectQuery) url.Values {
	values := url.Values{}
	if query.IncludeRecentWakeEvents {
		values.Set("include_recent_wake_events", "true")
	}
	return values
}

func networkPeersValues(query NetworkPeersQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Channel); trimmed != "" {
		values.Set("channel", trimmed)
	}
	return values
}

func networkConversationMessagesValues(query NetworkConversationMessagesQuery) url.Values {
	values := networkListValues(query.Limit, query.After)
	if trimmed := strings.TrimSpace(query.Before); trimmed != "" {
		values.Set("before", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Kind); trimmed != "" {
		values.Set("kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.WorkID); trimmed != "" {
		values.Set("work_id", trimmed)
	}
	return values
}

func networkListValues(limit int, after string) url.Values {
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if trimmed := strings.TrimSpace(after); trimmed != "" {
		values.Set("after", trimmed)
	}
	return values
}

func networkBasePath(workspaceRef string) (string, error) {
	workspaceRef, err := requireNetworkPathValue("workspace_id", workspaceRef)
	if err != nil {
		return "", err
	}
	return "/api/workspaces/" + url.PathEscape(workspaceRef) + "/network", nil
}

func hooksBasePath(workspaceRef string) (string, error) {
	workspaceRef, err := requireNetworkPathValue("workspace_id", workspaceRef)
	if err != nil {
		return "", err
	}
	return "/api/workspaces/" + url.PathEscape(workspaceRef) + "/hooks", nil
}

func networkChannelPath(workspaceRef string, channel string) (string, error) {
	base, err := networkBasePath(workspaceRef)
	if err != nil {
		return "", err
	}
	channel, err = requireNetworkPathValue("channel", channel)
	if err != nil {
		return "", err
	}
	return base + "/channels/" + url.PathEscape(channel), nil
}

func networkThreadPath(workspaceRef string, channel string, threadID string) (string, error) {
	path, err := networkChannelPath(workspaceRef, channel)
	if err != nil {
		return "", err
	}
	threadID, err = requireNetworkPathValue("thread_id", threadID)
	if err != nil {
		return "", err
	}
	return path + "/threads/" + url.PathEscape(threadID), nil
}

func networkThreadMessagesPath(workspaceRef string, channel string, threadID string) (string, error) {
	path, err := networkThreadPath(workspaceRef, channel, threadID)
	if err != nil {
		return "", err
	}
	return path + "/messages", nil
}

func networkDirectPath(workspaceRef string, channel string, directID string) (string, error) {
	path, err := networkChannelPath(workspaceRef, channel)
	if err != nil {
		return "", err
	}
	directID, err = requireNetworkPathValue("direct_id", directID)
	if err != nil {
		return "", err
	}
	return path + "/directs/" + url.PathEscape(directID), nil
}

func networkDirectMessagesPath(workspaceRef string, channel string, directID string) (string, error) {
	path, err := networkDirectPath(workspaceRef, channel, directID)
	if err != nil {
		return "", err
	}
	return path + "/messages", nil
}

func requireNetworkPathValue(name string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("cli: %s is required", name)
	}
	return trimmed, nil
}

func networkInboxValues(sessionID string) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	return values
}

func networkSubscriptionsValues(query NetworkSubscriptionsQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.ThreadID); trimmed != "" {
		values.Set("thread_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.SessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func vaultListValues(query VaultListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Prefix); trimmed != "" {
		values.Set("prefix", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Namespace); trimmed != "" {
		values.Set("namespace", trimmed)
	}
	return values
}

func vaultRefValues(ref string) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(ref); trimmed != "" {
		values.Set("ref", trimmed)
	}
	return values
}

func requireVaultRef(ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errors.New("cli: vault ref is required")
	}
	return trimmed, nil
}

func agentSoulHistoryValues(request AgentSoulHistoryRequest) url.Values {
	values := agentValues(AgentQuery{Workspace: request.WorkspaceID})
	if request.Limit > 0 {
		values.Set("limit", strconv.Itoa(request.Limit))
	}
	if trimmed := strings.TrimSpace(request.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	return values
}

func agentHeartbeatHistoryValues(request AgentHeartbeatHistoryRequest) url.Values {
	values := agentValues(AgentQuery{Workspace: request.WorkspaceID})
	if request.Limit > 0 {
		values.Set("limit", strconv.Itoa(request.Limit))
	}
	if trimmed := strings.TrimSpace(request.Cursor); trimmed != "" {
		values.Set("cursor", trimmed)
	}
	return values
}

func agentHeartbeatStatusValues(request AgentHeartbeatStatusRequest) url.Values {
	values := agentValues(AgentQuery{Workspace: request.WorkspaceID})
	if trimmed := strings.TrimSpace(request.SessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if request.IncludeSessionHealth {
		values.Set("include_session_health", "true")
	}
	if request.IncludeRecentWakeEvents {
		values.Set("include_recent_wake_events", "true")
	}
	return values
}

func skillValues(query SkillQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ForAgent); trimmed != "" {
		values.Set("for_agent", trimmed)
	}
	return values
}

func resourceListValues(query ResourceListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(string(query.Kind)); trimmed != "" {
		values.Set("kind", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.ScopeKind)); trimmed != "" {
		values.Set("scope_kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.ScopeID); trimmed != "" {
		values.Set("scope_id", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.OwnerKind)); trimmed != "" {
		values.Set("owner_kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.OwnerID); trimmed != "" {
		values.Set("owner_id", trimmed)
	}
	if trimmed := strings.TrimSpace(string(query.SourceKind)); trimmed != "" {
		values.Set("source_kind", trimmed)
	}
	if trimmed := strings.TrimSpace(query.SourceID); trimmed != "" {
		values.Set("source_id", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func agentChannelRecvValues(query AgentChannelRecvQuery) url.Values {
	values := url.Values{}
	if query.Wait {
		values.Set("wait", "true")
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
