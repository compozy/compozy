package daemon

import (
	"context"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (s *settingsRuntimeSurface) suppressedMCPAuthRuntimeStatus(
	ctx context.Context, key store.DeadEntityKey, tracked bool,
	target mcpauth.Target, server compozyconfig.MCPServer,
) (*settingspkg.MCPServerRuntimeStatus, error) {
	status, err := s.suppressedMCPRuntimeStatus(ctx, key, tracked)
	if err != nil || status == nil || !toolspkg.MCPAuthenticationFailure(status.Diagnostic) ||
		!mcpauth.OAuthCapable(server) {
		return status, err
	}
	authStatus, authErr := s.MCPAuthStatus(ctx, target, server)
	if authErr != nil || !authStatus.TokenPresent {
		return status, authErr
	}
	if authStatus.Status == mcpauth.StatusExpired && authStatus.Refreshable {
		return nil, nil
	}
	if authStatus.Status == mcpauth.StatusAuthenticated {
		return nil, clearMCPAuthDeadEntity(ctx, s.deadEntities, key)
	}
	return status, nil
}

func clearMCPAuthDeadEntity(ctx context.Context, service *deadentity.Service, key store.DeadEntityKey) error {
	if service == nil || key.Validate() != nil {
		return nil
	}
	status, err := service.Status(ctx, key)
	if err != nil {
		return fmt.Errorf("daemon: read MCP authentication recovery state: %w", err)
	}
	if !status.Dead || !toolspkg.MCPAuthenticationFailure(status.Reason) {
		return nil
	}
	return service.RecordSuccess(ctx, key)
}

func (s *settingsRuntimeSurface) recoverMCPAuthentication(
	ctx context.Context, target mcpauth.Target, server compozyconfig.MCPServer,
) error {
	key, tracked, err := s.settingsMCPDeadEntityKey(ctx, target, server.EffectiveRuntimeName())
	if err != nil || !tracked {
		return err
	}
	return clearMCPAuthDeadEntity(ctx, s.deadEntities, key)
}

func recoverResolvedMCPAuthentication(ctx context.Context, state *bootState, resolved *mcppkg.ResolvedServer) error {
	lens := resolved.ConsentScope
	key := store.DeadEntityKey{
		ProfileID: lens.ProfileID, WorkspaceID: lens.WorkspaceID,
		Kind: store.DeadEntityKindMCPSidecar, EntityID: resolved.Server.EffectiveRuntimeName(),
	}.Normalize()
	return clearMCPAuthDeadEntity(ctx, state.deadEntities, key)
}
