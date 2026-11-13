package windowmanager

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
