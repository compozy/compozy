package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var _ core.AgentCatalog = (*resourceAgentCatalog)(nil)

func (c *resourceAgentCatalog) ListAgents(ctx context.Context) ([]core.AgentCatalogEntry, error) {
	if ctx == nil {
		return nil, errors.New("daemon: list agent catalog context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c == nil || c.catalog == nil {
		return nil, nil
	}
	return c.agentEntriesForWorkspace(nil), nil
}

func (c *resourceAgentCatalog) GetAgent(ctx context.Context, name string) (core.AgentCatalogEntry, error) {
	if ctx == nil {
		return core.AgentCatalogEntry{}, errors.New("daemon: get agent catalog context is required")
	}
	if err := ctx.Err(); err != nil {
		return core.AgentCatalogEntry{}, err
	}
	target := strings.TrimSpace(name)
	if target == "" {
		return core.AgentCatalogEntry{}, errors.New("agent name is required")
	}
	for _, entry := range c.agentEntriesForWorkspace(nil) {
		if strings.TrimSpace(entry.Def.Name) == target {
			entry.Def = cloneAgentDef(entry.Def)
			return entry, nil
		}
	}
	return core.AgentCatalogEntry{}, fmt.Errorf("%w: %s", os.ErrNotExist, target)
}

func (c *resourceAgentCatalog) agentEntriesForWorkspace(
	resolved *workspacepkg.ResolvedWorkspace,
) []core.AgentCatalogEntry {
	if c == nil || c.catalog == nil {
		return nil
	}
	records := c.catalog.Snapshot()
	slices.SortFunc(records, func(left, right resources.Record[aghconfig.AgentDef]) int {
		return strings.Compare(agentRecordSortKey(left), agentRecordSortKey(right))
	})
	merged := make(map[string]core.AgentCatalogEntry)
	for _, record := range records {
		if record.Scope.Kind.Normalize() != resources.ResourceScopeKindGlobal {
			continue
		}
		name := strings.TrimSpace(record.Spec.Name)
		if name != "" {
			merged[name] = core.AgentCatalogEntry{
				Def:    cloneAgentDef(record.Spec),
				Origin: contract.AgentOriginGlobal,
			}
		}
	}
	workspaceID := ""
	if resolved != nil {
		workspaceID = strings.TrimSpace(resolved.ID)
	}
	if workspaceID != "" {
		for _, record := range records {
			if record.Scope.Kind.Normalize() != resources.ResourceScopeKindWorkspace ||
				strings.TrimSpace(record.Scope.ID) != workspaceID {
				continue
			}
			name := strings.TrimSpace(record.Spec.Name)
			if name != "" {
				merged[name] = core.AgentCatalogEntry{
					Def:         cloneAgentDef(record.Spec),
					Origin:      contract.AgentOriginWorkspace,
					WorkspaceID: strings.TrimSpace(record.Scope.ID),
				}
			}
		}
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	slices.Sort(names)
	entries := make([]core.AgentCatalogEntry, 0, len(names))
	for _, name := range names {
		entry := merged[name]
		entry.Def = cloneAgentDef(entry.Def)
		entries = append(entries, entry)
	}
	return entries
}
