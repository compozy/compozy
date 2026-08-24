package settings

import (
	"context"

	"github.com/compozy/compozy/internal/config/lifecycle"
)

func (s *service) classifyHooksExtensionsRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, ScopeUser, "")
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
	cfg, _, err := s.loadConfig(ctx, scope, workspaceID, "")
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

func (s *service) classifyPersonaRequest(ctx context.Context, req SectionUpdateRequest) lifecycle.Lifecycle {
	loaded, err := s.loadScopedSectionUpdateForProfile(
		ctx,
		req.Section,
		req.Scope,
		req.WorkspaceID,
		req.ProfileName,
		ScopeUser,
		ScopeProfile,
		ScopeWorkspace,
	)
	if err != nil {
		return lifecycle.Live
	}
	changed := diffPersonaSettings(loaded.config.Defaults, *req.Persona)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
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

func (s *service) classifyGatewayRequest(ctx context.Context, req SectionUpdateRequest) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.RestartRequired
	}
	changed := diffGatewaySettings(cfg.Gateway, *req.Gateway)
	return lifecycleForChangedPaths(changed, lifecycle.RestartRequired)
}

func (s *service) classifyWindowManagerRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	loaded, err := s.loadScopedSectionUpdate(
		ctx, req.Section, req.Scope, req.WorkspaceID, ScopeUser, ScopeWorkspace,
	)
	if err != nil {
		return lifecycle.Live
	}
	desired, desiredAliases := mergeWindowManagerRequest(
		loaded.config.WindowManager,
		loaded.config.CmdPalette.Aliases,
		req,
	)
	changed := diffWindowManagerSettings(
		loaded.config.WindowManager,
		loaded.config.CmdPalette.Aliases,
		desired,
		desiredAliases,
	)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
}

func (s *service) classifyCmdPaletteRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	loaded, err := s.loadScopedSectionUpdateForProfile(
		ctx,
		req.Section,
		req.Scope,
		req.WorkspaceID,
		req.ProfileName,
		ScopeUser,
		ScopeProfile,
		ScopeWorkspace,
	)
	if err != nil {
		return lifecycle.Live
	}
	desired := desiredCmdPaletteSection(loaded.config.CmdPalette, *req.CmdPalette)
	changed := diffCmdPaletteSettings(loaded.config.CmdPalette, desired)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
}

func (s *service) classifyAttentionRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	cfg, _, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return lifecycle.Live
	}
	changed := diffAttentionRequest(cfg.Attention, *req.Attention)
	return lifecycleForChangedPaths(changed, lifecycle.Live)
}
