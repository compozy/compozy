package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

// errStarvationSpawnUnresolvable reports that no agent covers a starved run's required
// capabilities. The escalation adapter maps it to scheduler.ErrSpawnUnresolvable so Tier2 spawn
// is skipped without failing the cycle while Tier3/Tier4 escalation still proceed.
var errStarvationSpawnUnresolvable = errors.New("daemon: starvation spawn has no capable agent")

// starvationSpawner resolves a deterministic capability-matched agent for a starved run, reusing
// the same workspace + agent resolution stack the review router uses.
type starvationSpawner struct {
	workspaces reviewRouterWorkspaceResolver
	agents     reviewRouterAgentResolver
}

// resolveAgent returns the lexicographically-first agent whose capabilities cover required. ok is
// false (with nil error) when nothing covers the set, which the caller treats as unresolvable.
func (s starvationSpawner) resolveAgent(
	ctx context.Context,
	workspaceID string,
	required []string,
) (string, bool, error) {
	required = trimmedNonEmptyStrings(required)
	resolved, err := resolveWorkspaceForSpawn(ctx, s.workspaces, workspaceID)
	if err != nil {
		return "", false, err
	}
	for _, agent := range sortedResolvedAgents(resolved) {
		ok, err := agentCoversCapabilities(s.agents, resolved, agent.Name, required)
		if err != nil {
			if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
				continue
			}
			return "", false, err
		}
		if ok {
			return strings.TrimSpace(agent.Name), true, nil
		}
	}
	return "", false, nil
}

func resolveWorkspaceForSpawn(
	ctx context.Context,
	resolver reviewRouterWorkspaceResolver,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" || resolver == nil {
		return nil, nil
	}
	resolved, err := resolver.Resolve(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func agentCoversCapabilities(
	resolver reviewRouterAgentResolver,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	required []string,
) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}
	agentName = strings.TrimSpace(agentName)
	if agentName == "" {
		return false, nil
	}
	agent, err := resolveAgentDef(resolver, agentName, resolved)
	if err != nil {
		return false, err
	}
	available := make(map[string]struct{})
	if agent.Capabilities != nil {
		for _, capability := range agent.Capabilities.Capabilities {
			id := strings.TrimSpace(capability.ID)
			if id != "" {
				available[id] = struct{}{}
			}
		}
	}
	for _, capability := range required {
		if _, ok := available[strings.TrimSpace(capability)]; !ok {
			return false, nil
		}
	}
	return true, nil
}

func resolveAgentDef(
	resolver reviewRouterAgentResolver,
	agentName string,
	resolved *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	if resolver != nil {
		return resolver.ResolveAgent(agentName, resolved)
	}
	if resolved != nil {
		for _, agent := range resolved.Agents {
			if strings.TrimSpace(agent.Name) == strings.TrimSpace(agentName) {
				return agent, nil
			}
		}
	}
	return aghconfig.AgentDef{}, fmt.Errorf("%w: %s", workspacepkg.ErrAgentNotAvailable, agentName)
}

func trimmedNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
