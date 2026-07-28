package windowmanager

func (r *reducer) redo(snapshot *Snapshot) (bool, error) {
	if len(snapshot.History.Redo) == 0 {
		return false, ErrHistoryBoundary
	}
	index := len(snapshot.History.Redo) - 1
	entry := cloneHistoryEntry(snapshot.History.Redo[index])
	snapshot.History.Redo = snapshot.History.Redo[:index]
	restoreStatePreservingRoutes(snapshot, entry.After)
	snapshot.History.Undo = append(snapshot.History.Undo, entry)
	r.markState(snapshot)
	return true, nil
}
