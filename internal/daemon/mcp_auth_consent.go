package daemon

import (
	"context"
	"fmt"
	"strings"

	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func authorizeMCPAuthNetwork(ctx context.Context, state *bootState, cfg mcpauth.ServerConfig) error {
	target := cfg.Target.Normalize()
	source := toolspkg.SourceRef{
		Kind:          toolspkg.SourceMCP,
		Owner:         target.ServerName,
		RawServerName: target.ServerName,
		ProfileID:     store.DefaultProfileID,
	}
	switch target.Scope {
	case mcpauth.ScopeWorkspace:
		source.WorkspaceID = target.WorkspaceID
	case mcpauth.ScopeWorkspaceProfile:
		workspace, profile, _ := strings.Cut(target.WorkspaceID, "@pf:")
		source.WorkspaceID = workspace
		id, err := (settingsMCPConsent{state: state}).profileID(ctx, profile)
		if err != nil {
			return err
		}
		source.ProfileID = id
	case mcpauth.ScopeProfile:
		id, err := (settingsMCPConsent{state: state}).profileID(ctx, target.WorkspaceID)
		if err != nil {
			return err
		}
		source.ProfileID = id
	}
	lens := cfg.ConsentScope
	if lens.ProfileID != "" {
		source.ProfileID = lens.ProfileID
	}
	if lens.ProfileName != "" {
		id, err := (settingsMCPConsent{state: state}).profileID(ctx, lens.ProfileName)
		if err != nil {
			return err
		}
		source.ProfileID = id
	}
	if lens.WorkspaceID != "" {
		source.WorkspaceID = lens.WorkspaceID
	}
	source.ResourceID = lens.ResourceID
	resolved, err := resolveDaemonMCPServer(ctx, state, source, target.Owner)
	if err != nil {
		return err
	}
	if resolved.Server.IsDisabled() {
		return &settingspkg.MCPApprovalError{
			Code: "mcp_server_disabled", Message: fmt.Sprintf("MCP server %q is off", target.ServerName),
		}
	}
	req := settingsConsentRequest(resolved.Target, resolved.Server)
	if state.profiles != nil {
		req.ProfileName, err = state.profiles.ProfileName(ctx, source.ProfileID)
		if err != nil {
			return err
		}
	}
	decision, err := (settingsMCPConsent{state: state}).MCPConsent(ctx, req)
	if err != nil {
		return err
	}
	if !decision.Admitted {
		return waitingDaemonMCPApprovalError(ctx, state, target.ServerName, source.WorkspaceID)
	}
	digest, err := state.mcpDefinitionDigest(resolved.Server)
	if err != nil {
		return err
	}
	if cfg.DefinitionDigest != "" && cfg.DefinitionDigest != digest {
		return &settingspkg.MCPApprovalError{
			Code: "mcp_approval_definition_changed",
			Message: fmt.Sprintf(
				"%s changed since you looked at it. List it again and approve the new definition",
				target.ServerName,
			),
		}
	}
	return nil
}

func requireResolvedMCPConsent(
	ctx context.Context,
	state *bootState,
	source toolspkg.SourceRef,
	resolved *mcppkg.ResolvedServer,
) error {
	req := settingsConsentRequest(resolved.Target, resolved.Server)
	if source.ProfileID != "" && state.profiles != nil {
		var err error
		req.ProfileName, err = state.profiles.ProfileName(ctx, source.ProfileID)
		if err != nil {
			return err
		}
	}
	decision, err := (settingsMCPConsent{state: state}).MCPConsent(ctx, req)
	if err != nil {
		return err
	}
	if !decision.Admitted {
		return waitingDaemonMCPApprovalError(ctx, state, resolved.Server.Name, req.WorkspaceID)
	}
	return nil
}
