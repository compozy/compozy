package observe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func defaultPermissionModeResolver(
	homePaths aghconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
) PermissionModeResolver {
	return func(ctx context.Context, agentName, workspaceID string) (string, error) {
		resolved, err := resolveObservedAgent(ctx, homePaths, resolver, agentName, "", "", workspaceID)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(resolved.Permissions), nil
	}
}

func defaultProviderAuthModeResolver(
	homePaths aghconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
) ProviderAuthModeResolver {
	return func(
		ctx context.Context,
		agentName string,
		provider string,
		model string,
		workspaceID string,
	) (aghconfig.ProviderAuthMode, error) {
		resolved, err := resolveObservedAgent(
			ctx,
			homePaths,
			resolver,
			agentName,
			provider,
			model,
			workspaceID,
		)
		if err != nil {
			return "", err
		}
		return resolved.AuthMode, nil
	}
}

func resolveObservedAgent(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
	agentName string,
	provider string,
	model string,
	workspaceID string,
) (aghconfig.ResolvedAgent, error) {
	if ctx == nil {
		return aghconfig.ResolvedAgent{}, errors.New("observe: agent resolver context is required")
	}

	cfg, agentDef, err := loadObservedAgent(ctx, homePaths, resolver, agentName, workspaceID)
	if err != nil {
		return aghconfig.ResolvedAgent{}, err
	}
	resolved, err := cfg.ResolveSessionAgentWithRuntime(agentDef, provider, model)
	if err != nil {
		return aghconfig.ResolvedAgent{}, fmt.Errorf("resolve agent %q: %w", agentName, err)
	}
	return resolved, nil
}

func loadObservedAgent(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
	agentName string,
	workspaceID string,
) (aghconfig.Config, aghconfig.AgentDef, error) {
	if strings.TrimSpace(workspaceID) == "" {
		cfg, err := aghconfig.LoadForHome(homePaths)
		if err != nil {
			return aghconfig.Config{}, aghconfig.AgentDef{}, fmt.Errorf("load config: %w", err)
		}
		agentDef, err := aghconfig.LoadAgentDef(agentName, homePaths)
		if err != nil {
			return aghconfig.Config{}, aghconfig.AgentDef{}, fmt.Errorf("load agent %q: %w", agentName, err)
		}
		return cfg, agentDef, nil
	}

	if resolver == nil {
		return aghconfig.Config{}, aghconfig.AgentDef{}, errors.New("observe: workspace resolver is required")
	}
	resolvedWorkspace, err := resolver.Resolve(ctx, workspaceID)
	if err != nil {
		return aghconfig.Config{}, aghconfig.AgentDef{}, fmt.Errorf("resolve workspace %q: %w", workspaceID, err)
	}
	cfg, err := aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(resolvedWorkspace.RootDir))
	if err != nil {
		return aghconfig.Config{}, aghconfig.AgentDef{}, fmt.Errorf("load config: %w", err)
	}
	agentDef, err := agentDefByName(resolvedWorkspace.Agents, agentName)
	if err != nil {
		return aghconfig.Config{}, aghconfig.AgentDef{}, fmt.Errorf("load agent %q: %w", agentName, err)
	}
	return cfg, agentDef, nil
}

func agentDefByName(agents []aghconfig.AgentDef, name string) (aghconfig.AgentDef, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return aghconfig.AgentDef{}, errors.New("agent name is required")
	}
	for _, agent := range agents {
		if strings.TrimSpace(agent.Name) == target {
			return agent, nil
		}
	}
	return aghconfig.AgentDef{}, workspacepkg.ErrAgentNotAvailable
}
