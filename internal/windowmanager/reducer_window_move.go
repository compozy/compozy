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
		if command.TargetWindowID != nil {
			return r.moveWindowGroupRelative(snapshot, command)
		}
		if command.Placement != "" && command.Placement != DropFloating {
			return false, fmt.Errorf(
				"group move without a target supports only floating placement: %w",
				ErrInvalidCommand,
			)
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
	if command.Placement == DropCenter && command.TargetWindowID != nil {
		return r.groupWindows(snapshot, GroupWindowsCommand{
			TargetWindowID: *command.TargetWindowID,
			WindowIDs:      []WindowID{command.WindowID},
		})
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

// moveWindowGroupRelative places the whole tab frame relative to a target:
// center folds every member into the target's stack, and axis placements
// splice the frame in as one stack node beside the target's slot.
func (r *reducer) moveWindowGroupRelative(snapshot *Snapshot, command MoveWindowCommand) (bool, error) {
	targetID := *command.TargetWindowID
	if targetID == command.WindowID {
		return false, fmt.Errorf("window cannot target itself: %w", ErrInvalidCommand)
	}
	if command.FloatingRect != nil {
		return false, fmt.Errorf("structural group move cannot carry a floating rect: %w", ErrInvalidCommand)
	}
	target, exists := snapshot.Windows[targetID]
	if !exists {
		return false, fmt.Errorf("target window %q: %w", targetID, ErrWindowNotFound)
	}
	if target.DesktopID != command.DestinationDesktopID {
		return false, fmt.Errorf("target window belongs to another desktop: %w", ErrInvalidCommand)
	}
	location, stacked := findStackByWindow(snapshot, command.WindowID)
	if !stacked {
		command.MoveGroup = false
		return r.moveWindow(snapshot, command)
	}
	if containsWindowID(location.members(), targetID) {
		return false, fmt.Errorf("target window is a member of the moved frame: %w", ErrInvalidCommand)
	}
	if command.Placement == DropCenter {
		return r.groupWindows(snapshot, GroupWindowsCommand{
			TargetWindowID: targetID,
			WindowIDs:      append([]WindowID(nil), location.members()...),
		})
	}
	targetPlacement, found := findWindowPlacement(snapshot, targetID)
	if !found || targetPlacement.groupIndex < 0 {
		return false, fmt.Errorf("target window %q is not tiled: %w", targetID, ErrInvalidCommand)
	}
	members := append([]WindowID(nil), location.members()...)
	sourceDesktopID := snapshot.Desktops[location.desktopIndex].ID
	var sourceGroupID *GroupID
	if location.groupIndex >= 0 {
		groupID := snapshot.Desktops[location.desktopIndex].Groups[location.groupIndex].ID
		sourceGroupID = &groupID
	}
	node, detached := detachStack(snapshot, location)
	if !detached {
		return false, fmt.Errorf("frame for window %q: %w", command.WindowID, ErrInvalidTopology)
	}
	if err := insertRelativeNode(snapshot, targetID, node, command.Placement, r.generate); err != nil {
		return false, err
	}
	for _, memberID := range members {
		member := snapshot.Windows[memberID]
		member.DesktopID = command.DestinationDesktopID
		member.Placement = WindowPlacementStacked
		member.Minimized = false
		member.ReturnAnchor = nil
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.node(node.ID)
	if sourceGroupID != nil {
		r.changes.group(*sourceGroupID)
	}
	r.changes.desktop(sourceDesktopID)
	r.changes.desktop(command.DestinationDesktopID)
	return true, nil
}

// moveFloatingStackFrame moves a floating tab frame as one unit: the deck's
// drag/resize commits one rect for every member, and a cross-desktop move
// carries the whole frame (members follow, order and active tab untouched).
func (r *reducer) moveFloatingStackFrame(
	snapshot *Snapshot,
	command MoveWindowCommand,
	placement windowPlacement,
) (bool, error) {
	source := &snapshot.Desktops[placement.desktopIndex]
	stack := source.FloatingStacks[placement.floatingStackIndex]
	moved := false
	if command.FloatingRect != nil {
		next := clampRect(*command.FloatingRect)
		if next != stack.Rect {
			stack.Rect = next
			moved = true
		}
	}
	if source.ID == command.DestinationDesktopID {
		if !moved {
			return false, nil
		}
		source.FloatingStacks[placement.floatingStackIndex] = stack
	} else {
		source.FloatingStacks = append(
			source.FloatingStacks[:placement.floatingStackIndex],
			source.FloatingStacks[placement.floatingStackIndex+1:]...)
		destinationIndex, _ := desktopIndexByID(snapshot, command.DestinationDesktopID)
		snapshot.Desktops[destinationIndex].FloatingStacks = append(
			snapshot.Desktops[destinationIndex].FloatingStacks,
			stack,
		)
		r.changes.desktop(source.ID)
		moved = true
	}
	for _, memberID := range stack.WindowIDs {
		member := snapshot.Windows[memberID]
		member.DesktopID = command.DestinationDesktopID
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.node(stack.ID)
	r.changes.desktop(command.DestinationDesktopID)
	return moved, nil
}

// floatTiledStackFrame detaches a tiled tab frame and floats it as one unit:
// a deck drag-out keeps member order and the active tab, dropping the whole
// frame at the released rect. Non-stacked tiled members keep rejecting
// placement fields — a whole tiled tree has no floating shape.
func (r *reducer) floatTiledStackFrame(snapshot *Snapshot, command MoveWindowCommand) (bool, error) {
	location, stacked := findStackByWindow(snapshot, command.WindowID)
	if !stacked {
		return false, fmt.Errorf("group move cannot include placement fields: %w", ErrInvalidCommand)
	}
	rect := location.rect(snapshot)
	if command.FloatingRect != nil {
		rect = clampRect(*command.FloatingRect)
	}
	activeID := clonePointer(location.activeID())
	members := append([]WindowID(nil), location.members()...)
	sourceDesktopID := snapshot.Desktops[location.desktopIndex].ID
	sourceGroupID := snapshot.Desktops[location.desktopIndex].Groups[location.groupIndex].ID
	node, detached := detachStack(snapshot, location)
	if !detached {
		return false, fmt.Errorf("frame for window %q: %w", command.WindowID, ErrInvalidTopology)
	}
	destinationIndex, _ := desktopIndexByID(snapshot, command.DestinationDesktopID)
	snapshot.Desktops[destinationIndex].FloatingStacks = append(
		snapshot.Desktops[destinationIndex].FloatingStacks,
		FloatingStack{ID: node.ID, WindowIDs: node.WindowIDs, ActiveID: activeID, Rect: rect},
	)
	for _, memberID := range members {
		member := snapshot.Windows[memberID]
		member.DesktopID = command.DestinationDesktopID
		member.Placement = WindowPlacementStacked
		member.Minimized = false
		member.ReturnAnchor = nil
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.node(node.ID)
	r.changes.group(sourceGroupID)
	r.changes.desktop(sourceDesktopID)
	r.changes.desktop(command.DestinationDesktopID)
	return true, nil
}

func (r *reducer) moveWindowGroup(snapshot *Snapshot, command MoveWindowCommand) (bool, error) {
	placement, found := findWindowPlacement(snapshot, command.WindowID)
	if found && placement.floatingStackIndex >= 0 {
		return r.moveFloatingStackFrame(snapshot, command, placement)
	}
	if !found || placement.groupIndex < 0 {
		return false, fmt.Errorf("window %q is not grouped: %w", command.WindowID, ErrInvalidCommand)
	}
	if command.Placement != "" || command.FloatingRect != nil {
		return r.floatTiledStackFrame(snapshot, command)
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
