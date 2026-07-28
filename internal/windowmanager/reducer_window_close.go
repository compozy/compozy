package windowmanager

import "fmt"

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
	r.changes.window(command.WindowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

func (r *reducer) minimizeWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	// A zoomed window returns to its source before minimizing so its anchor is the real placement.
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
	if desktop.Purpose != DesktopPurposeFocus || desktop.FocusOwner == nil || *desktop.FocusOwner != windowID {
		return false
	}
	return true
}
