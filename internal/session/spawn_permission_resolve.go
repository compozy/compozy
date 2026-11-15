package session

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// resolveSpawnPermissionPolicy turns the public spawn permission inputs into the
// concrete child policy. Inheritance and toolsets only select atoms from the
// parent's ceiling; ValidatePermissionSubset still guards the result.
func (m *Manager) resolveSpawnPermissionPolicy(
	ctx context.Context, parent *Info, opts *SpawnOpts,
) (store.SessionPermissionPolicy, error) {
	ceiling, err := m.EffectiveMCPAuthority(WithMCPAuthoritySession(ctx, parent), parent.ID)
	if err != nil {
		return store.SessionPermissionPolicy{}, err
	}
	policy := opts.PermissionPolicy
	inheritTools := false
	if opts.InheritPermissions {
		policy = ceiling
		inheritTools = true
		if n := opts.PermissionNarrowing; n != nil {
			if n.Tools != nil {
				policy.Tools, inheritTools = n.Tools, false
				policy.MaterializedTools = false
			}
			if n.Skills != nil {
				policy.Skills = n.Skills
			}
			if n.MCPServers != nil {
				policy.MCPServers = n.MCPServers
			}
			if n.WorkspacePaths != nil {
				policy.WorkspacePaths = n.WorkspacePaths
			}
		}
	}
	if len(opts.Toolsets) > 0 {
		selected, err := m.expandSpawnToolsets(opts.Toolsets, ceiling.Tools)
		if err != nil {
			return store.SessionPermissionPolicy{}, err
		}
		// Toolsets name the wanted tools, so they replace an inherited full ceiling.
		base := policy.Tools
		if inheritTools {
			base = nil
		}
		policy.Tools = append(slices.Clone(base), selected...)
		policy.MaterializedTools = false
	}
	return bindSpawnPermissionIntent(parent, opts, ceiling, policy)
}

// bindSpawnPermissionIntent records the immutable inherited bounds, separately from live authority.
func bindSpawnPermissionIntent(
	parent *Info, opts *SpawnOpts, ceiling, policy store.SessionPermissionPolicy,
) (store.SessionPermissionPolicy, error) {
	lineage := store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	stored := store.DecodeIntent(lineage.PermissionPolicy, lineage.Kind, string(parent.Type))
	sameScope, mcpNamed, toolsNamed := spawnPermissionSelections(parent, opts)
	if toolsNamed {
		if err := validatePermissionAtoms("tools", ceiling.Tools, policy.Tools); err != nil {
			return store.SessionPermissionPolicy{}, err
		}
	}
	if mcpNamed {
		if err := validatePermissionAtoms("mcp_servers", ceiling.MCPServers, policy.MCPServers); err != nil {
			return store.SessionPermissionPolicy{}, err
		}
	}
	policy.AncestorMCPCeiling = stored.AncestorMCPCeiling
	if stored.MCPServersMode == store.PermissionList {
		policy.AncestorMCPCeiling = store.IntersectMCPCeiling(policy.AncestorMCPCeiling, stored.MCPServers)
	}
	if !sameScope {
		policy.AncestorMCPCeiling = store.IntersectMCPCeiling(policy.AncestorMCPCeiling, ceiling.MCPServers)
	}
	policy.MCPServersMode = store.PermissionInherit
	if mcpNamed {
		policy.MCPServersMode = store.PermissionList
	}
	policy.ToolsMode = store.PermissionList
	if stored.ToolsMode == store.PermissionInherit && sameScope && !toolsNamed {
		policy.ToolsMode = store.PermissionInherit
	}
	if !toolsNamed {
		policy.Tools = stored.Tools
		if !sameScope {
			policy.Tools = ceiling.Tools
		}
	}
	if policy.MCPServersMode == store.PermissionInherit {
		policy.MCPServers = nil
	}
	policy.MaterializedMCPServers = false
	return store.NormalizeSessionPermissionPolicy(policy), nil
}

// expandSpawnToolsets returns the parent-granted members of the named toolsets.
func (m *Manager) expandSpawnToolsets(names []string, ceiling []string) ([]string, error) {
	granted := make(map[string]struct{}, len(ceiling))
	for _, atom := range ceiling {
		granted[strings.TrimSpace(atom)] = struct{}{}
	}
	selected := make([]string, 0)
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		id := toolspkg.ToolsetID(name)
		if err := id.Validate(); err != nil {
			return nil, &SpawnPermissionError{Field: "toolsets", Value: name, Rule: SpawnPermissionRuleUnknownToolset}
		}
		expanded, err := m.toolsetCatalog.Expand(id, m.toolUniverse)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: %w",
				&SpawnPermissionError{Field: "toolsets", Value: name, Rule: SpawnPermissionRuleUnknownToolset},
				err,
			)
		}
		for _, tool := range expanded {
			if _, ok := granted[tool.String()]; ok {
				selected = append(selected, tool.String())
			}
		}
	}
	return selected, nil
}

func spawnPermissionSelections(parent *Info, opts *SpawnOpts) (sameScope, mcpNamed, toolsNamed bool) {
	sameScope = opts.Workspace == "" || opts.Workspace == parent.WorkspaceID ||
		(opts.Workspace == workspaceaccess.UserSelector && parent.WorkspaceID == "")
	mcpNamed = opts.PermissionPolicy.MCPServersMode == store.PermissionList ||
		(len(opts.PermissionPolicy.MCPServers) > 0 && opts.PermissionPolicy.MCPServersMode != store.PermissionInherit) ||
		(opts.PermissionNarrowing != nil && opts.PermissionNarrowing.MCPServers != nil)
	toolsNamed = len(opts.Toolsets) > 0 || opts.PermissionPolicy.ToolsMode == store.PermissionList ||
		(len(opts.PermissionPolicy.Tools) > 0 && opts.PermissionPolicy.ToolsMode != store.PermissionInherit) ||
		(opts.PermissionNarrowing != nil && opts.PermissionNarrowing.Tools != nil)

	return sameScope, mcpNamed, toolsNamed
}
