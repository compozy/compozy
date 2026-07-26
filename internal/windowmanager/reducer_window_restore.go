package windowmanager

import "fmt"

func (r *reducer) restoreWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", windowID, ErrWindowNotFound)
	}
	if !window.Minimized {
		return false, nil
	}
	removeWindow(snapshot, windowID)
	window.Minimized = false
	if window.ReturnAnchor != nil {
		if _, exists := desktopIndexByID(snapshot, window.ReturnAnchor.DesktopID); exists {
			window.DesktopID = window.ReturnAnchor.DesktopID
		}
	}
	snapshot.Windows[windowID] = window
	if err := insertAtAnchor(snapshot, windowID, window.ReturnAnchor, r.focusedWindow, r.generate); err != nil {
		return false, err
	}
	window = snapshot.Windows[windowID]
	window.ReturnAnchor = nil
	snapshot.Windows[windowID] = window
	r.changes.window(windowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}
