package windowmanager

import "time"

// State is the revision-independent content restored by history.
type State struct {
	Desktops  []Desktop
	Windows   map[WindowID]Window
	Overrides WorkspaceConfig
}

// HistoryEntry stores an exact operation boundary.
type HistoryEntry struct {
	CommandID CommandID
	Before    State
	After     State
	Actor     Actor
	Origin    string
	CreatedAt time.Time
}

// History contains bounded undo and redo stacks.
type History struct {
	Undo []HistoryEntry `json:"undo"`
	Redo []HistoryEntry `json:"redo"`
}

func snapshotState(snapshot Snapshot) State {
	return State{
		Desktops:  cloneDesktops(snapshot.Desktops),
		Windows:   cloneWindows(snapshot.Windows),
		Overrides: cloneWorkspaceConfig(snapshot.Overrides),
	}
}

func restoreState(snapshot *Snapshot, state State) {
	snapshot.Desktops = cloneDesktops(state.Desktops)
	snapshot.Windows = cloneWindows(state.Windows)
	snapshot.Overrides = cloneWorkspaceConfig(state.Overrides)
}

func restoreStatePreservingRoutes(snapshot *Snapshot, state State) {
	routes := make(map[WindowID]RouteIntent, len(snapshot.Windows))
	for windowID, window := range snapshot.Windows {
		routes[windowID] = cloneRouteIntent(window.Route)
	}
	restoreState(snapshot, state)
	for windowID, route := range routes {
		window, exists := snapshot.Windows[windowID]
		if !exists {
			continue
		}
		window.Route = route
		snapshot.Windows[windowID] = window
	}
}

func appendHistory(snapshot *Snapshot, entry HistoryEntry, limit int) {
	snapshot.History.Undo = append(snapshot.History.Undo, cloneHistoryEntry(entry))
	if len(snapshot.History.Undo) > limit {
		snapshot.History.Undo = append([]HistoryEntry(nil), snapshot.History.Undo[len(snapshot.History.Undo)-limit:]...)
	}
	snapshot.History.Redo = nil
}
