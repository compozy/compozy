package windowmanager

import "fmt"

func (r *reducer) toggleFloating(snapshot *Snapshot, command ToggleFloatingCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if window.Placement == WindowPlacementFloating && !window.Minimized {
		removeWindow(snapshot, command.WindowID)
		if err := insertAtAnchor(
			snapshot,
			command.WindowID,
			window.ReturnAnchor,
			r.focusedWindow,
			r.generate,
		); err != nil {
			return false, err
		}
		placement, placed := findWindowPlacement(snapshot, command.WindowID)
		if !placed || placement.placement == WindowPlacementFloating {
			if placed {
				removeWindow(snapshot, command.WindowID)
			}
			if err := r.tileWindowFallback(snapshot, command.WindowID); err != nil {
				return false, err
			}
		}
		window = snapshot.Windows[command.WindowID]
		window.ReturnAnchor = nil
		snapshot.Windows[command.WindowID] = window
	} else {
		anchor := captureReturnAnchor(snapshot, command.WindowID)
		removeWindow(snapshot, command.WindowID)
		window.Placement = WindowPlacementFloating
		window.Minimized = false
		window.ReturnAnchor = anchor
		if command.FloatingRect != nil {
			window.FloatingRect = clampRect(*command.FloatingRect)
		} else {
			window.FloatingRect = clampRect(window.FloatingRect)
		}
		snapshot.Windows[command.WindowID] = window
		desktopIndex, _ := desktopIndexByID(snapshot, window.DesktopID)
		snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, command.WindowID)
	}
	r.changes.window(command.WindowID)
	r.changes.desktop(window.DesktopID)
	return true, nil
}

func (r *reducer) tileWindowFallback(snapshot *Snapshot, windowID WindowID) error {
	window := snapshot.Windows[windowID]
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return fmt.Errorf("desktop %q: %w", window.DesktopID, ErrDesktopNotFound)
	}
	desktop := &snapshot.Desktops[desktopIndex]
	if len(desktop.Groups) > 0 {
		members := nodeWindowIDs(desktop.Groups[0].Root)
		if len(members) > 0 {
			return insertRelative(snapshot, members[0], windowID, DropAfter, r.generate)
		}
	}
	leaf, err := newLeaf(windowID, r.generate)
	if err != nil {
		return err
	}
	groupIDRaw, err := r.generate("group")
	if err != nil {
		return fmt.Errorf("generate group ID: %w", err)
	}
	desktop.Groups = append(desktop.Groups, LayoutGroup{ID: GroupID(groupIDRaw), Frame: fullRect(), Root: leaf})
	r.changes.group(GroupID(groupIDRaw))
	r.changes.node(leaf.ID)
	return nil
}
