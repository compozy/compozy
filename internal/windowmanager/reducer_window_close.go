package windowmanager

import (
	"fmt"
	"slices"
	"time"
)

func (r *reducer) closeWindow(snapshot *Snapshot, command CloseWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if command.Minimize {
		if command.Scope != CloseScopeTab {
			return false, fmt.Errorf("minimize cannot include a close scope: %w", ErrInvalidCommand)
		}
		if window.Minimized {
			return false, nil
		}
		return r.minimizeWindow(snapshot, command.WindowID)
	}
	if !validCloseScope(command.Scope) {
		return false, fmt.Errorf("close scope %q: %w", command.Scope, ErrInvalidCommand)
	}
	if command.Scope == CloseScopeTab && window.Pinned {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowPinned)
	}
	entry, closeIDs, err := r.closedEntryForCommand(snapshot, command)
	if err != nil || len(closeIDs) == 0 {
		return false, err
	}
	for _, windowID := range closeIDs {
		closedWindow := snapshot.Windows[windowID]
		if !removeWindow(snapshot, windowID) {
			return false, fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
		}
		delete(snapshot.Windows, windowID)
		r.changes.window(windowID)
		r.changes.desktop(closedWindow.DesktopID)
	}
	if entry.StackID != nil {
		if _, stillStacked := findStackByID(snapshot, *entry.StackID); !stillStacked {
			r.changes.stackUngrouped(*entry.StackID)
		}
	}
	pushClosedEntry(snapshot, entry, r.config.ClosedEntryLimit)
	return true, nil
}

func (r *reducer) closeDeletedSessionWindows(snapshot *Snapshot, sessionID string) (bool, error) {
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
		stack, stacked := findStackByWindow(snapshot, windowID)
		if !removeWindow(snapshot, windowID) {
			return false, fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
		}
		delete(snapshot.Windows, windowID)
		r.changes.window(windowID)
		r.changes.desktop(window.DesktopID)
		if stacked {
			stackID := stack.id()
			if _, stillStacked := findStackByID(snapshot, stackID); !stillStacked {
				r.changes.stackUngrouped(stackID)
			}
		}
		changed = true
	}
	if removeDeletedSessionClosedEntries(snapshot, sessionID) {
		changed = true
	}
	if historyMatch {
		changed = true
	}
	if changed {
		// Session deletion is destructive. Discard stale layout snapshots that
		// could otherwise resurrect the deleted session through undo or redo.
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
			activePresent := false
			for _, window := range remaining {
				if window.ID == *entry.ActiveID {
					activePresent = true
					break
				}
			}
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

func validCloseScope(scope CloseScope) bool {
	switch scope {
	case CloseScopeTab, CloseScopeGroup, CloseScopeOthers, CloseScopeRight:
		return true
	default:
		return false
	}
}

func (r *reducer) closedEntryForCommand(
	snapshot *Snapshot,
	command CloseWindowCommand,
) (ClosedEntry, []WindowID, error) {
	window := snapshot.Windows[command.WindowID]
	placement, found := findWindowPlacement(snapshot, command.WindowID)
	if !found {
		return ClosedEntry{}, nil, fmt.Errorf("window %q has no placement: %w", command.WindowID, ErrInvalidTopology)
	}
	closeIDs := []WindowID{command.WindowID}
	entry := ClosedEntry{DesktopID: window.DesktopID, Rect: window.FloatingRect, ClosedAt: r.nowUTC()}
	if placement.groupIndex >= 0 {
		entry.Rect = snapshot.Desktops[placement.desktopIndex].Groups[placement.groupIndex].Frame
	}
	if location, stacked := findStackByWindow(snapshot, command.WindowID); stacked {
		stackID := location.id()
		entry.StackID = &stackID
		entry.ActiveID = clonePointer(location.activeID())
		entry.Rect = location.rect(snapshot)
		members := append([]WindowID(nil), location.members()...)
		switch command.Scope {
		case CloseScopeGroup:
			closeIDs = members
		case CloseScopeOthers:
			closeIDs = closeStackMembers(snapshot, members, command.WindowID, false)
		case CloseScopeRight:
			closeIDs = closeStackMembers(snapshot, members, command.WindowID, true)
		}
	} else {
		switch command.Scope {
		case CloseScopeOthers, CloseScopeRight:
			return ClosedEntry{}, nil, nil
		case CloseScopeGroup, CloseScopeTab:
		}
		activeID := command.WindowID
		entry.ActiveID = &activeID
	}
	entry.Windows = make([]Window, 0, len(closeIDs))
	for _, windowID := range closeIDs {
		closedWindow := snapshot.Windows[windowID]
		closedWindow.ReturnAnchor = captureReturnAnchor(snapshot, windowID)
		entry.Windows = append(entry.Windows, closedWindow)
	}
	return entry, closeIDs, nil
}

func closeStackMembers(snapshot *Snapshot, members []WindowID, selected WindowID, rightOnly bool) []WindowID {
	selectedIndex := 0
	for index, windowID := range members {
		if windowID == selected {
			selectedIndex = index
			break
		}
	}
	closeIDs := make([]WindowID, 0, len(members))
	for index, windowID := range members {
		if windowID == selected || snapshot.Windows[windowID].Pinned || (rightOnly && index <= selectedIndex) {
			continue
		}
		closeIDs = append(closeIDs, windowID)
	}
	return closeIDs
}

func (r *reducer) nowUTC() time.Time {
	if r.now == nil {
		return time.Now().UTC()
	}
	return r.now().UTC()
}

func (r *reducer) minimizeWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	if ownsFocusDesktop(snapshot, windowID) {
		if _, err := r.restoreZoomedWindow(snapshot, windowID); err != nil {
			return false, err
		}
	}
	window := snapshot.Windows[windowID]
	anchor := captureReturnAnchor(snapshot, windowID)
	if !removeWindow(snapshot, windowID) {
		return false, fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
	}
	window.Minimized = true
	window.Placement = WindowPlacementFloating
	window.ReturnAnchor = anchor
	window.FloatingRect = clampRect(window.FloatingRect)
	snapshot.Windows[windowID] = window
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return false, fmt.Errorf("window %q desktop %q: %w", windowID, window.DesktopID, ErrInvalidTopology)
	}
	snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
	r.changes.window(windowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

func ownsFocusDesktop(snapshot *Snapshot, windowID WindowID) bool {
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return false
	}
	index, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return false
	}
	desktop := snapshot.Desktops[index]
	return desktop.Purpose == DesktopPurposeFocus && desktop.FocusOwner != nil && *desktop.FocusOwner == windowID
}
