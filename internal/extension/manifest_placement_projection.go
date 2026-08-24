package extensionpkg

import (
	"fmt"
	"strings"
)

// ManifestPlacement is one resource declaration and the profile name that owns
// it. An empty profile means the resource is available to every profile.
type ManifestPlacement struct {
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
	Profile  string `json:"profile,omitempty"`
}

type manifestPlacementEntry struct {
	kind     string
	resource string
	profile  string
	field    string
}

func walkManifestPlacements(m *Manifest, visit func(manifestPlacementEntry)) {
	if m == nil || visit == nil {
		return
	}
	appendPaths := func(kind, field string, values []ManifestResourcePath) {
		for index, value := range values {
			visit(manifestPlacementEntry{
				kind: kind, resource: value.Path, profile: value.Profile,
				field: fmt.Sprintf("resources.%s[%d].profile", field, index),
			})
		}
	}
	appendPaths("skill", "skills", m.Resources.Skills)
	appendPaths("loop", "loops", m.Resources.Loops)
	appendPaths("agent", "agents", m.Resources.Agents)
	appendPaths("automation", "automation", m.Resources.Automation)
	appendPaths("layout", "layouts", m.Resources.Layouts)
	for index := range m.Resources.Hooks {
		hook := &m.Resources.Hooks[index]
		visit(manifestPlacementEntry{
			kind: "hook", resource: hook.Name, profile: hook.Profile,
			field: fmt.Sprintf("resources.hooks[%d].profile", index),
		})
	}
	for _, name := range sortedMapKeys(m.Resources.Tools) {
		visit(manifestPlacementEntry{
			kind: cmdPaletteActionTool, resource: name, profile: m.Resources.Tools[name].Profile,
			field: "resources.tools." + name + ".profile",
		})
	}
	for _, name := range sortedMapKeys(m.Resources.MCPServers) {
		visit(manifestPlacementEntry{
			kind: "mcp_server", resource: name, profile: m.Resources.MCPServers[name].Profile,
			field: "resources.mcp_servers." + name + ".profile",
		})
	}
	for index, group := range m.Resources.CommandGroups {
		visit(manifestPlacementEntry{
			kind: "command_group", resource: group.Path, profile: group.Profile,
			field: fmt.Sprintf("resources.command_groups[%d].profile", index),
		})
	}
	for index, command := range m.Resources.CmdPalette.Commands {
		visit(manifestPlacementEntry{
			kind: "cmd_palette_command", resource: command.ID, profile: command.Profile,
			field: fmt.Sprintf("resources.cmd_palette.commands[%d].profile", index),
		})
	}
	for index, view := range m.Resources.CmdPalette.Views {
		visit(manifestPlacementEntry{
			kind: "cmd_palette_view", resource: view.ID, profile: view.Profile,
			field: fmt.Sprintf("resources.cmd_palette.views[%d].profile", index),
		})
	}
}

// PlacementMatrix returns every profile-aware resource declaration in stable order.
func (m *Manifest) PlacementMatrix() []ManifestPlacement {
	if m == nil {
		return nil
	}
	placements := make([]ManifestPlacement, 0)
	walkManifestPlacements(m, func(entry manifestPlacementEntry) {
		placements = append(placements, ManifestPlacement{
			Kind: strings.TrimSpace(entry.kind), Resource: strings.TrimSpace(entry.resource),
			Profile: strings.TrimSpace(entry.profile),
		})
	})
	for index := range placements {
		placements[index].Kind = strings.TrimSpace(placements[index].Kind)
		placements[index].Resource = strings.TrimSpace(placements[index].Resource)
		placements[index].Profile = strings.TrimSpace(placements[index].Profile)
	}
	return placements
}

// DormantPlacements returns name-bound declarations whose profile is absent.
func (m *Manifest) DormantPlacements(profileNames []string) []ManifestPlacement {
	known := make(map[string]struct{}, len(profileNames)+1)
	known[hostAPIBridgesDefaultKey] = struct{}{}
	for _, name := range profileNames {
		known[strings.TrimSpace(name)] = struct{}{}
	}
	placements := m.PlacementMatrix()
	dormant := make([]ManifestPlacement, 0)
	for _, placement := range placements {
		if placement.Profile == "" {
			continue
		}
		if _, exists := known[placement.Profile]; !exists {
			dormant = append(dormant, placement)
		}
	}
	return dormant
}
