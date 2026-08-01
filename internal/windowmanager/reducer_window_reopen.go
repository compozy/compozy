package windowmanager

func (r *reducer) reopenWindow(snapshot *Snapshot) bool {
	if len(snapshot.ClosedEntries) == 0 {
		return false
	}
	entry := snapshot.ClosedEntries[len(snapshot.ClosedEntries)-1]
	snapshot.ClosedEntries = snapshot.ClosedEntries[:len(snapshot.ClosedEntries)-1]
	reopen := make([]Window, 0, len(entry.Windows))
	for _, window := range entry.Windows {
		if _, live := snapshot.Windows[window.ID]; !live {
			reopen = append(reopen, window)
		}
	}
	if len(reopen) == 0 {
		return true
	}
	desktopID := r.reopenDesktop(snapshot, entry.DesktopID)
	for _, window := range reopen {
		window.DesktopID = desktopID
		window.Minimized = false
		window.ReturnAnchor = nil
		snapshot.Windows[window.ID] = window
		r.changes.window(window.ID)
	}

	if entry.StackID != nil {
		if location, found := findStackByID(snapshot, *entry.StackID); found {
			r.rejoinClosedWindows(snapshot, location, reopen, entry.ActiveID)
			r.changes.desktop(snapshot.Desktops[location.desktopIndex].ID)
			return true
		}
	}
	if len(reopen) >= 2 && entry.StackID != nil {
		windowIDs := make([]WindowID, 0, len(reopen))
		for _, window := range reopen {
			windowIDs = append(windowIDs, window.ID)
		}
		windowIDs = collatePinned(windowIDs, snapshot.Windows)
		activeID := windowIDs[0]
		if entry.ActiveID != nil && containsWindowID(windowIDs, *entry.ActiveID) {
			activeID = *entry.ActiveID
		}
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		stackID := *entry.StackID
		snapshot.Desktops[desktopIndex].FloatingStacks = append(
			snapshot.Desktops[desktopIndex].FloatingStacks,
			FloatingStack{ID: stackID, WindowIDs: windowIDs, ActiveID: &activeID, Rect: entry.Rect},
		)
		for _, windowID := range windowIDs {
			window := snapshot.Windows[windowID]
			window.Placement = WindowPlacementStacked
			window.FloatingRect = entry.Rect
			snapshot.Windows[windowID] = window
		}
		r.changes.stackGrouped(stackID)
	} else {
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		for _, reopened := range reopen {
			window := snapshot.Windows[reopened.ID]
			window.Placement = WindowPlacementFloating
			window.FloatingRect = entry.Rect
			snapshot.Windows[reopened.ID] = window
			snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, reopened.ID)
		}
	}
	r.changes.desktop(desktopID)
	return true
}

func (r *reducer) reopenDesktop(snapshot *Snapshot, requested DesktopID) DesktopID {
	if _, exists := desktopIndexByID(snapshot, requested); exists {
		return requested
	}
	if r.activeDesktop != nil {
		if _, exists := desktopIndexByID(snapshot, *r.activeDesktop); exists {
			return *r.activeDesktop
		}
	}
	for _, desktop := range snapshot.Desktops {
		if desktop.Purpose == DesktopPurposeStandard {
			return desktop.ID
		}
	}
	return snapshot.Desktops[0].ID
}

func (r *reducer) rejoinClosedWindows(
	snapshot *Snapshot,
	location stackLocation,
	windows []Window,
	activeID *WindowID,
) {
	members := append([]WindowID(nil), location.members()...)
	for _, reopened := range windows {
		window := snapshot.Windows[reopened.ID]
		window.DesktopID = snapshot.Desktops[location.desktopIndex].ID
		window.Placement = WindowPlacementStacked
		snapshot.Windows[reopened.ID] = window
		members = append(members, reopened.ID)
	}
	members = collatePinned(members, snapshot.Windows)
	location.setMembers(members)
	if activeID != nil && containsWindowID(members, *activeID) {
		location.setActive(*activeID)
	} else if location.activeID() == nil || !containsWindowID(members, *location.activeID()) {
		location.setActive(members[0])
	}
	r.changes.node(location.id())
}
