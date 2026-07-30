package windowmanager

import "fmt"

func (r *reducer) pinWindow(snapshot *Snapshot, command PinWindowCommand) (bool, error) {
	window, exists := snapshot.Windows[command.WindowID]
	if !exists {
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
	}
	if window.Pinned == command.Pinned {
		return false, nil
	}
	window.Pinned = command.Pinned
	snapshot.Windows[command.WindowID] = window
	if location, found := findStackByWindow(snapshot, command.WindowID); found {
		location.setMembers(collatePinned(location.members(), snapshot.Windows))
		r.changes.node(location.id())
	}
	r.changes.window(command.WindowID)
	return true, nil
}
