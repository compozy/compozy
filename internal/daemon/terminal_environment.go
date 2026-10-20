package daemon

import (
	"context"
	"maps"
	"slices"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/terminal"
)

type terminalSessionEnvironment interface {
	SessionEnvironment(context.Context, session.PromptRunIdentity) (config.SessionEnv, error)
}

var _ terminalSessionEnvironment = (*session.Manager)(nil)

func terminalEnvironmentProvider(state *bootState) terminal.EnvironmentProvider {
	return func(ctx context.Context, workspaceID string, actor terminal.Actor) (terminal.LaunchEnvironment, error) {
		if actor.Kind == terminal.ActorKindSystem {
			return terminal.LaunchEnvironment{}, nil
		}
		if state.profiles == nil || state.providerVault == nil {
			return terminal.LaunchEnvironment{}, terminal.ErrServiceUnavailable
		}
		profileName, err := state.profiles.ProfileName(ctx, actor.ProfileID)
		if err != nil {
			return terminal.LaunchEnvironment{}, err
		}
		declarations, err := terminalEnvironmentDeclarations(ctx, state, workspaceID, profileName, actor)
		if err != nil {
			return terminal.LaunchEnvironment{}, err
		}
		values, err := providers.ResolveSessionEnv(ctx, declarations, profileName, state.providerVault)
		if err != nil {
			return terminal.LaunchEnvironment{}, err
		}
		result := terminal.LaunchEnvironment{Values: values, Unset: slices.Sorted(maps.Keys(declarations))}
		cleanups := make([]func(), 0, len(values))
		for _, value := range values {
			cleanups = append(cleanups, diagnostics.RegisterRequiredSecret(value))
		}
		result.Release = func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
		}
		for name := range declarations {
			if _, present := values[name]; !present {
				state.logger.Info("terminal.session_env.optional_missing", "name", name)
			}
		}
		return result, nil
	}
}

func terminalEnvironmentDeclarations(
	ctx context.Context, state *bootState, workspaceID, profileName string, actor terminal.Actor,
) (config.SessionEnv, error) {
	if actor.Kind == terminal.ActorKindAgent {
		owner, ok := state.sessions.(terminalSessionEnvironment)
		if !ok {
			return nil, terminal.ErrServiceUnavailable
		}
		return owner.SessionEnvironment(ctx, session.PromptRunIdentity{
			SessionID: actor.SessionID, WorkspaceID: workspaceID, ProfileID: actor.ProfileID,
			RunID: actor.RunID, Generation: actor.Generation,
		})
	}
	resolved, err := state.workspaceResolver.ResolveForProfile(ctx, workspaceID, profileName)
	if err != nil {
		return nil, err
	}
	for _, agent := range resolved.Agents {
		if agent.Name != resolved.Config.Defaults.Agent {
			continue
		}
		route, err := resolved.Config.ResolveAgent(agent)
		if err != nil {
			return nil, err
		}
		return route.SessionEnv, nil
	}
	provider, err := resolved.Config.ResolveProvider(resolved.Config.Defaults.Provider)
	if err != nil {
		return nil, err
	}
	return config.MergeSessionEnv(nil, provider.SessionEnv), nil
}
