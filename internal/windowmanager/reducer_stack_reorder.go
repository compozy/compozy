package windowmanager

import "fmt"

func (r *reducer) reorderStack(snapshot *Snapshot, command ReorderStackCommand) (bool, error) {
	location, found := findStackByWindow(snapshot, command.WindowID)
	if !found {
		if _, exists := snapshot.Windows[command.WindowID]; !exists {
			return false, fmt.Errorf("window %q: %w", command.WindowID, ErrWindowNotFound)
		}
		return false, fmt.Errorf("window %q: %w", command.WindowID, ErrNotStacked)
	}
	members := append([]WindowID(nil), location.members()...)
	current := -1
	for index, windowID := range members {
		if windowID == command.WindowID {
			current = index
			members = append(members[:index], members[index+1:]...)
			break
		}
	}
	pinnedCount := pinnedPrefixLength(members, snapshot.Windows)
	index := command.Index
	if snapshot.Windows[command.WindowID].Pinned {
		index = min(max(index, 0), pinnedCount)
	} else {
		index = min(max(index, pinnedCount), len(members))
	}
	if current == index {
		return false, nil
	}
	members = spliceWindowIDs(members, index, []WindowID{command.WindowID})
	location.setMembers(members)
	r.changes.window(command.WindowID)
	r.changes.node(location.id())
	return true, nil
}
