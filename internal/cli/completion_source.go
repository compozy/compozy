package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/spf13/cobra"
)

const completionBudget = 400 * time.Millisecond

type completionRuntime struct {
	deps   commandDeps
	once   sync.Once
	client DaemonClient
	err    error
}

func newCompletionRuntime(deps commandDeps) *completionRuntime {
	return &completionRuntime{deps: deps}
}

func completionBaseContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func completionCommandDeps(ctx context.Context, deps commandDeps, cmd *cobra.Command) commandDeps {
	deps.windowManagerWarning = compozyconfig.WithWindowManagerWarning(func(string, bool) {})
	if deps.defaultLoadConfig {
		deps.loadConfig = func() (compozyconfig.Config, error) {
			homePaths, err := deps.resolveHome()
			if err != nil {
				return compozyconfig.Config{}, err
			}
			return compozyconfig.LoadForHome(homePaths, deps.windowManagerWarning)
		}
	}
	if deps.defaultNewClient {
		deps.newClient = nil
	}
	deps.commandContext = func() context.Context { return ctx }
	if cmd != nil && deps.computerTarget != nil {
		originalTarget := deps.computerTarget
		deps.computerTarget = func() (string, bool) {
			flag := cmd.Flag(computerFlagName)
			if flag == nil || !flag.Changed {
				return "", false
			}
			return originalTarget()
		}
	}
	return deps.withDefaults()
}

func (r *completionRuntime) ensureClient(ctx context.Context, cmd *cobra.Command) (DaemonClient, error) {
	if r == nil {
		return nil, fmt.Errorf("cli: completion runtime is nil")
	}
	r.once.Do(func() {
		deps := completionCommandDeps(ctx, r.deps, cmd)
		r.client, r.err = clientFromDeps(deps)
	})
	return r.client, r.err
}

func completionProfileContext(ctx context.Context, cmd *cobra.Command) context.Context {
	if cmd == nil {
		return ctx
	}
	if flag := cmd.Flag(allProfilesFlagName); flag != nil && flag.Changed {
		return context.WithValue(ctx, profileReadSelectionContextKey{}, profileReadSelection{AllProfiles: true})
	}
	if flag := cmd.Flag(profileFlagName); flag != nil {
		if name := strings.TrimSpace(flag.Value.String()); name != "" {
			return context.WithValue(ctx, profileReadSelectionContextKey{}, profileReadSelection{Profile: name})
		}
	}
	return ctx
}

func completionWorkspaceRef(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
	rt *completionRuntime,
) string {
	if cmd != nil {
		if flag := cmd.Flag(workspaceFlagName); flag != nil {
			if value := strings.TrimSpace(flag.Value.String()); value != "" {
				return value
			}
		}
	}
	if deps.getenv != nil {
		if value := strings.TrimSpace(deps.getenv(workspaceEnvName)); value != "" {
			return value
		}
	}
	if rt == nil || deps.getwd == nil {
		return ""
	}
	cwd, err := currentWorkingDirectory(deps)
	if err != nil {
		return ""
	}
	client, err := rt.ensureClient(ctx, cmd)
	if err != nil || client == nil {
		return ""
	}
	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		return ""
	}
	route, err := nearestCLIWorkspaceRoute(cwd, workspaces)
	if err != nil {
		return ""
	}
	return route
}

func (r *completionRuntime) complete(
	cmd *cobra.Command,
	kind completionKind,
	workspaceOverride string,
	providerOverride string,
) []completionCandidate {
	if kind == kindOutput {
		return r.outputFormats()
	}
	ctx, cancel := context.WithTimeout(completionBaseContext(cmd), completionBudget)
	defer cancel()
	ctx = completionProfileContext(ctx, cmd)

	if _, err := r.ensureClient(ctx, cmd); err != nil {
		return nil
	}

	workspaceRef := ""
	if kindNeedsWorkspace(kind) {
		workspaceRef = strings.TrimSpace(workspaceOverride)
		if workspaceRef == "" {
			workspaceRef = completionWorkspaceRef(ctx, cmd, r.deps, r)
		}
	}
	return r.candidates(ctx, kind, workspaceRef, providerOverride)
}

func (r *completionRuntime) candidates(
	ctx context.Context,
	kind completionKind,
	workspaceRef string,
	providerOverride string,
) []completionCandidate {
	switch kind {
	case kindWorkspace:
		return r.workspaces(ctx)
	case kindProfile:
		return r.profiles(ctx)
	case kindAgent:
		return r.agents(ctx, workspaceRef)
	case kindProvider:
		return r.providers(ctx)
	case kindModel:
		return r.models(ctx, providerOverride)
	case kindSession:
		return r.sessions(ctx, workspaceRef)
	case kindTask:
		return r.tasks(ctx, workspaceRef)
	case kindLoop:
		return r.loops(ctx, workspaceRef)
	case kindAutomation:
		return r.automations(ctx, workspaceRef)
	case kindWorktree:
		return r.worktrees(ctx, workspaceRef)
	case kindSessionFolder:
		return r.sessionFolders(ctx, workspaceRef)
	case kindWorkspaceFolder:
		return r.workspaceFolders(ctx)
	case kindExtension:
		return r.extensions(ctx)
	case kindComputer:
		return r.computers(ctx)
	case kindVault:
		return r.vaultSecrets(ctx)
	case kindToolset:
		return r.toolsets(ctx, workspaceRef)
	default:
		return nil
	}
}

func kindNeedsWorkspace(kind completionKind) bool {
	switch kind {
	case kindAgent, kindSession, kindTask, kindLoop, kindAutomation, kindWorktree, kindSessionFolder, kindToolset:
		return true
	default:
		return false
	}
}

func (r *completionRuntime) outputFormats() []completionCandidate {
	return []completionCandidate{
		{value: string(OutputHuman), description: "Human-readable output"},
		{value: string(OutputJSON), description: "JSON output"},
		{value: string(OutputJSONL), description: "JSON Lines output"},
		{value: string(OutputToon), description: "TOON output"},
	}
}
