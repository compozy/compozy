package windowmanager

import "fmt"

// unzoomUnit ends the zoom of the unit the owner holds and returns it to the
// slot the zoom left: the exact source island when it is unchanged, else the
// structural anchor, else beside the client's focused window, else floating.
// A unit that recorded no slot only drops its flag.
func (r *reducer) unzoomUnit(snapshot *Snapshot, ownerID WindowID) error {
	owner := snapshot.Windows[ownerID]
	anchor := cloneReturnAnchor(owner.ReturnAnchor)
	if anchor == nil {
		r.endZoom(snapshot, zoomUnitMembers(snapshot, ownerID))
		r.changes.desktop(owner.DesktopID)
		return nil
	}
	destinationID := owner.DesktopID
	if _, exists := desktopIndexByID(snapshot, anchor.DesktopID); exists {
		destinationID = anchor.DesktopID
	}
	location, stacked := findStackByWindow(snapshot, ownerID)
	if stacked {
		if err := r.returnZoomedStack(snapshot, location, ownerID, anchor, destinationID); err != nil {
			return err
		}
	} else if err := r.returnZoomedWindow(snapshot, ownerID, anchor, destinationID); err != nil {
		return err
	}
	r.changes.desktop(owner.DesktopID)
	r.changes.desktop(destinationID)
	return nil
}

func (r *reducer) returnZoomedWindow(
	snapshot *Snapshot,
	windowID WindowID,
	anchor *ReturnAnchor,
	destinationID DesktopID,
) error {
	if !removeWindow(snapshot, windowID) {
		return fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
	}
	window := snapshot.Windows[windowID]
	window.DesktopID = destinationID
	window.Zoomed = false
	window.ReturnAnchor = nil
	snapshot.Windows[windowID] = window
	restored, err := r.restoreSoloSlot(snapshot, windowID, anchor)
	if err != nil {
		return err
	}
	if restored {
		r.changes.group(anchor.SourceGroup.ID)
		markNodeChanges(anchor.SourceGroup.Root, &r.changes)
	} else if err := insertAtAnchor(snapshot, windowID, anchor, r.focusedWindow, r.generate); err != nil {
		return err
	}
	r.changes.window(windowID)
	return nil
}

// restoreSoloSlot puts a window back into its unchanged source island: verbatim
// when its slot was a leaf, or as a fresh leaf in the slot of the frame it was
// the last tab of.
func (r *reducer) restoreSoloSlot(snapshot *Snapshot, windowID WindowID, anchor *ReturnAnchor) (bool, error) {
	if restoreExactSourceGroup(snapshot, windowID, anchor) {
		return true, nil
	}
	if anchor.SourceGroup == nil {
		return false, nil
	}
	leaf, err := newLeaf(windowID, r.generate)
	if err != nil {
		return false, err
	}
	return restoreExactSourceNode(snapshot, windowID, leaf, anchor), nil
}

// returnZoomedStack carries the whole frame back as one unit so member order
// and the active tab survive the round trip.
func (r *reducer) returnZoomedStack(
	snapshot *Snapshot,
	location stackLocation,
	ownerID WindowID,
	anchor *ReturnAnchor,
	destinationID DesktopID,
) error {
	activeID := clonePointer(location.activeID())
	node, detached := detachStack(snapshot, location)
	if !detached {
		return fmt.Errorf("frame for window %q: %w", ownerID, ErrInvalidTopology)
	}
	if err := r.placeReturnedStack(snapshot, node, activeID, ownerID, anchor, destinationID); err != nil {
		return err
	}
	for _, memberID := range node.WindowIDs {
		member := snapshot.Windows[memberID]
		member.DesktopID = destinationID
		member.Placement = WindowPlacementStacked
		member.Zoomed = false
		member.ReturnAnchor = nil
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.node(node.ID)
	return nil
}

// placeReturnedStack drops the frame at the owner's origin: its floating slot,
// its exact tiled slot, the split it left, beside a surviving neighbor, or
// floating at the owner's rect when the origin is gone.
func (r *reducer) placeReturnedStack(
	snapshot *Snapshot,
	node LayoutNode,
	activeID *WindowID,
	ownerID WindowID,
	anchor *ReturnAnchor,
	destinationID DesktopID,
) error {
	destinationIndex, exists := desktopIndexByID(snapshot, destinationID)
	if !exists {
		return fmt.Errorf("desktop %q: %w", destinationID, ErrDesktopNotFound)
	}
	rect := clampRect(snapshot.Windows[ownerID].FloatingRect)
	if destinationID == anchor.DesktopID {
		if anchor.SourceStack != nil {
			rect = clampRect(anchor.SourceStack.Rect)
		} else {
			if restoreExactSourceNode(snapshot, ownerID, node, anchor) {
				r.changes.group(anchor.SourceGroup.ID)
				return nil
			}
			inserted, err := insertNodeAtStructuralAnchor(snapshot, node, anchor, r.generate)
			if err != nil || inserted {
				return err
			}
		}
	}
	snapshot.Desktops[destinationIndex].FloatingStacks = append(
		snapshot.Desktops[destinationIndex].FloatingStacks,
		FloatingStack{ID: node.ID, WindowIDs: node.WindowIDs, ActiveID: activeID, Rect: rect},
	)
	return nil
}
