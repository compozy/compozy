package daemon

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/api/core"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (n *daemonNativeTools) workspaceAgents(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]core.AgentCatalogEntry, error) {
	if resolved == nil {
		return nil, errors.New("daemon: resolved workspace is required")
	}
	merged := make(map[string]core.AgentCatalogEntry, len(resolved.Agents))
	for _, agent := range resolved.Agents {
		name := strings.TrimSpace(agent.Name)
		if name != "" {
			merged[name] = core.AgentCatalogEntryFromDef(n.deps.HomePaths, agent, resolved.ID)
		}
	}
	if n.deps.AgentCatalog != nil {
		catalogAgents, err := n.deps.AgentCatalog.ListAgents(ctx)
		if err != nil {
			return nil, fmt.Errorf("daemon: list agent catalog entries: %w", err)
		}
		for _, entry := range catalogAgents {
			agent := entry.Def
			name := strings.TrimSpace(agent.Name)
			if name == "" {
				continue
			}
			if _, exists := merged[name]; !exists {
				merged[name] = entry
			}
		}
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	slices.Sort(names)
	agents := make([]core.AgentCatalogEntry, 0, len(names))
	for _, name := range names {
		agents = append(agents, merged[name])
	}
	return agents, nil
}
