package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func resolveStoredSessionWorkspace(
	ctx context.Context,
	meta store.SessionMeta,
	resolver workspacepkg.RuntimeResolver,
) (workspacepkg.ResolvedWorkspace, error) {
	if resolver == nil {
		return workspacepkg.ResolvedWorkspace{}, errors.New("session: workspace resolver is required")
	}

	workspaceID := strings.TrimSpace(meta.WorkspaceID)
	if workspaceID == "" {
		return workspacepkg.ResolvedWorkspace{}, errors.New("session: session workspace id is required")
	}

	resolved, err := resolver.Resolve(ctx, workspaceID)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
			"session: resolve workspace %q for session %q: %w",
			workspaceID,
			meta.ID,
			err,
		)
	}
	return resolved, nil
}

func (m *Manager) resolveCreateWorkspace(ctx context.Context, opts CreateOpts) (workspacepkg.ResolvedWorkspace, error) {
	resolver, err := m.requireWorkspaceResolver()
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}

	workspaceRef := strings.TrimSpace(opts.Workspace)
	workspacePath := strings.TrimSpace(opts.WorkspacePath)
	switch {
	case workspaceRef == "" && workspacePath == "":
		return workspacepkg.ResolvedWorkspace{}, errors.New("session: workspace or workspace path is required")
	case workspaceRef != "" && workspacePath != "":
		return workspacepkg.ResolvedWorkspace{}, errors.New(
			"session: workspace and workspace path are mutually exclusive",
		)
	case workspacePath != "":
		resolved, err := resolver.ResolveOrRegister(ctx, workspacePath)
		if err != nil {
			return workspacepkg.ResolvedWorkspace{}, fmt.Errorf(
				"session: resolve workspace path %q: %w",
				workspacePath,
				err,
			)
		}
		return resolved, nil
	default:
		resolved, err := resolver.Resolve(ctx, workspaceRef)
		if err != nil {
			return workspacepkg.ResolvedWorkspace{}, fmt.Errorf("session: resolve workspace %q: %w", workspaceRef, err)
		}
		return resolved, nil
	}
}

func applyCreateSandboxOverride(
	resolved *workspacepkg.ResolvedWorkspace,
	opts CreateOpts,
) (bool, error) {
	if resolved == nil {
		return false, errors.New("session: resolved workspace is required")
	}
	sandboxRef := strings.TrimSpace(opts.SandboxRef)
	if opts.DisableSandbox {
		if sandboxRef != "" {
			return false, errors.New(
				"session: sandbox ref and disabled sandbox are mutually exclusive",
			)
		}
		return true, nil
	}
	if sandboxRef == "" {
		return false, nil
	}

	sandbox, err := resolved.Config.ResolveSandbox(sandboxRef)
	if err != nil {
		return false, fmt.Errorf(
			"session: resolve sandbox ref %q: %w",
			sandboxRef,
			err,
		)
	}
	resolved.Sandbox = sandbox
	resolved.SandboxRef = sandboxRef
	return false, nil
}

func (m *Manager) resolveResumeWorkspace(
	ctx context.Context,
	meta store.SessionMeta,
) (workspacepkg.ResolvedWorkspace, error) {
	resolver, err := m.requireWorkspaceResolver()
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}

	return resolveStoredSessionWorkspace(ctx, meta, resolver)
}

func (m *Manager) requireWorkspaceResolver() (workspacepkg.RuntimeResolver, error) {
	if m.workspace == nil {
		return nil, errors.New("session: workspace resolver is required")
	}
	return m.workspace, nil
}

func resolveWorkspaceAgent(
	agentName string,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (compozyconfig.AgentDef, error) {
	target := strings.TrimSpace(agentName)
	if target == "" {
		return compozyconfig.AgentDef{}, errors.New("session: agent name is required")
	}
	if resolvedWorkspace == nil {
		return compozyconfig.AgentDef{}, errors.New("session: resolved workspace is required")
	}

	for _, agent := range resolvedWorkspace.Agents {
		if strings.TrimSpace(agent.Name) != target {
			continue
		}
		return agent, nil
	}

	return compozyconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, target)
}

func (m *Manager) resolveWorkspaceAgentArtifacts(
	agentName string,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (AgentArtifacts, error) {
	if m != nil && m.agentResolver != nil {
		if resolver, ok := m.agentResolver.(AgentArtifactResolver); ok {
			return resolver.ResolveAgentArtifacts(agentName, resolvedWorkspace)
		}
		agent, err := m.agentResolver.ResolveAgent(agentName, resolvedWorkspace)
		if err != nil {
			return AgentArtifacts{}, err
		}
		return AgentArtifacts{Agent: agent}, nil
	}
	agent, err := resolveWorkspaceAgent(agentName, resolvedWorkspace)
	if err != nil {
		return AgentArtifacts{}, err
	}
	return AgentArtifacts{Agent: agent}, nil
}

func (m *Manager) resolveWorkspaceAgentArtifactsForSession(
	agentName string,
	sessionType Type,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
) (AgentArtifacts, error) {
	artifacts, err := m.resolveWorkspaceAgentArtifacts(agentName, resolvedWorkspace)
	if err == nil {
		return artifacts, nil
	}
	if !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return AgentArtifacts{}, err
	}

	fallback, ok := builtinSessionAgentDef(agentName, sessionType)
	if !ok {
		return AgentArtifacts{}, err
	}
	return AgentArtifacts{Agent: fallback}, nil
}

func resolveWorkspaceSessionAgentForType(
	agentName string,
	provider string,
	sessionType Type,
	resolvedWorkspace *workspacepkg.ResolvedWorkspace,
	agentResolver AgentResolver,
) (compozyconfig.ResolvedAgent, error) {
	if resolvedWorkspace == nil {
		return compozyconfig.ResolvedAgent{}, errors.New("session: resolved workspace is required")
	}

	var (
		agentDef compozyconfig.AgentDef
		err      error
	)
	if agentResolver != nil {
		agentDef, err = agentResolver.ResolveAgent(agentName, resolvedWorkspace)
	} else {
		agentDef, err = resolveWorkspaceAgent(agentName, resolvedWorkspace)
	}
	if err != nil {
		if !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return compozyconfig.ResolvedAgent{}, err
		}
		fallback, ok := builtinSessionAgentDef(agentName, sessionType)
		if !ok {
			return compozyconfig.ResolvedAgent{}, err
		}
		agentDef = fallback
	}

	resolved, err := resolvedWorkspace.Config.ResolveSessionAgent(agentDef, provider)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	return resolved, nil
}

func builtinSessionAgentDef(agentName string, sessionType Type) (compozyconfig.AgentDef, bool) {
	expectedName := ""
	switch normalizeSessionType(sessionType) {
	case SessionTypeCoordinator:
		expectedName = compozyconfig.BuiltinCoordinatorAgentName
	case SessionTypeDream:
		expectedName = compozyconfig.BuiltinDreamingCuratorAgentName
	default:
		return compozyconfig.AgentDef{}, false
	}
	def, ok := compozyconfig.BuiltinAgentDef(agentName)
	if !ok || def.Name != expectedName {
		return compozyconfig.AgentDef{}, false
	}
	return def, true
}
