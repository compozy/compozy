package extensionpkg

// cmdPaletteCommandVisibleForResources applies one visibility rule to every
// command-palette projection. Actions that target a declared resource inherit
// that resource's profile placement; otherwise a hidden target could still be
// reached through a visible command.
func cmdPaletteCommandVisibleForResources(
	command CmdPaletteCommand,
	tools map[string]ToolConfig,
	views []CmdPaletteView,
	profileName string,
) bool {
	if !manifestPlacementVisible(command.Profile, profileName) {
		return false
	}
	switch command.Action.Kind {
	case cmdPaletteActionTool:
		tool, exists := tools[command.Action.Tool]
		return exists && manifestPlacementVisible(tool.Profile, profileName)
	case cmdPaletteActionView:
		for _, view := range views {
			if view.ID == command.Action.View {
				return cmdPaletteViewVisibleForResources(view, tools, profileName)
			}
		}
		return false
	default:
		return true
	}
}

func cmdPaletteViewVisibleForResources(
	view CmdPaletteView,
	tools map[string]ToolConfig,
	profileName string,
) bool {
	if !manifestPlacementVisible(view.Profile, profileName) {
		return false
	}
	if view.Source == nil {
		return true
	}
	tool, exists := tools[view.Source.Tool]
	return exists && manifestPlacementVisible(tool.Profile, profileName)
}
