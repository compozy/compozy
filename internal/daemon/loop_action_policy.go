package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type loopSessionAgentResolver interface {
	ResolveAgent(name string, resolved *workspacepkg.ResolvedWorkspace) (aghconfig.AgentDef, error)
}

type loopSessionPolicyGate struct {
	workspaceResolver workspacepkg.RuntimeResolver
	agentResolver     loopSessionAgentResolver
}

type loopSessionPolicyResolution struct {
	workspace workspacepkg.ResolvedWorkspace
	agent     aghconfig.ResolvedAgent
}

func (g *loopSessionPolicyGate) apply(
	ctx context.Context,
	opts *session.CreateOpts,
	agentName string,
	allowedTools []string,
) error {
	_, err := g.applyResolved(ctx, opts, agentName, allowedTools)
	return err
}

func (g *loopSessionPolicyGate) applyResolved(
	ctx context.Context,
	opts *session.CreateOpts,
	agentName string,
	allowedTools []string,
) (loopSessionPolicyResolution, error) {
	if opts == nil {
		return loopSessionPolicyResolution{}, errors.New("daemon: loop session options are required")
	}
	resolved, err := g.resolveWorkspace(ctx, *opts)
	if err != nil {
		return loopSessionPolicyResolution{}, err
	}
	agentDef, err := g.resolveAgent(agentName, &resolved)
	if err != nil {
		return loopSessionPolicyResolution{}, fmt.Errorf(
			"%w: resolve loop session agent policy for %q: %w",
			looppkg.ErrValidation,
			strings.TrimSpace(agentName),
			err,
		)
	}
	resolvedAgent, err := resolved.Config.ResolveSessionAgentWithRuntime(agentDef, opts.Provider, opts.Model)
	if err != nil {
		return loopSessionPolicyResolution{}, fmt.Errorf(
			"%w: resolve loop session policy for %q: %w",
			looppkg.ErrValidation,
			strings.TrimSpace(agentName),
			err,
		)
	}
	policy := sessionPolicyFromResolvedAgentWorkspace(resolvedAgent, &resolved)
	applySessionSandboxPolicy(opts, policy)
	applySessionPermissionPolicy(opts, policy)
	if err := applyAllowedToolsNarrowing(opts, allowedTools); err != nil {
		return loopSessionPolicyResolution{}, err
	}
	cwd, err := session.ResolveSessionCWD(resolved.RootDir, opts.CWD)
	if err != nil {
		return loopSessionPolicyResolution{}, err
	}
	opts.CWD = cwd
	opts.Provider = strings.TrimSpace(resolvedAgent.Provider)
	opts.Model = strings.TrimSpace(resolvedAgent.Model)
	if runtimeMode := normalizeSessionRuntimeMode(policy.Runtime.Mode); runtimeMode != "" {
		opts.RuntimeMode = string(runtimeMode)
	}
	if permissions := strings.TrimSpace(string(opts.Permissions)); permissions != "" {
		resolvedAgent.Permissions = permissions
	}
	if len(opts.AllowedToolsOverride) > 0 {
		resolvedAgent.Tools = append([]string(nil), opts.AllowedToolsOverride...)
		resolvedAgent.Toolsets = nil
	}
	return loopSessionPolicyResolution{workspace: resolved, agent: resolvedAgent}, nil
}

func (g *loopSessionPolicyGate) resolveWorkspace(
	ctx context.Context,
	opts session.CreateOpts,
) (workspacepkg.ResolvedWorkspace, error) {
	if g == nil || g.workspaceResolver == nil {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceResolverUnavailable
	}
	if workspaceID := strings.TrimSpace(opts.Workspace); workspaceID != "" {
		resolved, err := g.workspaceResolver.Resolve(ctx, workspaceID)
		if err != nil {
			return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
				"daemon: resolve loop session workspace %q: %w",
				workspaceID,
				err,
			)
		}
		return resolved, nil
	}
	workspacePath := strings.TrimSpace(opts.WorkspacePath)
	if workspacePath == "" {
		return workspacepkg.ResolvedWorkspace{}, errors.New("daemon: loop session workspace is required")
	}
	resolved, err := g.workspaceResolver.ResolveOrRegister(ctx, workspacePath)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
			"daemon: resolve loop session workspace path %q: %w",
			workspacePath,
			err,
		)
	}
	return resolved, nil
}

func (g *loopSessionPolicyGate) resolveAgent(
	agentName string,
	resolved *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	target := strings.TrimSpace(agentName)
	if target == "" {
		return aghconfig.AgentDef{}, errors.New("daemon: loop session agent name is required")
	}
	if g != nil && g.agentResolver != nil {
		return g.agentResolver.ResolveAgent(target, resolved)
	}
	for _, agent := range resolved.Agents {
		if strings.TrimSpace(agent.Name) == target {
			return agent, nil
		}
	}
	return aghconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, target)
}

func sessionPolicyFromResolvedAgentWorkspace(
	agent aghconfig.ResolvedAgent,
	resolved *workspacepkg.ResolvedWorkspace,
) SessionPolicy {
	policy := SessionPolicy{
		Runtime: SessionRuntimePolicy{
			Permissions: aghconfig.PermissionMode(strings.TrimSpace(agent.Permissions)),
		},
	}
	if sandboxRef := resolvedWorkspaceSandboxRef(resolved); sandboxRef != "" {
		policy.Sandbox = SessionSandboxPolicy{
			Mode:       SessionSandboxModeRef,
			SandboxRef: sandboxRef,
		}
	}
	return policy
}

func resolvedWorkspaceSandboxRef(resolved *workspacepkg.ResolvedWorkspace) string {
	if resolved == nil {
		return ""
	}
	if sandboxRef := strings.TrimSpace(resolved.SandboxRef); sandboxRef != "" {
		return sandboxRef
	}
	return strings.TrimSpace(resolved.Config.Defaults.Sandbox)
}
