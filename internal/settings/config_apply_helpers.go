package settings

import (
	"context"
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
		if req.WindowManager == nil {
			return lifecycle.Live
		}
		return s.classifyWindowManagerRequest(ctx, req)
	case SectionAttention:
		if req.Attention == nil {
			return lifecycle.Live
		}
		return s.classifyAttentionRequest(ctx, req)
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
	case CollectionMCPServers:
		for _, item := range envelope.MCPServers {
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
	cloned.Attention = cloneAttentionConfig(cfg.Attention)
	return cloned
}

func mapsClone[K comparable, V any](source map[K]V) map[K]V {
	return maps.Clone(source)
}
