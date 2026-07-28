package windowmanager

func (r *reducer) undo(snapshot *Snapshot) (bool, error) {
	if len(snapshot.History.Undo) == 0 {
		return false, ErrHistoryBoundary
	}
	index := len(snapshot.History.Undo) - 1
	entry := cloneHistoryEntry(snapshot.History.Undo[index])
	snapshot.History.Undo = snapshot.History.Undo[:index]
	restoreStatePreservingRoutes(snapshot, entry.Before)
	snapshot.History.Redo = append(snapshot.History.Redo, entry)
	r.markState(snapshot)
	return true, nil
}
