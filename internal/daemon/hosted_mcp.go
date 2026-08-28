package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type hostedMCPLauncherAdapter struct {
	service *mcppkg.HostedService
}

func (d *Daemon) buildHostedMCPService(state *bootState) (*mcppkg.HostedService, error) {
	if state == nil {
		return nil, errors.New("daemon: hosted MCP state is required")
	}
	if !state.cfg.Tools.Enabled || !state.cfg.Tools.HostedMCPEnabled {
		logger := slog.Default()
		if state.logger != nil {
			logger = state.logger
		}
		reason := "tools_disabled"
		if state.cfg.Tools.Enabled {
			reason = "hosted_mcp_disabled"
		}
		logger.Info(
			"daemon.hosted_mcp.disabled",
			"reason",
			reason,
			"tools_enabled",
			state.cfg.Tools.Enabled,
			"hosted_mcp_enabled",
			state.cfg.Tools.HostedMCPEnabled,
		)
		return nil, nil
	}
	executable, err := d.executable()
	if err != nil {
		return nil, fmt.Errorf("daemon: resolve hosted MCP executable: %w", err)
	}
	return mcppkg.NewHostedService(&mcppkg.HostedConfig{
		Enabled:        true,
		BindNonceTTL:   state.cfg.Tools.HostedMCP.BindNonceTTL(),
		ExpectedBinary: executable,
		HomePaths:      d.homePaths,
		Logger:         state.logger,
		Now:            d.now,
		Registry: func() toolspkg.Registry {
			if state == nil {
				return nil
			}
			return state.toolRegistry
		},
		ProjectionGeneration: func(ctx context.Context, scope toolspkg.Scope) (string, bool) {
			registry := state.toolRegistry
			generation, ok := registry.(toolspkg.ProjectionGenerationProvider)
			if !ok {
				return "", false
			}
			return generation.ProjectionGeneration(ctx, scope)
		},
	})
}

func (l hostedMCPLauncherAdapter) Launch(
	ctx context.Context,
	req session.HostedMCPLaunchRequest,
) (compozyconfig.MCPServer, error) {
	if l.service == nil {
		return compozyconfig.MCPServer{}, mcppkg.ErrHostedDisabled
	}
	return l.service.Launch(ctx, mcppkg.HostedLaunchRequest{
		SessionID:   req.SessionID,
		ProfileID:   req.ProfileID,
		WorkspaceID: req.WorkspaceID,
		AgentName:   req.AgentName,
	})
}

func (l hostedMCPLauncherAdapter) ArmLaunch(ctx context.Context, sessionID string) error {
	if l.service == nil {
		return mcppkg.ErrHostedDisabled
	}
	return l.service.ArmLaunch(ctx, sessionID)
}

func (l hostedMCPLauncherAdapter) BindRun(
	ctx context.Context,
	sessionID, runID string,
	generation int64,
) error {
	if l.service == nil {
		return mcppkg.ErrHostedDisabled
	}
	return l.service.BindRun(ctx, sessionID, runID, generation)
}

func (l hostedMCPLauncherAdapter) ReleaseRun(sessionID, runID string, generation int64) {
	if l.service != nil {
		l.service.ReleaseRun(sessionID, runID, generation)
	}
}

func (l hostedMCPLauncherAdapter) CancelLaunch(sessionID string) {
	if l.service != nil {
		l.service.CancelLaunch(sessionID)
	}
}

func (l hostedMCPLauncherAdapter) ReleaseSession(sessionID string) {
	if l.service != nil {
		l.service.ReleaseSession(sessionID)
	}
}

func hostedMCPLauncher(service *mcppkg.HostedService) session.HostedMCPLauncher {
	if service == nil {
		return nil
	}
	return hostedMCPLauncherAdapter{service: service}
}
