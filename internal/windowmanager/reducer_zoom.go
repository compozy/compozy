package windowmanager

import (
	"fmt"
	"slices"
)

// zoomWindow toggles the zoom of the unit holding the window. A zoomed unit is
// the only full-frame island of a desktop: its own when nothing else shows
// there, else a fresh desktop right after it so no window gets covered.
// Unzooming returns the unit to the slot the zoom left.
func (r *reducer) zoomWindow(snapshot *Snapshot, command ZoomWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if window.Minimized {
		if _, err := r.restoreWindow(snapshot, command.WindowID); err != nil {
			return false, err
		}
		if unitZoomed(snapshot, zoomUnitMembers(snapshot, command.WindowID)) {
			return true, nil
		}
		window = snapshot.Windows[command.WindowID]
	}
	if _, found := desktopIndexByID(snapshot, window.DesktopID); !found {
		return false, fmt.Errorf("desktop %q: %w", window.DesktopID, ErrDesktopNotFound)
	}
	members := zoomUnitMembers(snapshot, command.WindowID)
	if ownerID, zoomed := zoomOwner(snapshot, members); zoomed {
		return true, r.unzoomUnit(snapshot, ownerID)
	}
	return true, r.liftZoom(snapshot, command.WindowID)
}

// liftZoom detaches the unit holding the window and zooms it: in place when
// its desktop shows nothing else, otherwise on a fresh desktop.
func (r *reducer) liftZoom(snapshot *Snapshot, ownerID WindowID) error {
	owner := snapshot.Windows[ownerID]
	desktopIndex, exists := desktopIndexByID(snapshot, owner.DesktopID)
	if !exists {
		return fmt.Errorf("desktop %q: %w", owner.DesktopID, ErrDesktopNotFound)
	}
	members := zoomUnitMembers(snapshot, ownerID)
	anchor := captureReturnAnchor(snapshot, ownerID)
	if anchor == nil {
		return fmt.Errorf("window %q has no placement: %w", ownerID, ErrInvalidTopology)
	}
	node, err := r.detachZoomUnit(snapshot, ownerID, anchor)
	if err != nil {
		return err
	}
	destinationID := owner.DesktopID
	if desktopOccupied(snapshot, owner.DesktopID, members) {
		destinationID, err = r.insertDesktopAfter(snapshot, desktopIndex)
		if err != nil {
			return err
		}
	}
	if anchor.GroupID != nil {
		r.changes.group(*anchor.GroupID)
	}
	r.changes.desktop(owner.DesktopID)
	return r.placeZoomedUnit(snapshot, node, ownerID, destinationID, anchor)
}

// detachZoomUnit lifts the unit out of its slot: a tab frame as its stack node,
// a solo window as a fresh leaf. A floating frame's rect rides along on the
// anchor so unzoom can float it back where it was.
func (r *reducer) detachZoomUnit(snapshot *Snapshot, ownerID WindowID, anchor *ReturnAnchor) (LayoutNode, error) {
	location, stacked := findStackByWindow(snapshot, ownerID)
	if !stacked {
		if !removeWindow(snapshot, ownerID) {
			return LayoutNode{}, fmt.Errorf("window %q has no placement: %w", ownerID, ErrInvalidTopology)
		}
		return newLeaf(ownerID, r.generate)
	}
	if location.floatingStackIndex >= 0 {
		stack := cloneFloatingStack(
			snapshot.Desktops[location.desktopIndex].FloatingStacks[location.floatingStackIndex],
		)
		anchor.SourceStack = &stack
	}
	node, detached := detachStack(snapshot, location)
	if !detached {
		return LayoutNode{}, fmt.Errorf("frame for window %q: %w", ownerID, ErrInvalidTopology)
	}
	return node, nil
}

// placeZoomedUnit makes the node the full-frame island of the destination and
// flags the owner zoomed, holding the slot the unit returns to on unzoom.
func (r *reducer) placeZoomedUnit(
	snapshot *Snapshot,
	node LayoutNode,
	ownerID WindowID,
	destinationID DesktopID,
	anchor *ReturnAnchor,
) error {
	destinationIndex, exists := desktopIndexByID(snapshot, destinationID)
	if !exists {
		return fmt.Errorf("desktop %q: %w", destinationID, ErrDesktopNotFound)
	}
	candidate := LayoutGroup{Frame: fullRect(), Root: node}
	groups := append(slices.Clone(snapshot.Desktops[destinationIndex].Groups), candidate)
	if layoutGroupsOverlap(groups) {
		return fmt.Errorf("zoomed unit overlaps a layout group on desktop %q: %w", destinationID, ErrInvalidTopology)
	}
	generated, err := r.generate("group")
	if err != nil {
		return fmt.Errorf("generate group ID: %w", err)
	}
	groupID := GroupID(generated)
	candidate.ID = groupID
	snapshot.Desktops[destinationIndex].Groups = append(snapshot.Desktops[destinationIndex].Groups, candidate)
	placement := WindowPlacementTiled
	if node.Kind == NodeKindStack {
		placement = WindowPlacementStacked
	}
	for _, memberID := range nodeWindowIDs(node) {
		member := snapshot.Windows[memberID]
		member.DesktopID = destinationID
		member.Placement = placement
		member.Minimized = false
		member.Zoomed = memberID == ownerID
		member.ReturnAnchor = nil
		if memberID == ownerID {
			member.ReturnAnchor = anchor
		}
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.group(groupID)
	markNodeChanges(node, &r.changes)
	r.changes.desktop(destinationID)
	return nil
}
