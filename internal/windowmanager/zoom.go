package windowmanager

import "slices"

// zoomUnitMembers returns every window sharing the zoom unit of the window:
// the whole tab frame when it is stacked, else the window alone.
func zoomUnitMembers(snapshot *Snapshot, windowID WindowID) []WindowID {
	if location, stacked := findStackByWindow(snapshot, windowID); stacked {
		return slices.Clone(location.members())
	}
	return []WindowID{windowID}
}

// zoomOwner returns the member that holds the unit's zoom.
func zoomOwner(snapshot *Snapshot, members []WindowID) (WindowID, bool) {
	for _, memberID := range members {
		if window, exists := snapshot.Windows[memberID]; exists && window.Zoomed && !window.Minimized {
			return memberID, true
		}
	}
	return "", false
}

func unitZoomed(snapshot *Snapshot, members []WindowID) bool {
	_, zoomed := zoomOwner(snapshot, members)
	return zoomed
}

// zoomedUnit returns the members of the desktop's zoomed unit, if any.
func zoomedUnit(snapshot *Snapshot, desktopID DesktopID) ([]WindowID, bool) {
	for windowID, window := range snapshot.Windows {
		if window.DesktopID == desktopID && window.Zoomed && !window.Minimized {
			return zoomUnitMembers(snapshot, windowID), true
		}
	}
	return nil, false
}

// desktopOccupied reports whether the desktop shows any window outside the unit.
func desktopOccupied(snapshot *Snapshot, desktopID DesktopID, unit []WindowID) bool {
	for windowID, window := range snapshot.Windows {
		if window.DesktopID == desktopID && !window.Minimized && !slices.Contains(unit, windowID) {
			return true
		}
	}
	return false
}

// liftedZoomDesktops lists desktops holding a unit that zoom carried in from
// another desktop, including one whose zoomed window is minimized right now.
func liftedZoomDesktops(snapshot *Snapshot) map[DesktopID]struct{} {
	lifted := make(map[DesktopID]struct{})
	for _, window := range snapshot.Windows {
		anchor := window.ReturnAnchor
		if anchor == nil || anchor.DesktopID == window.DesktopID {
			continue
		}
		if window.Zoomed || (window.Minimized && anchor.Zoomed) {
			lifted[window.DesktopID] = struct{}{}
		}
	}
	return lifted
}

// endZoom ends the zoom of each window and forgets the slot it would return
// to: the unit now belongs to the tree it sits in.
func (r *reducer) endZoom(snapshot *Snapshot, windowIDs []WindowID) bool {
	changed := false
	for _, windowID := range windowIDs {
		window, exists := snapshot.Windows[windowID]
		if !exists || !window.Zoomed {
			continue
		}
		window.Zoomed = false
		window.ReturnAnchor = nil
		snapshot.Windows[windowID] = window
		r.changes.window(windowID)
		changed = true
	}
	return changed
}

// clearDesktopZoom ends the zoom on one desktop; structural changes call it so
// the tree they edit becomes visible again.
func (r *reducer) clearDesktopZoom(snapshot *Snapshot, desktopID DesktopID) {
	for windowID, window := range snapshot.Windows {
		if window.DesktopID == desktopID && window.Zoomed {
			r.endZoom(snapshot, []WindowID{windowID})
		}
	}
}

// revealPlacedWindow ends a desktop zoom that would hide a visible window the
// command just placed outside the zoomed unit.
func (r *reducer) revealPlacedWindow(snapshot *Snapshot, windowID WindowID) {
	window, exists := snapshot.Windows[windowID]
	if !exists || window.Minimized {
		return
	}
	members, zoomed := zoomedUnit(snapshot, window.DesktopID)
	if !zoomed || containsWindowID(members, windowID) {
		return
	}
	r.clearDesktopZoom(snapshot, window.DesktopID)
}

// zoomHeir picks the member that inherits a departing member's zoom so a
// zoomed tab frame stays zoomed when one tab leaves.
func zoomHeir(members []WindowID, activeID *WindowID, departing WindowID) (WindowID, bool) {
	if activeID != nil && *activeID != departing && containsWindowID(members, *activeID) {
		return *activeID, true
	}
	index := slices.Index(members, departing)
	if index < 0 {
		return "", false
	}
	remaining := slices.Concat(members[:index], members[index+1:])
	heir := nextActiveAfterRemoval(remaining, index)
	if heir == nil {
		return "", false
	}
	return *heir, true
}

// forEachDesktopUnit visits every arrangement unit of a desktop in a stable
// order: tiled units in tree order, then floating frames, then floating windows.
func forEachDesktopUnit(desktop *Desktop, visit func(members []WindowID)) {
	for groupIndex := range desktop.Groups {
		forEachNodeUnit(&desktop.Groups[groupIndex].Root, visit)
	}
	for stackIndex := range desktop.FloatingStacks {
		visit(desktop.FloatingStacks[stackIndex].WindowIDs)
	}
	for _, windowID := range desktop.Floating {
		visit([]WindowID{windowID})
	}
}

func forEachNodeUnit(node *LayoutNode, visit func(members []WindowID)) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil {
			visit([]WindowID{*node.WindowID})
		}
	case NodeKindStack:
		visit(node.WindowIDs)
	case NodeKindSplit:
		for index := range node.Children {
			forEachNodeUnit(&node.Children[index], visit)
		}
	}
}
