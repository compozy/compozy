package windowmanager

func reservedWindowID(snapshot *Snapshot, windowID WindowID) bool {
	if _, exists := snapshot.Windows[windowID]; exists {
		return true
	}
	for _, entry := range snapshot.ClosedEntries {
		for _, window := range entry.Windows {
			if window.ID == windowID {
				return true
			}
		}
	}
	return false
}

func pushClosedEntry(snapshot *Snapshot, entry ClosedEntry, limit int) {
	entry.Windows = cloneClosedEntries([]ClosedEntry{entry})[0].Windows
	entry.ActiveID = clonePointer(entry.ActiveID)
	entry.StackID = clonePointer(entry.StackID)
	snapshot.ClosedEntries = append(snapshot.ClosedEntries, entry)
	if len(snapshot.ClosedEntries) > limit {
		snapshot.ClosedEntries = append(
			[]ClosedEntry(nil),
			snapshot.ClosedEntries[len(snapshot.ClosedEntries)-limit:]...,
		)
	}
}

func closedEntryWindowIDs(entry ClosedEntry) []WindowID {
	windowIDs := make([]WindowID, 0, len(entry.Windows))
	for _, window := range entry.Windows {
		windowIDs = append(windowIDs, window.ID)
	}
	return windowIDs
}
