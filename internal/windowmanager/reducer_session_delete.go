package windowmanager

import (
	"fmt"
	"slices"
	"strings"
)

const sessionEmptyRoutePath = "/sessions"

func (r *reducer) retireDeletedSessionWindows(snapshot *Snapshot, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, fmt.Errorf("deleted session id is required: %w", ErrInvalidCommand)
	}
	historyMatch := historyContainsDeletedSession(snapshot.History, sessionID)
	windowIDs := make([]WindowID, 0)
	for windowID, window := range snapshot.Windows {
		if windowBelongsToSession(window, sessionID) {
			windowIDs = append(windowIDs, windowID)
		}
	}
	slices.Sort(windowIDs)
	changed := false
	for _, windowID := range windowIDs {
		window, exists := snapshot.Windows[windowID]
		if !exists || !windowBelongsToSession(window, sessionID) {
			continue
		}
		removeWindow(snapshot, windowID)
		delete(snapshot.Windows, windowID)
		r.changes.window(windowID)
		changed = true
	}
	if removeDeletedSessionClosedEntries(snapshot, sessionID) {
		changed = true
	}
	if historyMatch {
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
	if window.App != "session" {
		return false
	}
	if window.InstanceKey != nil && *window.InstanceKey == sessionID {
		return true
	}
	matches := func(route RouteIntent) bool {
		return route.Pathname == "/sessions/"+sessionID || strings.HasPrefix(route.Pathname, "/sessions/"+sessionID+"/")
	}
	return matches(window.Route) || slices.ContainsFunc(window.NavStack, matches)
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
