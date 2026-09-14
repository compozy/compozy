package settings

import (
	"context"
	"errors"
	"maps"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

func (s *service) classifySectionApplyRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	switch req.Section {
	case SectionSkills:
		if req.Skills == nil {
			return lifecycle.Live
		}
		return s.classifySkillsRequest(ctx, req)
	case SectionGeneral:
		if req.General == nil {
			return lifecycle.RestartRequired
		}
		return s.classifyGeneralRequest(ctx, req)
	case SectionPersona:
		if req.Persona == nil {
			return lifecycle.Live
		}
		return s.classifyPersonaRequest(ctx, req)
	case SectionRoles:
		if req.Roles == nil {
			return lifecycle.Live
		}
		return s.classifyRolesRequest(ctx, req)
	case SectionHooksExtensions:
		if req.HooksExtensions == nil {
			return lifecycle.RestartRequired
		}
		return s.classifyHooksExtensionsRequest(ctx, req)
	default:
		return s.classifyRuntimeSectionApplyRequest(ctx, req)
	}
}

func (s *service) classifyRuntimeSectionApplyRequest(
	ctx context.Context,
	req SectionUpdateRequest,
) lifecycle.Lifecycle {
	switch req.Section {
	case SectionNetwork:
		if req.Network == nil {
			return lifecycle.RestartRequired
		}
		return s.classifyNetworkRequest(ctx, req)
	case SectionGateway:
		if req.Gateway == nil {
			return lifecycle.RestartRequired
		}
		return s.classifyGatewayRequest(ctx, req)
	case SectionWindowManager:
		if !hasWindowManagerMutation(req) {
			return lifecycle.Live
		}
		return s.classifyWindowManagerRequest(ctx, req)
	case SectionCmdPalette:
		if req.CmdPalette == nil {
			return lifecycle.Live
		}
		return s.classifyCmdPaletteRequest(ctx, req)
	case SectionAttention:
		if req.Attention == nil {
			return lifecycle.Live
		}
		return s.classifyAttentionRequest(ctx, req)
	case SectionMarketplace:
		return lifecycle.Live
	case SectionShell:
		return lifecycleForChangedPaths([]string{"shell.sessions.sort", "shell.sessions.scope"}, lifecycle.Live)
	default:
		return lifecycle.RestartRequired
	}
}

func lifecycleForChangedPaths(paths []string, fallback lifecycle.Lifecycle) lifecycle.Lifecycle {
	if len(paths) == 0 {
		return lifecycle.Live
	}
	configLifecycle, _, err := lifecycle.ClassifyPaths(paths)
	if err != nil {
		return fallback
	}
	return configLifecycle
}

func (s *service) collectionItemExistsBeforeMutation(
	ctx context.Context,
	req CollectionRequest,
	name string,
) (bool, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return false, nil
	}
	if req.Collection == CollectionMCPServers {
		scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
		if err != nil {
			return false, err
		}
		profileName, err := normalizeSettingsProfileName(scope, req.ProfileName)
		if err != nil {
			return false, err
		}
		_, _, err = s.resolveMCPAuthTarget(ctx, MCPAuthTargetRequest{
			Scope: scope, WorkspaceID: workspaceID, ProfileName: profileName,
			Name: trimmedName, Owner: req.Owner,
		})
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return err == nil, err
	}
	envelope, err := s.ListCollection(ctx, req)
	if err != nil {
		return false, err
	}
	switch req.Collection {
	case CollectionProviders:
		for i := range envelope.Providers {
			item := &envelope.Providers[i]
			if item.Name == trimmedName {
				return true, nil
			}
		}
	case CollectionSandboxes:
		for _, item := range envelope.Sandboxes {
			if item.Name == trimmedName {
				return true, nil
			}
		}
	case CollectionHooks:
		for i := range envelope.Hooks {
			item := &envelope.Hooks[i]
			if item.Name == trimmedName {
				return true, nil
			}
		}
	}
	return false, nil
}

func automationSettingsFromConfig(cfg *compozyconfig.Config) AutomationSettings {
	return AutomationSettings{
		Enabled:           cfg.Automation.Enabled,
		Timezone:          cfg.Automation.Timezone,
		MaxConcurrentJobs: cfg.Automation.MaxConcurrentJobs,
		DefaultFireLimit:  cfg.Automation.DefaultFireLimit,
	}
}

func cloneActiveConfig(cfg *compozyconfig.Config) compozyconfig.Config {
	cloned := *cfg
	cloned.Providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
	cloned.Sandboxes = mapsClone(cfg.Sandboxes)
	cloned.MCPServers = append([]compozyconfig.MCPServer(nil), cfg.MCPServers...)
	cloned.Hooks.Declarations = append([]hookspkg.HookDecl(nil), cfg.Hooks.Declarations...)
	cloned.Roles = compozyconfig.CloneRolesConfig(&cfg.Roles)
	cloned.RoleSources = compozyconfig.CloneRoleFieldSources(cfg.RoleSources)
	cloned.WindowManager = cloneWindowManagerConfig(cfg.WindowManager)
	cloned.CmdPalette = compozyconfig.CloneCmdPaletteConfig(cfg.CmdPalette)
	cloned.Attention = cloneAttentionConfig(cfg.Attention)
	return cloned
}

func mapsClone[K comparable, V any](source map[K]V) map[K]V {
	return maps.Clone(source)
}
