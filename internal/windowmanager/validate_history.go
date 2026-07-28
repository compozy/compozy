package windowmanager

import "fmt"

func (v *snapshotValidator) validateHistory() {
	v.validateHistoryEntries("undo", v.snapshot.History.Undo)
	v.validateHistoryEntries("redo", v.snapshot.History.Redo)
}

func (v *snapshotValidator) validateHistoryEntries(stack string, entries []HistoryEntry) {
	for index, entry := range entries {
		prefix := fmt.Sprintf("history.%s[%d]", stack, index)
		v.validateHistoryState(prefix+".before", entry.Before)
		v.validateHistoryState(prefix+".after", entry.After)
	}
}

func (v *snapshotValidator) validateHistoryState(path string, state State) {
	stateValidator := newSnapshotValidator(Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: v.snapshot.WorkspaceID,
		Desktops:    state.Desktops,
		Windows:     state.Windows,
		Overrides:   state.Overrides,
	})
	stateValidator.validate()
	for _, diagnostic := range stateValidator.diagnostics {
		v.add(diagnostic.Code, path+"."+diagnostic.Path, diagnostic.Message)
	}
}
