package windowmanager

import "slices"

const (
	retiredWindowMinimize  = "window.minimize"
	retiredWindowMergeAll  = "window.merge_all"
	retiredWindowTabDetach = "window.tab.detach"
)

// RetiredPaletteCommandIDs remain invocable as no-ops until v0.8.0.
func RetiredPaletteCommandIDs() []string {
	return []string{
		retiredWindowMinimize, string(CommandWindowZoom), string(CommandWindowToggleFloating),
		retiredWindowMergeAll, retiredWindowTabDetach,
		string(CommandWindowResize), string(CommandLayoutFrameResize), string(CommandWindowStackGroup),
	}
}

func RetiredPaletteCommand(id string) bool {
	return slices.Contains(RetiredPaletteCommandIDs(), id)
}

// RetiredShortcut accepts stored bindings without assigning any action until v0.8.0.
func RetiredShortcut(id string) bool {
	switch id {
	case retiredWindowMinimize, string(CommandWindowZoom), string(CommandWindowToggleFloating),
		retiredWindowMergeAll, retiredWindowTabDetach,
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
