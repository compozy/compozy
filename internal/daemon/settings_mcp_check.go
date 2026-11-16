package daemon

import (
	"context"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ settingspkg.MCPRuntimeChecker = (*settingsRuntimeSurface)(nil)

func (s *settingsRuntimeSurface) CheckMCPServerRuntime(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (settingspkg.MCPServerRuntimeStatus, error) {
	key, tracked, err := s.settingsMCPDeadEntityKey(ctx, target, server.EffectiveRuntimeName())
	if err != nil {
		return settingspkg.MCPServerRuntimeStatus{}, err
	}
	if tracked && s.deadEntities != nil {
		ctx = deadentity.WithProbeRetry(ctx, key)
	}
	return s.MCPServerRuntimeStatus(toolspkg.WithFreshMCPDiscovery(ctx), target, server)
}
