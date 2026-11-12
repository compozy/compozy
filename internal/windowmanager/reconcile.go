package windowmanager

import "slices"

// ReconcileRegisteredApps removes retired apps and repairs their durable topology without mutating its input.
func ReconcileRegisteredApps(snapshot Snapshot, registered func(app string) bool) (Snapshot, bool) {
	return reconcileWindows(snapshot, func(window Window) bool { return registered(window.App) })
}

func reconcileWindows(snapshot Snapshot, keep func(Window) bool) (Snapshot, bool) {
	reconciled := cloneSnapshot(snapshot)
	changed := reconcileAppWindows(reconciled.Windows, keep)
	for _, entries := range [][]HistoryEntry{reconciled.History.Undo, reconciled.History.Redo} {
		for index := range entries {
			for _, state := range []*State{&entries[index].Before, &entries[index].After} {
				if reconcileAppWindows(state.Windows, keep) {
					changed = true
					normalized := NormalizeSnapshot(Snapshot{Desktops: state.Desktops, Windows: state.Windows})
					state.Desktops, state.Windows = normalized.Desktops, normalized.Windows
				}
			}
		}
	}
	closed := reconciled.ClosedEntries[:0]
	for _, entry := range reconciled.ClosedEntries {
		windows := entry.Windows[:0]
		for _, window := range entry.Windows {
			if !keep(window) {
				changed = true
				continue
			}
			if reconcileAppRoutes(&window) {
				changed = true
			}
			windows = append(windows, window)
		}
		entry.Windows = windows
		if len(windows) == 0 {
			continue
		}
		if entry.ActiveID != nil &&
			!slices.ContainsFunc(windows, func(window Window) bool { return window.ID == *entry.ActiveID }) {
			entry.ActiveID = new(windows[0].ID)
		}
		closed = append(closed, entry)
	}
	reconciled.ClosedEntries = closed
	if !changed {
		return snapshot, false
	}
	return NormalizeSnapshot(reconciled), true
}

func reconcileAppWindows(windows map[WindowID]Window, keep func(Window) bool) bool {
	changed := false
	for id, window := range windows {
		if !keep(window) {
			delete(windows, id)
			changed = true
			continue
		}
		if reconcileAppRoutes(&window) {
			windows[id] = window
			changed = true
		}
	}
	return changed
}

func reconcileAppRoutes(window *Window) bool {
	if window.App != "settings" {
		return false
	}
	changed := false
	if window.Route.Pathname == "/settings/memory" {
		window.Route.Pathname = "/settings"
		changed = true
	}
	for index := range window.NavStack {
		if window.NavStack[index].Pathname == "/settings/memory" {
			window.NavStack[index].Pathname = "/settings"
			changed = true
		}
	}
	return changed
}
