package session

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// resolveSpawnPermissionPolicy turns the public spawn permission inputs into the
// concrete child policy. Inheritance and toolsets only select atoms from the
// parent's ceiling; ValidatePermissionSubset still guards the result.
func (m *Manager) resolveSpawnPermissionPolicy(parent *Info, opts SpawnOpts) (store.SessionPermissionPolicy, error) {
	ceiling := store.NormalizeSessionLineage(parent.ID, parent.Lineage).PermissionPolicy
	policy := opts.PermissionPolicy
	inheritTools := false
	if opts.InheritPermissions {
		policy = ceiling
		inheritTools = true
		if n := opts.PermissionNarrowing; n != nil {
			if n.Tools != nil {
				policy.Tools, inheritTools = n.Tools, false
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
	}
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
