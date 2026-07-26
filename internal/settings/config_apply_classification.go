package settings

import (
	"context"

	"github.com/compozy/agh/internal/config/lifecycle"
)

func (s *service) classifyHooksExtensionsRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.RestartRequired
	}
	changed := diffExtensionsSettings(cfg.Extensions, *req.HooksExtensions)
	return lifecycleForChangedPaths(changed, lifecycle.RestartRequired)
}

func (s *service) classifySkillsRequest(ctx context.Context, req SectionUpdateRequest) lifecycle.Lifecycle {
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.Live
	}
	if scope == ScopeWorkspace {
		return lifecycle.Live
	}
	if scope == ScopeAgent {
		return lifecycle.Live
	}
	cfg, _, err := s.loadConfig(ctx, scope, workspaceID)
	if err != nil {
		return lifecycle.Live
	}
	changed := diffSkillsSettings(cfg.Skills, *req.Skills)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
}

func (s *service) classifyGeneralRequest(ctx context.Context, req SectionUpdateRequest) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.RestartRequired
	}
	changed := diffGeneralSettings(&cfg, *req.General)
	return lifecycleForChangedPaths(changed, lifecycle.RestartRequired)
}

func (s *service) classifyRolesRequest(ctx context.Context, req SectionUpdateRequest) lifecycle.Lifecycle {
	loaded, err := s.loadRolesSectionUpdate(ctx, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.Live
	}
	changed := diffRolesSettings(&loaded.config.Roles, req.Roles)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
}

func (s *service) classifyNetworkRequest(ctx context.Context, req SectionUpdateRequest) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.RestartRequired
	}
	changed := diffNetworkSettings(cfg.Network, *req.Network)
	return lifecycleForChangedPaths(changed, lifecycle.RestartRequired)
}

func (s *service) classifyWindowManagerRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.Live
	}
	changed := diffWindowManagerSettings(cfg.WindowManager, *req.WindowManager)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
}
