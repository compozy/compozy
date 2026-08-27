package windowmanager

import (
	"fmt"
	"slices"
)

const sessionEmptyRoutePath = "/sessions"

func (r *reducer) retireDeletedSessionWindows(snapshot *Snapshot, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, fmt.Errorf("deleted session id is required: %w", ErrInvalidCommand)
	}
	emptyRoute, err := CanonicalRouteIntent(RouteIntent{
		Pathname: sessionEmptyRoutePath,
		Search:   RouteSearch{},
	})
	if err != nil {
		return false, fmt.Errorf("session empty route: %w: %w", err, ErrInvalidCommand)
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
		window.Route = emptyRoute
		window.InstanceKey = nil
		window.NavStack = nil
		snapshot.Windows[windowID] = window
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
	return window.App == "session" && window.InstanceKey != nil && *window.InstanceKey == sessionID
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
		if len(remaining) != len(entry.Windows) && entry.ActiveID != nil {
			activePresent := slices.ContainsFunc(remaining, func(window Window) bool {
				return window.ID == *entry.ActiveID
			})
			if !activePresent {
				activeID := remaining[0].ID
				entry.ActiveID = &activeID
			}
		}
		entry.Windows = remaining
		entries = append(entries, entry)
	}
	if !changed {
		return false
	}
	snapshot.ClosedEntries = entries
	return true
}
