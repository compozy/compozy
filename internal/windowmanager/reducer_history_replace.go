package windowmanager

func (r *reducer) replace(snapshot *Snapshot, command ReplaceLayoutCommand) (bool, error) {
	document := command.Document
	diagnostics := make([]Diagnostic, 0, 2)
	if document.Version != SnapshotVersion {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    topologyUnsupportedVersionCode,
			Path:    "version",
			Message: "document version must be 3",
		})
	}
	if document.WorkspaceID != snapshot.WorkspaceID {
		diagnostics = append(diagnostics, Diagnostic{
			Code:    topologyWorkspaceMismatchCode,
			Path:    "workspace_id",
			Message: "layout workspace does not match request",
		})
	}
	if len(diagnostics) > 0 {
		return false, &TopologyError{Diagnostics: diagnostics}
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
	r.releaseDetachedFocusDesktops(&candidate)
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
