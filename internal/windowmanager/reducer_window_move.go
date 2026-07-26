package windowmanager

import "fmt"

func (r *reducer) moveWindow(snapshot *Snapshot, command MoveWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if _, exists := desktopIndexByID(snapshot, command.DestinationDesktopID); !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DestinationDesktopID, ErrDesktopNotFound)
	}
	if command.MoveGroup {
		if command.TargetWindowID != nil || command.Placement != "" || command.FloatingRect != nil {
			return false, fmt.Errorf("group move cannot include placement fields: %w", ErrInvalidCommand)
		}
		return r.moveWindowGroup(snapshot, command)
	}
	if command.TargetWindowID != nil {
		if *command.TargetWindowID == command.WindowID {
			return false, fmt.Errorf("window cannot target itself: %w", ErrInvalidCommand)
		}
		target, exists := snapshot.Windows[*command.TargetWindowID]
		if !exists {
			return false, fmt.Errorf("target window %q: %w", *command.TargetWindowID, ErrWindowNotFound)
		}
		if target.DesktopID != command.DestinationDesktopID {
			return false, fmt.Errorf("target window belongs to another desktop: %w", ErrInvalidCommand)
		}
	}
	anchor := captureReturnAnchor(snapshot, command.WindowID)
	if !removeWindow(snapshot, command.WindowID) {
		return false, fmt.Errorf("window %q has no placement: %w", command.WindowID, ErrInvalidTopology)
	}
	window.DesktopID = command.DestinationDesktopID
	window.Minimized = false
	window.ReturnAnchor = anchor
	if command.FloatingRect != nil {
		window.FloatingRect = clampRect(*command.FloatingRect)
	}
	snapshot.Windows[command.WindowID] = window
	if command.Placement == "" {
		command.Placement = DropFloating
	}
	if command.Placement == DropFloating {
		destinationIndex, _ := desktopIndexByID(snapshot, command.DestinationDesktopID)
		snapshot.Desktops[destinationIndex].Floating = append(
			snapshot.Desktops[destinationIndex].Floating,
			command.WindowID,
		)
	} else {
		if command.TargetWindowID == nil {
			return false, fmt.Errorf("structural move requires target: %w", ErrInvalidCommand)
		}
		if err := insertRelative(
			snapshot,
			*command.TargetWindowID,
			command.WindowID,
			command.Placement,
			r.generate,
		); err != nil {
			return false, err
		}
	}
	r.changes.window(command.WindowID)
	r.changes.desktop(window.DesktopID)
	if anchor != nil {
		r.changes.desktop(anchor.DesktopID)
	}
	return true, nil
}

func (r *reducer) moveWindowGroup(snapshot *Snapshot, command MoveWindowCommand) (bool, error) {
	placement, found := findWindowPlacement(snapshot, command.WindowID)
	if !found || placement.groupIndex < 0 {
		return false, fmt.Errorf("window %q is not grouped: %w", command.WindowID, ErrInvalidCommand)
	}
	sourceDesktop := &snapshot.Desktops[placement.desktopIndex]
	if sourceDesktop.ID == command.DestinationDesktopID {
		return false, nil
	}
	group := sourceDesktop.Groups[placement.groupIndex]
	sourceDesktop.Groups = append(
		sourceDesktop.Groups[:placement.groupIndex],
		sourceDesktop.Groups[placement.groupIndex+1:]...)
	destinationIndex, _ := desktopIndexByID(snapshot, command.DestinationDesktopID)
	snapshot.Desktops[destinationIndex].Groups = append(snapshot.Desktops[destinationIndex].Groups, group)
	for _, memberID := range nodeWindowIDs(group.Root) {
		member := snapshot.Windows[memberID]
		member.DesktopID = command.DestinationDesktopID
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.group(group.ID)
	r.changes.desktop(sourceDesktop.ID)
	r.changes.desktop(command.DestinationDesktopID)
	return true, nil
}
