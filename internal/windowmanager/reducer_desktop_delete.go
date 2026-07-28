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
	nonEmpty := len(source.Groups) > 0 || len(source.Floating) > 0
	if nonEmpty && command.DestinationID == nil {
		return false, ErrDestinationRequired
	}
	if command.DestinationID != nil && *command.DestinationID == command.DesktopID {
		return false, fmt.Errorf("destination must differ from source: %w", ErrDestinationRequired)
	}
	if command.DestinationID != nil {
		destinationIndex, found := desktopIndexByID(snapshot, *command.DestinationID)
		if !found {
			return false, fmt.Errorf("desktop %q: %w", *command.DestinationID, ErrDesktopNotFound)
		}
		destination := &snapshot.Desktops[destinationIndex]
		transferredGroups := cloneDesktop(source).Groups
		destination.Groups = append(destination.Groups, transferredGroups...)
		for _, group := range transferredGroups {
			r.changes.group(group.ID)
		}
		if layoutGroupsOverlap(destination.Groups) {
			reflowLayoutGroupFrames(destination.Groups)
			for _, group := range destination.Groups {
				r.changes.group(group.ID)
			}
		}
		destination.Floating = append(destination.Floating, source.Floating...)
		for windowID, window := range snapshot.Windows {
			if window.DesktopID != source.ID {
				continue
			}
			window.DesktopID = destination.ID
			snapshot.Windows[windowID] = window
			r.changes.window(windowID)
		}
		r.changes.desktop(destination.ID)
	}
	snapshot.Desktops = append(snapshot.Desktops[:index], snapshot.Desktops[index+1:]...)
	setDesktopOrders(snapshot)
	r.changes.desktop(source.ID)
	return true, nil
}
