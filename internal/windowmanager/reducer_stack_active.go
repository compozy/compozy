package windowmanager

import "fmt"

func (r *reducer) setStackActive(snapshot *Snapshot, command SetStackActiveCommand) (bool, error) {
	location, found := findStackByWindow(snapshot, command.WindowID)
	if !found {
		if _, exists := snapshot.Windows[command.WindowID]; !exists {
			return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
		}
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrNotStacked)
	}
	if location.activeID() != nil && *location.activeID() == command.WindowID {
		return false, nil
	}
	location.setActive(command.WindowID)
	r.changes.window(command.WindowID)
	r.changes.node(location.id())
	return true, nil
}
