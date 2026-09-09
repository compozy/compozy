package session

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// resolveSpawnProviderCommand preserves native account routing within one authority scope.
func (m *Manager) resolveSpawnProviderCommand(
	ctx context.Context,
	spec *sessionStartSpec,
	agent compozyconfig.AgentDef,
	resolved compozyconfig.ResolvedAgent,
	visited map[string]bool,
) (compozyconfig.ResolvedAgent, error) {
	if normalizeSessionType(spec.sessionType) != SessionTypeSpawned || strings.TrimSpace(agent.Command) != "" ||
		spec.lineage == nil ||
		spec.lineage.ParentSessionID == "" ||
		resolved.AuthMode != compozyconfig.ProviderAuthModeNativeCLI ||
		resolved.HomePolicy != compozyconfig.ProviderHomePolicyOperator {
		return resolved, nil
	}
	if current, ok := m.Get(spec.sessionID); ok {
		route := current.providerRoutingSnapshot()
		if compatibleSpawnProviderRoute(route, resolved) {
			resolved.Command = route.Command
			return resolved, nil
		}
	}
	parentID := spec.lineage.ParentSessionID
	if visited[parentID] {
		return compozyconfig.ResolvedAgent{}, fmt.Errorf(
			"session: invalid provider routing lineage for %q",
			spec.sessionID,
		)
	}
	visited[parentID] = true
	parentRoute, err := m.creatorProviderRoute(ctx, spec, resolved.Provider, parentID, visited)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	if compatibleSpawnProviderRoute(parentRoute, resolved) {
		resolved.Command = parentRoute.Command
	}
	return resolved, nil
}

func (m *Manager) creatorProviderRoute(
	ctx context.Context,
	spec *sessionStartSpec,
	provider string,
	parentID string,
	visited map[string]bool,
) (compozyconfig.ResolvedAgent, error) {
	if parent, ok := m.Get(parentID); ok {
		info := parent.Info()
		if info.Provider != provider || info.WorkspaceID != spec.workspace.ID || info.ProfileID != spec.profileID {
			return compozyconfig.ResolvedAgent{}, nil
		}
		return parent.providerRoutingSnapshot(), nil
	}
	meta, err := m.readMetaWithContext(ctx, parentID)
	if errors.Is(err, ErrSessionNotFound) {
		spec.startLogger(m).Info("session.provider_route.creator_unavailable", "creator_session_id", parentID)
		return compozyconfig.ResolvedAgent{}, nil
	}
	if err != nil {
		return compozyconfig.ResolvedAgent{}, fmt.Errorf(
			"session: resolve creator provider route for %q: %w",
			spec.sessionID,
			err,
		)
	}
	if meta.Provider != provider || meta.WorkspaceID != spec.workspace.ID || meta.ProfileID != spec.profileID {
		return compozyconfig.ResolvedAgent{}, nil
	}
	workspace, err := m.resolveResumeWorkspace(ctx, meta)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	parentSpec, err := sessionStartSpecFromMeta(meta, &workspace, workspace.RootDir)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	artifacts, err := m.resolveWorkspaceAgentArtifactsForSession(
		parentSpec.agentName,
		parentSpec.sessionType,
		&workspace,
	)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	parentRoute, err := workspace.Config.ResolveSessionAgent(artifacts.Agent, meta.Provider)
	if err != nil {
		return compozyconfig.ResolvedAgent{}, err
	}
	return m.resolveSpawnProviderCommand(ctx, &parentSpec, artifacts.Agent, parentRoute, visited)
}

func compatibleSpawnProviderRoute(parent, child compozyconfig.ResolvedAgent) bool {
	return strings.TrimSpace(parent.Command) != "" && parent.Provider == child.Provider &&
		parent.AuthMode == child.AuthMode && parent.HomePolicy == child.HomePolicy &&
		parent.EnvPolicy == child.EnvPolicy && parent.Harness == child.Harness &&
		parent.RuntimeProvider == child.RuntimeProvider && parent.Transport == child.Transport &&
		parent.BaseURL == child.BaseURL && parent.AuthStatusCmd == child.AuthStatusCmd &&
		parent.AuthLoginCmd == child.AuthLoginCmd
}

func (s *Session) providerRoutingSnapshot() compozyconfig.ResolvedAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerRoute
}

func (s *Session) setProviderRouting(resolved compozyconfig.ResolvedAgent) {
	s.mu.Lock()
	s.providerRoute = resolved
	s.mu.Unlock()
}

func providerCommandFingerprint(command string) string {
	if strings.TrimSpace(command) == "" {
		return ""
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(command)))
}
