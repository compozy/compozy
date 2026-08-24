package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) sessionMCPServers(
	ctx context.Context,
	spec *sessionStartSpec,
	resolved compozyconfig.ResolvedAgent,
	agentDef compozyconfig.AgentDef,
) ([]compozyconfig.MCPServer, error) {
	required := sessionRequiresHostedMCP(spec, resolved)
	if strings.EqualFold(spec.runtimeMode, RuntimeModeVerdictOnly) {
		if required {
			return nil, fmt.Errorf("%w: verdict-only runtime", ErrHostedMCPUnavailable)
		}
		return nil, nil
	}
	if !resolved.SessionMCP {
		if required {
			return nil, fmt.Errorf("%w: provider %q disables session MCP", ErrHostedMCPUnavailable, resolved.Provider)
		}
		spec.startLogger(m).Info(
			"session.mcp.skipped",
			"reason", "provider_session_mcp_disabled",
			"resolved_agent_name", strings.TrimSpace(resolved.Name),
			"resolved_provider", strings.TrimSpace(resolved.Provider),
		)
		return nil, nil
	}
	if m.hostedMCP == nil {
		if required {
			return nil, fmt.Errorf("%w: hosted MCP launcher is not configured", ErrHostedMCPUnavailable)
		}
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
		SessionID: spec.sessionID, ProfileID: spec.profileID,
		WorkspaceID: spec.workspace.ID, AgentName: resolved.Name,
	})
	if err != nil {
		return nil, errors.Join(
			ErrHostedMCPUnavailable,
			fmt.Errorf("session: mint hosted MCP launch for %q: %w", spec.sessionID, err),
		)
	}
	return []compozyconfig.MCPServer{hosted}, nil
}

func sessionRequiresHostedMCP(spec *sessionStartSpec, resolved compozyconfig.ResolvedAgent) bool {
	if len(resolved.Tools) > 0 || len(resolved.Toolsets) > 0 {
		return true
	}
	if spec == nil {
		return false
	}
	lineage := store.NormalizeSessionLineage(spec.sessionID, spec.lineage)
	policy := store.NormalizeSessionPermissionPolicy(lineage.PermissionPolicy)
	return len(policy.Tools) > 0
}

func (m *Manager) sessionMCPServerActivator(
	sessionID string,
	servers []compozyconfig.MCPServer,
) func(context.Context) error {
	if m.hostedMCP == nil || len(servers) == 0 {
		return nil
	}
	return func(ctx context.Context) error {
		if err := m.hostedMCP.ArmLaunch(ctx, sessionID); err != nil {
			return fmt.Errorf("session: arm hosted MCP launch for %q: %w", sessionID, err)
		}
		return nil
	}
}
