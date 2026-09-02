package windowmanager

import (
	"fmt"
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
	window := snapshot.Windows[windowID]
	anchor := minimizeAnchor(snapshot, window)
	if !removeWindow(snapshot, windowID) {
		return false, fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
	}
	window.Minimized = true
	window.Zoomed = false
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

// minimizeAnchor keeps the slot a solo zoomed window will return to, so restore
// re-zooms it there and unzoom still takes it home. A zoomed tab leaves its
// zoom with the deck and anchors to its deck slot like any other tab.
func minimizeAnchor(snapshot *Snapshot, window Window) *ReturnAnchor {
	if _, stacked := findStackByWindow(snapshot, window.ID); stacked || !window.Zoomed {
		return captureReturnAnchor(snapshot, window.ID)
	}
	anchor := cloneReturnAnchor(window.ReturnAnchor)
	if anchor == nil {
		anchor = captureReturnAnchor(snapshot, window.ID)
	}
	if anchor != nil {
		anchor.Zoomed = true
	}
	return anchor
}
