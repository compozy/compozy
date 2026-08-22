package extensionpkg

import "strings"

// ManifestPlacement is one resource declaration and the profile name that owns
// it. An empty profile means the resource is available to every profile.
type ManifestPlacement struct {
	Kind     string `json:"kind"`
	Resource string `json:"resource"`
	Profile  string `json:"profile,omitempty"`
}

// PlacementMatrix returns every profile-aware resource declaration in stable order.
func (m *Manifest) PlacementMatrix() []ManifestPlacement {
	if m == nil {
		return nil
	}
	placements := make([]ManifestPlacement, 0)
	appendPaths := func(kind string, values []ManifestResourcePath) {
		for _, value := range values {
			placements = append(placements, ManifestPlacement{
				Kind: kind, Resource: value.Path, Profile: strings.TrimSpace(value.Profile),
			})
		}
	}
	appendPaths("skill", m.Resources.Skills)
	appendPaths("loop", m.Resources.Loops)
	appendPaths("agent", m.Resources.Agents)
	appendPaths("automation", m.Resources.Automation)
	appendPaths("layout", m.Resources.Layouts)
	for _, hook := range m.Resources.Hooks {
		placements = append(placements, ManifestPlacement{Kind: "hook", Resource: hook.Name, Profile: hook.Profile})
	}
	for _, name := range sortedMapKeys(m.Resources.Tools) {
		placements = append(placements, ManifestPlacement{Kind: "tool", Resource: name, Profile: m.Resources.Tools[name].Profile})
	}
	for _, name := range sortedMapKeys(m.Resources.MCPServers) {
		placements = append(placements, ManifestPlacement{Kind: "mcp_server", Resource: name, Profile: m.Resources.MCPServers[name].Profile})
	}
	for _, group := range m.Resources.CommandGroups {
		placements = append(placements, ManifestPlacement{Kind: "command_group", Resource: group.Path, Profile: group.Profile})
	}
	for _, command := range m.Resources.CmdPalette.Commands {
		placements = append(placements, ManifestPlacement{Kind: "cmd_palette_command", Resource: command.ID, Profile: command.Profile})
	}
	for _, view := range m.Resources.CmdPalette.Views {
		placements = append(placements, ManifestPlacement{Kind: "cmd_palette_view", Resource: view.ID, Profile: view.Profile})
	}
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
	known["default"] = struct{}{}
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
