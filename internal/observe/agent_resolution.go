package observe

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func defaultProviderAuthModeResolver(
	homePaths compozyconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
	agentResolver session.AgentResolver,
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
			workspaceResolver,
			agentResolver,
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
	workspaceResolver workspacepkg.RuntimeResolver,
	agentResolver session.AgentResolver,
	agentName string,
	provider string,
	model string,
	workspaceID string,
) (compozyconfig.ResolvedAgent, error) {
	if ctx == nil {
		return compozyconfig.ResolvedAgent{}, errors.New("observe: agent resolver context is required")
	}

	cfg, agentDef, err := loadObservedAgent(
		ctx,
		homePaths,
		workspaceResolver,
		agentResolver,
		agentName,
		workspaceID,
	)
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
	workspaceResolver workspacepkg.RuntimeResolver,
	agentResolver session.AgentResolver,
	agentName string,
	workspaceID string,
) (compozyconfig.Config, compozyconfig.AgentDef, error) {
	var resolvedWorkspace *workspacepkg.ResolvedWorkspace
	loadOptions := make([]compozyconfig.LoadOption, 0, 1)
	if strings.TrimSpace(workspaceID) != "" {
		if workspaceResolver == nil {
			return compozyconfig.Config{}, compozyconfig.AgentDef{}, errors.New(
				"observe: workspace resolver is required",
			)
		}
		workspace, err := workspaceResolver.Resolve(ctx, workspaceID)
		if err != nil {
			return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf(
				"resolve workspace %q: %w",
				workspaceID,
				err,
			)
		}
		resolvedWorkspace = &workspace
		loadOptions = append(loadOptions, compozyconfig.WithWorkspaceRoot(workspace.RootDir))
	}
	cfg, err := compozyconfig.LoadForHome(homePaths, loadOptions...)
	if err != nil {
		return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load config: %w", err)
	}
	if agentResolver != nil {
		agentDef, err := agentResolver.ResolveAgent(agentName, resolvedWorkspace)
		if err != nil {
			return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load agent %q: %w", agentName, err)
		}
		return cfg, agentDef, nil
	}
	if resolvedWorkspace == nil {
		agentDef, err := compozyconfig.LoadAgentDef(agentName, homePaths)
		if err != nil {
			return compozyconfig.Config{}, compozyconfig.AgentDef{}, fmt.Errorf("load agent %q: %w", agentName, err)
		}
		return cfg, agentDef, nil
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
