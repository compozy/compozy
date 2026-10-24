package windowmanager

import "encoding/json"

// RebindWorkspace rewrites every retained window binding from one workspace reference to another:
// current routes, navigation stacks, closed entries, and undo/redo states. Migrations use it when the
// bound work itself moves to a new scope, so migrated windows keep opening the same items instead of
// being retired as stale bindings.
func RebindWorkspace(snapshot *Snapshot, from string, to string) bool {
	if snapshot == nil || from == "" || from == to {
		return false
	}
	encoded, err := json.Marshal(to)
	if err != nil {
		return false
	}
	changed := false
	rebindRoute := func(route *RouteIntent) {
		if routeBindsWorkspace(*route, from) {
			route.Search["workspace"] = encoded
			changed = true
		}
	}
	rebindWindow := func(window *Window) {
		rebindRoute(&window.Route)
		for i := range window.NavStack {
			rebindRoute(&window.NavStack[i])
		}
	}
	rebindWindows := func(windows map[WindowID]Window) {
		for id, window := range windows {
			rebindWindow(&window)
			windows[id] = window
		}
	}
	rebindWindows(snapshot.Windows)
	for i := range snapshot.ClosedEntries {
		for j := range snapshot.ClosedEntries[i].Windows {
			rebindWindow(&snapshot.ClosedEntries[i].Windows[j])
		}
	}
	for _, entries := range [][]HistoryEntry{snapshot.History.Undo, snapshot.History.Redo} {
		for i := range entries {
			rebindWindows(entries[i].Before.Windows)
			rebindWindows(entries[i].After.Windows)
		}
	}
	return changed
}
