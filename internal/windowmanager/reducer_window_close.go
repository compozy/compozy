package windowmanager

import (
	"fmt"
	"slices"
)

func (r *reducer) closeWindow(snapshot *Snapshot, command CloseWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if command.Minimize && window.Minimized {
		return false, nil
	}
	if command.Minimize {
		return r.minimizeWindow(snapshot, command.WindowID)
	}
	if !removeWindow(snapshot, command.WindowID) {
		return false, fmt.Errorf("window %q has no placement: %w", command.WindowID, ErrInvalidTopology)
	}
	delete(snapshot.Windows, command.WindowID)
	r.removeClosedFocusDesktop(snapshot, command.WindowID)
	r.changes.window(command.WindowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

func (r *reducer) minimizeWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	// A zoomed window returns to its source before minimizing so its anchor is the real placement.
	if focusIndex, owned := ownedFocusDesktopIndex(snapshot, windowID); owned {
		if _, err := r.restoreZoomedWindow(snapshot, windowID, focusIndex); err != nil {
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
		return false, fmt.Errorf(
			"window %q desktop %q: %w",
			windowID,
			window.DesktopID,
			ErrInvalidTopology,
		)
	}
	snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
	r.changes.window(windowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

func ownedFocusDesktopIndex(snapshot *Snapshot, windowID WindowID) (int, bool) {
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return -1, false
	}
	index, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return -1, false
	}
	desktop := snapshot.Desktops[index]
	if desktop.Purpose != DesktopPurposeFocus || desktop.FocusOwner == nil || *desktop.FocusOwner != windowID {
		return -1, false
	}
	return index, true
}

func (r *reducer) removeClosedFocusDesktop(snapshot *Snapshot, windowID WindowID) {
	for index := range slices.Backward(snapshot.Desktops) {
		desktop := snapshot.Desktops[index]
		if desktop.Purpose != DesktopPurposeFocus || desktop.FocusOwner == nil || *desktop.FocusOwner != windowID {
			continue
		}
		// Windows opened onto the focus desktop during zoom must outlive the owner's close.
		residents := len(desktop.Groups) > 0 || len(desktop.Floating) > 0
		if residents || len(snapshot.Desktops) == 1 {
			snapshot.Desktops[index].Purpose = DesktopPurposeStandard
			snapshot.Desktops[index].FocusOwner = nil
		} else {
			snapshot.Desktops = append(snapshot.Desktops[:index], snapshot.Desktops[index+1:]...)
		}
		r.changes.desktop(desktop.ID)
	}
}
