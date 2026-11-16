package daemon

import (
	"context"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/deadentity"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

var _ settingspkg.MCPRuntimeChecker = (*settingsRuntimeSurface)(nil)

func (s *settingsRuntimeSurface) CheckMCPServerRuntime(
	ctx context.Context,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
) (settingspkg.MCPServerRuntimeStatus, error) {
	if key, tracked := settingsMCPDeadEntityKey(
		target,
		server.EffectiveRuntimeName(),
	); tracked &&
		s.deadEntities != nil {
		s.deadEntities.ForgetEntity(key)
		ctx = deadentity.WithProbeRetry(ctx, key)
	}
	return s.MCPServerRuntimeStatus(ctx, target, server)
}
