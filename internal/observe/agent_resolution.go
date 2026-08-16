package observe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func defaultPermissionModeResolver(
	homePaths compozyconfig.HomePaths,
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
	homePaths compozyconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
) ProviderAuthModeResolver {
	return func(
		ctx context.Context,
		agentName string,
		provider string,
		model string,
		workspaceID string,
	) (compozyconfig.ProviderAuthMode, error) {
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
	homePaths compozyconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
	agentName string,
	provider string,
	model string,
	workspaceID string,
) (compozyconfig.ResolvedAgent, error) {
	if ctx == nil {
		return compozyconfig.ResolvedAgent{}, errors.New("observe: agent resolver context is required")
	}

	cfg, agentDef, err := loadObservedAgent(ctx, homePaths, resolver, agentName, workspaceID)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	resolved, err := cfg.ResolveSessionAgentWithRuntime(agentDef, compozyconfig.RuntimeOverrides{
		Provider: provider,
		Model:    model,
	})
	if err != nil {
		return compozyconfig.ResolvedAgent{}, fmt.Errorf("resolve agent %q: %w", agentName, err)
	}
	return resolved, nil
}

func loadObservedAgent(
	ctx context.Context,
	homePaths compozyconfig.HomePaths,
	resolver workspacepkg.RuntimeResolver,
	agentName string,
	workspaceID string,
) (compozyconfig.Config, compozyconfig.AgentDef, error) {
	if strings.TrimSpace(workspaceID) == "" {
		cfg, err := compozyconfig.LoadForHome(homePaths)
		if err != nil {
			return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load config: %w", err)
		}
		agentDef, err := compozyconfig.LoadAgentDef(agentName, homePaths)
		if err != nil {
			return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load agent %q: %w", agentName, err)
		}
		return cfg, agentDef, nil
	}

	if resolver == nil {
		return compozyconfig.Config{}, compozyconfig.AgentDef{}, errors.New("observe: workspace resolver is required")
	}
	resolvedWorkspace, err := resolver.Resolve(ctx, workspaceID)
	if err != nil {
		return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf(
			"resolve workspace %q: %w",
			workspaceID,
			err,
		)
	}
	cfg, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithWorkspaceRoot(resolvedWorkspace.RootDir))
	if err != nil {
		return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load config: %w", err)
	}
	if agentDef, ok := compozyconfig.BuiltinAgentDef(agentName); ok {
		return cfg, agentDef, nil
	}
	agentDef, err := agentDefByName(resolvedWorkspace.Agents, agentName)
	if err != nil {
		return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load agent %q: %w", agentName, err)
	}
	return cfg, agentDef, nil
}

func agentDefByName(agents []compozyconfig.AgentDef, name string) (compozyconfig.AgentDef, error) {
	target := strings.TrimSpace(name)
	if target == "" {
		return compozyconfig.AgentDef{}, errors.New("agent name is required")
	}
	for _, agent := range agents {
		if strings.TrimSpace(agent.Name) == target {
			return agent, nil
		}
	}
	return compozyconfig.AgentDef{}, workspacepkg.ErrAgentNotAvailable
}
