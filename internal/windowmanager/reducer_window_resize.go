package windowmanager

import "fmt"

func (r *reducer) resizeWindow(snapshot *Snapshot, command ResizeWindowCommand) (bool, error) {
	if _, exists := snapshot.Windows[command.WindowID]; !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	placement, found := findWindowPlacement(snapshot, command.WindowID)
	if !found {
		return false, fmt.Errorf("window %q has no placement: %w", command.WindowID, ErrInvalidTopology)
	}
	switch {
	case placement.floatingStackIndex >= 0:
		return r.resizeFloatingStackFrame(snapshot, placement, command.Frame)
	case placement.floatingIndex >= 0:
		return r.resizeFloatingWindow(snapshot, command)
	default:
		return r.resizeTiledUnit(snapshot, placement, command)
	}
}

func (r *reducer) resizeFloatingWindow(snapshot *Snapshot, command ResizeWindowCommand) (bool, error) {
	window := snapshot.Windows[command.WindowID]
	next := clampRect(command.Frame)
	if rectsAlmostEqual(window.FloatingRect, next) {
		return false, nil
	}
	window.FloatingRect = next
	snapshot.Windows[command.WindowID] = window
	r.changes.window(command.WindowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

func (r *reducer) resizeFloatingStackFrame(
	snapshot *Snapshot,
	placement windowPlacement,
	frame NormalizedRect,
) (bool, error) {
	desktop := &snapshot.Desktops[placement.desktopIndex]
	stack := desktop.FloatingStacks[placement.floatingStackIndex]
	next := clampRect(frame)
	if rectsAlmostEqual(stack.Rect, next) {
		return false, nil
	}
	stack.Rect = next
	desktop.FloatingStacks[placement.floatingStackIndex] = stack
	for _, memberID := range stack.WindowIDs {
		member := snapshot.Windows[memberID]
		member.FloatingRect = next
		snapshot.Windows[memberID] = member
		r.changes.window(memberID)
	}
	r.changes.node(stack.ID)
	r.changes.desktop(desktop.ID)
	return true, nil
}

func (r *reducer) resizeTiledUnit(
	snapshot *Snapshot,
	placement windowPlacement,
	command ResizeWindowCommand,
) (bool, error) {
	if err := validateGroupFrame(command.Frame); err != nil {
		return false, fmt.Errorf("window %q: %w", command.WindowID, err)
	}
	desktop := &snapshot.Desktops[placement.desktopIndex]
	group := desktop.Groups[placement.groupIndex]
	if group.Root.ID != placement.nodeID {
		return r.detachTiledUnit(snapshot, placement, command)
	}
	if rectsAlmostEqual(group.Frame, command.Frame) {
		return false, nil
	}
	desktop.Groups[placement.groupIndex].Frame = command.Frame
	if layoutGroupsOverlap(desktop.Groups) {
		return false, fmt.Errorf("frame overlaps a neighboring island: %w", ErrInvalidCommand)
	}
	r.changes.group(group.ID)
	r.changes.desktop(desktop.ID)
	return true, nil
}

// detachTiledUnit lifts a split member into its own island at the requested
// frame while every surviving sibling keeps its exact projected zone.
func (r *reducer) detachTiledUnit(
	snapshot *Snapshot,
	placement windowPlacement,
	command ResizeWindowCommand,
) (bool, error) {
	desktop := &snapshot.Desktops[placement.desktopIndex]
	group := desktop.Groups[placement.groupIndex]
	path, found := nodePath(group.Root, placement.nodeID)
	if !found {
		return false, fmt.Errorf("unit %q not in group %q: %w", placement.nodeID, group.ID, ErrInvalidTopology)
	}
	unit := nodeAtPath(group.Root, path)
	remainders, err := detachRemainderGroups(group, path, r.generate)
	if err != nil {
		return false, err
	}
	if len(remainders) == 1 {
		remainders[0].ID = group.ID
	}
	unitGroupID, err := r.generate("group")
	if err != nil {
		return false, fmt.Errorf("generate group ID: %w", err)
	}
	groups := make([]LayoutGroup, 0, len(desktop.Groups)+len(remainders))
	groups = append(groups, desktop.Groups[:placement.groupIndex]...)
	groups = append(groups, remainders...)
	groups = append(groups, desktop.Groups[placement.groupIndex+1:]...)
	groups = append(groups, LayoutGroup{ID: GroupID(unitGroupID), Frame: command.Frame, Root: unit})
	desktop.Groups = groups
	if layoutGroupsOverlap(desktop.Groups) {
		return false, fmt.Errorf("frame overlaps a neighboring island: %w", ErrInvalidCommand)
	}
	for _, memberID := range nodeWindowIDs(unit) {
		r.changes.window(memberID)
	}
	r.changes.group(group.ID)
	for _, remainder := range remainders {
		r.changes.group(remainder.ID)
	}
	r.changes.group(GroupID(unitGroupID))
	r.changes.node(unit.ID)
	r.changes.desktop(desktop.ID)
	return true, nil
}
