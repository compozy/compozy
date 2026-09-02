package windowmanager

import "fmt"

func (r *reducer) deleteDesktop(snapshot *Snapshot, command DeleteDesktopCommand) (bool, error) {
	index, exists := desktopIndexByID(snapshot, command.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
	}
	if len(snapshot.Desktops) == 1 {
		return false, ErrFinalDesktop
	}
	source := snapshot.Desktops[index]
	if !desktopEmpty(source) && command.DestinationID == nil {
		return false, ErrDestinationRequired
	}
	if command.DestinationID != nil && *command.DestinationID == command.DesktopID {
		return false, fmt.Errorf("destination must differ from source: %w", ErrDestinationRequired)
	}
	if command.DestinationID != nil {
		if err := r.transferDesktopContents(snapshot, source, *command.DestinationID); err != nil {
			return false, err
		}
	}
	snapshot.Desktops = append(snapshot.Desktops[:index], snapshot.Desktops[index+1:]...)
	setDesktopOrders(snapshot)
	r.changes.desktop(source.ID)
	return true, nil
}

// transferDesktopContents moves every island, floating frame, and floating
// window of the source onto the destination; arriving windows drop their zoom
// so the destination keeps whatever it was already showing.
func (r *reducer) transferDesktopContents(snapshot *Snapshot, source Desktop, destinationID DesktopID) error {
	destinationIndex, found := desktopIndexByID(snapshot, destinationID)
	if !found {
		return fmt.Errorf("desktop %q: %w", destinationID, ErrDesktopNotFound)
	}
	destination := &snapshot.Desktops[destinationIndex]
	transferred := cloneDesktop(source)
	destination.Groups = append(destination.Groups, transferred.Groups...)
	for _, group := range transferred.Groups {
		r.changes.group(group.ID)
	}
	if layoutGroupsOverlap(destination.Groups) {
		reflowLayoutGroupFrames(destination.Groups)
		for _, group := range destination.Groups {
			r.changes.group(group.ID)
		}
	}
	destination.Floating = append(destination.Floating, transferred.Floating...)
	destination.FloatingStacks = append(destination.FloatingStacks, transferred.FloatingStacks...)
	for _, stack := range transferred.FloatingStacks {
		r.changes.node(stack.ID)
	}
	for windowID, window := range snapshot.Windows {
		if window.DesktopID != source.ID {
			continue
		}
		window.DesktopID = destination.ID
		window.Zoomed = false
		window.ReturnAnchor = nil
		snapshot.Windows[windowID] = window
		r.changes.window(windowID)
	}
	r.changes.desktop(destination.ID)
	return nil
}
