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
	type navigationState struct {
		route    RouteIntent
		navStack []RouteIntent
	}
	navigation := make(map[WindowID]navigationState, len(snapshot.Windows))
	for windowID, window := range snapshot.Windows {
		navigation[windowID] = navigationState{
			route:    cloneRouteIntent(window.Route),
			navStack: cloneRouteIntents(window.NavStack),
		}
	}
	restoreState(snapshot, state)
	for windowID, current := range navigation {
		window, exists := snapshot.Windows[windowID]
		if !exists {
			continue
		}
		window.Route = current.route
		window.NavStack = current.navStack
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
