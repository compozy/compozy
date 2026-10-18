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

func hooksBasePath(workspaceRef string) (string, error) {
	workspaceRef, err := requirePathValue("workspace_id", workspaceRef)
	if err != nil {
		return "", err
	}
	return "/api/workspaces/" + url.PathEscape(workspaceRef) + "/hooks", nil
}

func requirePathValue(name string, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("cli: %s is required", name)
	}
	return trimmed, nil
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

func skillDetailValues(query SkillQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.Workspace); trimmed != "" {
		values.Set("workspace_id", trimmed)
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
