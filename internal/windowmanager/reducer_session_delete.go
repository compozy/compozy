package windowmanager

import (
	"fmt"
	"slices"
	"strings"
)

const (
	sessionWindowApp      = "session"
	sessionEmptyRoutePath = "/sessions"
)

func (r *reducer) retireDeletedSessionWindows(snapshot *Snapshot, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, fmt.Errorf("deleted session id is required: %w", ErrInvalidCommand)
	}
	changed := historyContainsDeletedSession(snapshot.History, sessionID)
	for windowID, window := range snapshot.Windows {
		if window.App != sessionWindowApp {
			continue
		}
		if windowShowsSession(window, sessionID) {
			if _, err := r.navigateWindow(snapshot, NavigateWindowCommand{
				WindowID:    windowID,
				Mode:        NavigateReplace,
				InstanceKey: new(""),
				Route:       RouteIntent{Pathname: sessionEmptyRoutePath, Search: RouteSearch{}},
			}); err != nil {
				return false, err
			}
			changed = true
			continue
		}
		stack := slices.DeleteFunc(window.NavStack, func(route RouteIntent) bool {
			return routeShowsSession(route, sessionID)
		})
		if len(stack) != len(window.NavStack) {
			window.NavStack = stack
			snapshot.Windows[windowID] = window
			r.changes.window(windowID)
			changed = true
		}
	}
	if removeDeletedSessionClosedEntries(snapshot, sessionID) {
		changed = true
	}
	if changed {
		// Deletion must not be undone: leftover history would resurrect the session tab.
		snapshot.History.Undo = []HistoryEntry{}
		snapshot.History.Redo = []HistoryEntry{}
	}
	return changed, nil
}

func historyContainsDeletedSession(history History, sessionID string) bool {
	for _, entries := range [][]HistoryEntry{history.Undo, history.Redo} {
		for _, entry := range entries {
			if stateContainsDeletedSession(entry.Before, sessionID) ||
				stateContainsDeletedSession(entry.After, sessionID) {
				return true
			}
		}
	}
	return false
}

func stateContainsDeletedSession(state State, sessionID string) bool {
	for _, window := range state.Windows {
		if windowBelongsToSession(window, sessionID) {
			return true
		}
	}
	return false
}

func windowBelongsToSession(window Window, sessionID string) bool {
	if window.App != sessionWindowApp {
		return false
	}
	return windowShowsSession(window, sessionID) || slices.ContainsFunc(window.NavStack, func(route RouteIntent) bool {
		return routeShowsSession(route, sessionID)
	})
}

func windowShowsSession(window Window, sessionID string) bool {
	return valueOrZero(window.InstanceKey) == sessionID || routeShowsSession(window.Route, sessionID)
}

func routeShowsSession(route RouteIntent, sessionID string) bool {
	path := "/sessions/" + sessionID
	return route.Pathname == path || strings.HasPrefix(route.Pathname, path+"/")
}

func removeDeletedSessionClosedEntries(snapshot *Snapshot, sessionID string) bool {
	if len(snapshot.ClosedEntries) == 0 {
		return false
	}
	changed := false
	entries := make([]ClosedEntry, 0, len(snapshot.ClosedEntries))
	for _, entry := range snapshot.ClosedEntries {
		remaining := make([]Window, 0, len(entry.Windows))
		for _, window := range entry.Windows {
			if windowBelongsToSession(window, sessionID) {
				changed = true
				continue
			}
			remaining = append(remaining, window)
		}
		if len(remaining) == 0 {
			continue
		}

		entry.Windows = remaining
		entries = append(entries, pruneClosedEntry(entry))
	}
	if !changed {
		return false
	}
	snapshot.ClosedEntries = entries
	return true
}
