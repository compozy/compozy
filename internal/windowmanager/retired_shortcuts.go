package windowmanager

// RetiredPaletteCommandIDs remain invocable as no-ops until v0.8.0.
func RetiredPaletteCommandIDs() []string {
	return []string{
		"window.minimize", "window.zoom", "window.toggle_floating", "window.merge_all", "window.tab.detach",
		"window.resize", "layout.frame_resize", "window.stack.group",
	}
}

func RetiredPaletteCommand(id string) bool {
	for _, retired := range RetiredPaletteCommandIDs() {
		if id == retired {
			return true
		}
	}
	return false
}

// RetiredShortcut accepts stored bindings without assigning any action until v0.8.0.
func RetiredShortcut(id string) bool {
	switch id {
	case "window.minimize", "window.zoom", "window.toggle_floating", "window.merge_all", "window.tab.detach",
		"window.tile.top-left", "window.tile.top-right", "window.tile.bottom-left", "window.tile.bottom-right":
		return true
	default:
		return false
	}
}

// CommandAlias translates the two public window IDs removed in v0.8.0.
func CommandAlias(id string) (string, bool) {
	switch CommandID(id) {
	case CommandWindowStackReorder:
		return string(CommandWindowTabReorder), true
	case CommandWindowStackSetActive:
		return string(CommandWindowTabActivate), true
	default:
		return id, false
	}
}
