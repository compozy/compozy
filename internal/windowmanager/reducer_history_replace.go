package windowmanager

import "fmt"

func (r *reducer) replace(snapshot *Snapshot, command ReplaceLayoutCommand) (bool, error) {
	document := command.Document
	if document.Version != SnapshotVersion || document.WorkspaceID != snapshot.WorkspaceID {
		return false, fmt.Errorf("layout document identity: %w", ErrInvalidTopology)
	}
	candidate := Snapshot{
		Version:     document.Version,
		WorkspaceID: document.WorkspaceID,
		Revision:    snapshot.Revision,
		Desktops:    cloneDesktops(document.Desktops),
		Windows:     cloneWindows(document.Windows),
		History:     cloneHistory(snapshot.History),
		Overrides:   cloneWorkspaceConfig(document.Overrides),
		UpdatedAt:   snapshot.UpdatedAt,
	}
	for windowID, current := range snapshot.Windows {
		window, exists := candidate.Windows[windowID]
		if !exists {
			continue
		}
		window.Route = cloneRouteIntent(current.Route)
		candidate.Windows[windowID] = window
	}
	if err := ValidateSnapshot(candidate); err != nil {
		return false, err
	}
	equal, err := statesEqual(snapshotState(*snapshot), snapshotState(candidate))
	if err != nil {
		return false, err
	}
	if equal {
		return false, nil
	}
	restoreStatePreservingRoutes(snapshot, snapshotState(candidate))
	r.markState(snapshot)
	return true, nil
}
