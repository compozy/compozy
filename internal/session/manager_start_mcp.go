package session

import (
	"context"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (m *Manager) sessionMCPServers(
	ctx context.Context,
	spec *sessionStartSpec,
	resolved compozyconfig.ResolvedAgent,
	agentDef compozyconfig.AgentDef,
) ([]compozyconfig.MCPServer, error) {
	if strings.EqualFold(spec.runtimeMode, RuntimeModeVerdictOnly) {
		return nil, nil
	}
	if !resolved.SessionMCP {
		spec.startLogger(m).Info(
			"session.mcp.skipped",
			"reason", "provider_session_mcp_disabled",
			"resolved_agent_name", strings.TrimSpace(resolved.Name),
			"resolved_provider", strings.TrimSpace(resolved.Provider),
		)
		return nil, nil
	}
	if m.hostedMCP == nil {
		spec.startLogger(m).Warn(
			"session.mcp.hosted_mcp_unavailable",
			"reason", "hosted_mcp_launcher_unavailable",
			"resolved_agent_name", strings.TrimSpace(resolved.Name),
			"resolved_provider", strings.TrimSpace(resolved.Provider),
			"configured_mcp_servers", len(resolved.MCPServers),
		)
		return m.resolveStartMCPServers(ctx, &spec.workspace, agentDef, resolved.MCPServers)
	}
	hosted, err := m.hostedMCP.Launch(ctx, HostedMCPLaunchRequest{
		SessionID: spec.sessionID, WorkspaceID: spec.workspace.ID, AgentName: resolved.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("session: mint hosted MCP launch for %q: %w", spec.sessionID, err)
	}
	return []compozyconfig.MCPServer{hosted}, nil
}
