package windowmanager

import (
	"fmt"
	"slices"
)

func (r *reducer) zoomWindow(snapshot *Snapshot, command ZoomWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	currentDesktopIndex, found := desktopIndexByID(snapshot, window.DesktopID)
	if !found {
		return false, fmt.Errorf("desktop %q: %w", window.DesktopID, ErrDesktopNotFound)
	}
	currentDesktop := snapshot.Desktops[currentDesktopIndex]
	if currentDesktop.Purpose == DesktopPurposeFocus && currentDesktop.FocusOwner != nil &&
		*currentDesktop.FocusOwner == command.WindowID {
		return r.restoreZoomedWindow(snapshot, command.WindowID)
	}
	anchor := captureReturnAnchor(snapshot, command.WindowID)
	if anchor == nil {
		return false, fmt.Errorf("window %q has no placement: %w", command.WindowID, ErrInvalidTopology)
	}
	removeWindow(snapshot, command.WindowID)
	desktopIDRaw, err := r.generate("desktop")
	if err != nil {
		return false, fmt.Errorf("generate focus desktop ID: %w", err)
	}
	desktopID := DesktopID(desktopIDRaw)
	owner := command.WindowID
	focus := Desktop{
		ID:         desktopID,
		Name:       "Focus — " + window.App,
		Purpose:    DesktopPurposeFocus,
		FocusOwner: &owner,
		Groups:     []LayoutGroup{},
		Floating:   []WindowID{},
	}
	focusIndex := min(currentDesktopIndex+1, len(snapshot.Desktops))
	snapshot.Desktops = slices.Insert(snapshot.Desktops, focusIndex, focus)
	r.changes.desktop(desktopID)
	leaf, err := newLeaf(command.WindowID, r.generate)
	if err != nil {
		return false, err
	}
	groupIDRaw, err := r.generate("group")
	if err != nil {
		return false, fmt.Errorf("generate focus group ID: %w", err)
	}
	groupID := GroupID(groupIDRaw)
	snapshot.Desktops[focusIndex].Groups = []LayoutGroup{{ID: groupID, Frame: fullRect(), Root: leaf}}
	snapshot.Desktops[focusIndex].Floating = nil
	window.DesktopID = snapshot.Desktops[focusIndex].ID
	window.Placement = WindowPlacementTiled
	window.Minimized = false
	window.ReturnAnchor = anchor
	snapshot.Windows[command.WindowID] = window
	r.changes.window(command.WindowID)
	r.changes.group(groupID)
	r.changes.node(leaf.ID)
	r.changes.desktop(anchor.DesktopID)
	return true, nil
}

func (r *reducer) restoreZoomedWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	window := snapshot.Windows[windowID]
	anchor := cloneReturnAnchor(window.ReturnAnchor)
	removeWindow(snapshot, windowID)
	restoredExactly := restoreExactSourceGroup(snapshot, windowID, anchor)
	if !restoredExactly {
		if anchor != nil {
			if _, exists := desktopIndexByID(snapshot, anchor.DesktopID); exists {
				window.DesktopID = anchor.DesktopID
			}
		}
		snapshot.Windows[windowID] = window
		if err := insertAtAnchor(snapshot, windowID, anchor, r.focusedWindow, r.generate); err != nil {
			return false, err
		}
	}
	if restoredExactly {
		r.changes.group(anchor.SourceGroup.ID)
		markNodeChanges(anchor.SourceGroup.Root, &r.changes)
	}
	window = snapshot.Windows[windowID]
	window.ReturnAnchor = nil
	snapshot.Windows[windowID] = window
	r.changes.window(windowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}
