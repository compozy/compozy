package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
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

func (c *resourceAgentCatalog) ListAgentsForWorkspace(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]core.AgentCatalogEntry, error) {
	if ctx == nil {
		return nil, errors.New("daemon: list workspace agent catalog context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, errors.New("daemon: resolved workspace is required to list agent catalog")
	}
	if c == nil || c.catalog == nil {
		return nil, nil
	}
	return c.agentEntriesForWorkspace(resolved), nil
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
	slices.SortFunc(records, func(left, right resources.Record[compozyconfig.AgentDef]) int {
		return strings.Compare(agentRecordSortKey(left), agentRecordSortKey(right))
	})
	lens := agentCatalogLensFor(resolved)
	type rankedEntry struct {
		entry core.AgentCatalogEntry
		rank  int
		key   string
	}
	merged := make(map[string]rankedEntry)
	for _, record := range records {
		rank, visible := lens.rank(record.Scope)
		if !visible {
			continue
		}
		name := strings.TrimSpace(record.Spec.Name)
		if name == "" {
			continue
		}
		sortKey := agentRecordSortKey(record)
		current, exists := merged[name]
		if exists && (current.rank > rank || current.rank == rank && current.key >= sortKey) {
			continue
		}
		workspaceID, workspaceScoped := lens.entryScope(record.Scope)
		origin := contract.AgentOriginGlobal
		if workspaceScoped {
			origin = contract.AgentOriginWorkspace
		}
		merged[name] = rankedEntry{entry: core.AgentCatalogEntry{
			Def: cloneAgentDef(record.Spec), Origin: origin, WorkspaceID: workspaceID,
		}, rank: rank, key: sortKey}
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	slices.Sort(names)
	entries := make([]core.AgentCatalogEntry, 0, len(names))
	for _, name := range names {
		entry := merged[name].entry
		entry.Def = cloneAgentDef(entry.Def)
		entries = append(entries, entry)
	}
	return entries
}
