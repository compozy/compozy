package windowmanager

import "fmt"

// restoreWindow returns a minimized window to where it left: the exact source
// group when its residue is unchanged, else the structural anchor, else beside
// the client's focused window on the same desktop, else floating. A window
// that was zoomed when it minimized comes back zoomed on that same desktop.
func (r *reducer) restoreWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", windowID, ErrWindowNotFound)
	}
	if !window.Minimized {
		return false, nil
	}
	anchor := cloneReturnAnchor(window.ReturnAnchor)
	removeWindow(snapshot, windowID)
	window.Minimized = false
	window.Zoomed = false
	window.ReturnAnchor = nil
	if anchor != nil && anchor.Zoomed {
		snapshot.Windows[windowID] = window
		return true, r.restoreZoomedWindow(snapshot, windowID, anchor)
	}
	if anchor != nil {
		if _, exists := desktopIndexByID(snapshot, anchor.DesktopID); exists {
			window.DesktopID = anchor.DesktopID
		}
	}
	snapshot.Windows[windowID] = window
	if restoreExactSourceGroup(snapshot, windowID, anchor) {
		r.changes.group(anchor.SourceGroup.ID)
		markNodeChanges(anchor.SourceGroup.Root, &r.changes)
	} else if err := insertAtAnchor(snapshot, windowID, anchor, r.focusedWindow, r.generate); err != nil {
		return false, err
	}
	window = snapshot.Windows[windowID]
	r.revealPlacedWindow(snapshot, windowID)
	r.changes.window(windowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

// restoreZoomedWindow re-zooms a window on the desktop it minimized from. The
// desktop's current zoom yields first, and the slot the zoom originally left
// stays the window's return anchor so unzoom still takes it home.
func (r *reducer) restoreZoomedWindow(snapshot *Snapshot, windowID WindowID, anchor *ReturnAnchor) error {
	window := snapshot.Windows[windowID]
	if members, zoomed := zoomedUnit(snapshot, window.DesktopID); zoomed {
		if ownerID, owned := zoomOwner(snapshot, members); owned {
			if err := r.unzoomUnit(snapshot, ownerID); err != nil {
				return err
			}
		}
	}
	leaf, err := newLeaf(windowID, r.generate)
	if err != nil {
		return err
	}
	anchor.Zoomed = false
	return r.placeZoomedUnit(snapshot, leaf, windowID, window.DesktopID, anchor)
}
