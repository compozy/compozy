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
	if owner, restoring := zoomRestoreOwner(snapshot, currentDesktop, command.WindowID); restoring {
		return r.restoreZoomedWindow(snapshot, owner)
	}
	if location, stacked := findStackByWindow(snapshot, command.WindowID); stacked {
		return r.zoomStackFrame(snapshot, command.WindowID, window.App, currentDesktopIndex, location)
	}
	anchor := captureReturnAnchor(snapshot, command.WindowID)
	if anchor == nil {
		return false, fmt.Errorf("window %q has no placement: %w", command.WindowID, ErrInvalidTopology)
	}
	removeWindow(snapshot, command.WindowID)
	focusIndex, groupID, leaf, err := r.insertFocusDesktop(
		snapshot,
		command.WindowID,
		window.App,
		currentDesktopIndex,
		nil,
	)
	if err != nil {
		return false, err
	}
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

// zoomRestoreOwner reports whether a zoom press on this window unzooms the
// focus desktop, and which member's anchor drives the restore.
func zoomRestoreOwner(snapshot *Snapshot, desktop Desktop, windowID WindowID) (WindowID, bool) {
	if desktop.Purpose != DesktopPurposeFocus || desktop.FocusOwner == nil {
		return "", false
	}
	owner := *desktop.FocusOwner
	if owner == windowID {
		return owner, true
	}
	if _, exists := snapshot.Windows[owner]; !exists {
		return windowID, true
	}
	if location, stacked := findStackByWindow(snapshot, windowID); stacked &&
		containsWindowID(location.members(), owner) {
		return owner, true
	}
	return "", false
}

// zoomStackFrame moves the whole tab frame to a fresh focus desktop as one
// tiled stack; the pressed member owns the return anchor.
func (r *reducer) zoomStackFrame(
	snapshot *Snapshot,
	ownerID WindowID,
	app string,
	currentDesktopIndex int,
	location stackLocation,
) (bool, error) {
	anchor := captureReturnAnchor(snapshot, ownerID)
	if anchor == nil {
		return false, fmt.Errorf("window %q has no placement: %w", ownerID, ErrInvalidTopology)
	}
	if location.floatingStackIndex >= 0 {
		sourceStack := cloneFloatingStack(
			snapshot.Desktops[location.desktopIndex].FloatingStacks[location.floatingStackIndex],
		)
		anchor.SourceStack = &sourceStack
	}
	members := append([]WindowID(nil), location.members()...)
	node, detached := detachStack(snapshot, location)
	if !detached {
		return false, fmt.Errorf("frame for window %q: %w", ownerID, ErrInvalidTopology)
	}
	focusIndex, groupID, stackNode, err := r.insertFocusDesktop(
		snapshot,
		ownerID,
		app,
		currentDesktopIndex,
		&node,
	)
	if err != nil {
		return false, err
	}
	focusDesktopID := snapshot.Desktops[focusIndex].ID
	for _, memberID := range members {
		member := snapshot.Windows[memberID]
		member.DesktopID = focusDesktopID
		member.Placement = WindowPlacementStacked
		member.Minimized = false
		member.ReturnAnchor = nil
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	owner := snapshot.Windows[ownerID]
	owner.ReturnAnchor = anchor
	snapshot.Windows[ownerID] = owner
	r.changes.group(groupID)
	r.changes.node(stackNode.ID)
	r.changes.desktop(anchor.DesktopID)
	return true, nil
}

// insertFocusDesktop creates the focus desktop hosting one full-frame group:
// the provided stack node, or a fresh leaf for a solo window.
func (r *reducer) insertFocusDesktop(
	snapshot *Snapshot,
	ownerID WindowID,
	app string,
	currentDesktopIndex int,
	root *LayoutNode,
) (int, GroupID, LayoutNode, error) {
	desktopIDRaw, err := r.generate("desktop")
	if err != nil {
		return 0, "", LayoutNode{}, fmt.Errorf("generate focus desktop ID: %w", err)
	}
	desktopID := DesktopID(desktopIDRaw)
	owner := ownerID
	focus := Desktop{
		ID:         desktopID,
		Name:       "Focus — " + app,
		Purpose:    DesktopPurposeFocus,
		FocusOwner: &owner,
		Groups:     []LayoutGroup{},
		Floating:   []WindowID{},
	}
	focusIndex := min(currentDesktopIndex+1, len(snapshot.Desktops))
	snapshot.Desktops = slices.Insert(snapshot.Desktops, focusIndex, focus)
	r.changes.desktop(desktopID)
	var content LayoutNode
	if root != nil {
		content = *root
	} else {
		leaf, leafErr := newLeaf(ownerID, r.generate)
		if leafErr != nil {
			return 0, "", LayoutNode{}, leafErr
		}
		content = leaf
	}
	groupIDRaw, err := r.generate("group")
	if err != nil {
		return 0, "", LayoutNode{}, fmt.Errorf("generate focus group ID: %w", err)
	}
	groupID := GroupID(groupIDRaw)
	snapshot.Desktops[focusIndex].Groups = []LayoutGroup{{ID: groupID, Frame: fullRect(), Root: content}}
	snapshot.Desktops[focusIndex].Floating = nil
	return focusIndex, groupID, content, nil
}

func (r *reducer) restoreZoomedWindow(snapshot *Snapshot, windowID WindowID) (bool, error) {
	if location, stacked := findStackByWindow(snapshot, windowID); stacked {
		return r.restoreZoomedStackFrame(snapshot, windowID, location)
	}
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

// restoreZoomedStackFrame returns the zoomed frame to its source as one unit:
// exact tiled restore, the original floating stack slot, or a graduated
// fallback group when the source drifted away.
func (r *reducer) restoreZoomedStackFrame(
	snapshot *Snapshot,
	ownerID WindowID,
	location stackLocation,
) (bool, error) {
	owner := snapshot.Windows[ownerID]
	anchor := cloneReturnAnchor(owner.ReturnAnchor)
	members := append([]WindowID(nil), location.members()...)
	activeID := clonePointer(location.activeID())
	focusDesktopID := snapshot.Desktops[location.desktopIndex].ID
	node, detached := detachStack(snapshot, location)
	if !detached {
		return false, fmt.Errorf("frame for window %q: %w", ownerID, ErrInvalidTopology)
	}
	destination := focusDesktopID
	if anchor != nil {
		if _, exists := desktopIndexByID(snapshot, anchor.DesktopID); exists {
			destination = anchor.DesktopID
		}
	}
	if err := r.placeRestoredStack(snapshot, node, activeID, anchor, destination); err != nil {
		return false, err
	}
	for _, memberID := range members {
		member := snapshot.Windows[memberID]
		member.DesktopID = destination
		member.Placement = WindowPlacementStacked
		member.ReturnAnchor = nil
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.node(node.ID)
	r.changes.desktop(destination)
	r.changes.desktop(focusDesktopID)
	return true, nil
}

func (r *reducer) placeRestoredStack(
	snapshot *Snapshot,
	node LayoutNode,
	activeID *WindowID,
	anchor *ReturnAnchor,
	destination DesktopID,
) error {
	destinationIndex, exists := desktopIndexByID(snapshot, destination)
	if !exists {
		return fmt.Errorf("desktop %q: %w", destination, ErrDesktopNotFound)
	}
	if anchor != nil && anchor.SourceStack != nil && destination == anchor.DesktopID {
		stack := cloneFloatingStack(*anchor.SourceStack)
		stack.ID = node.ID
		stack.WindowIDs = node.WindowIDs
		stack.ActiveID = activeID
		stack.Minimized = false
		snapshot.Desktops[destinationIndex].FloatingStacks = append(
			snapshot.Desktops[destinationIndex].FloatingStacks,
			stack,
		)
		return nil
	}
	if anchor != nil && destination == anchor.DesktopID &&
		restoreExactSourceStack(snapshot, node, anchor) {
		r.changes.group(anchor.SourceGroup.ID)
		markNodeChanges(anchor.SourceGroup.Root, &r.changes)
		return nil
	}
	groupIDRaw, err := r.generate("group")
	if err != nil {
		return fmt.Errorf("generate restore group ID: %w", err)
	}
	groupID := GroupID(groupIDRaw)
	snapshot.Desktops[destinationIndex].Groups = append(
		snapshot.Desktops[destinationIndex].Groups,
		LayoutGroup{ID: groupID, Frame: fullRect(), Root: node},
	)
	r.changes.group(groupID)
	return nil
}
