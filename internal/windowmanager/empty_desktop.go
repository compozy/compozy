package windowmanager

const newTabApp = "new-tab"

// DefaultSnapshot is the stable empty-workspace topology before the first commit: one desktop
// with no tabs. A New Tab exists only when someone opens one (#1799).
func DefaultSnapshot(id WorkspaceID) Snapshot {
	return Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: id,
		Desktops:    []Desktop{{ID: "desktop-default", Name: defaultDesktopName, Tabs: []Tab{}}},
		Windows:     map[WindowID]Window{},
		History:     History{Undo: []HistoryEntry{}, Redo: []HistoryEntry{}},
	}
}

// RetireEmptyDesktopPlaceholders drops the New Tab windows of every desktop that holds nothing
// else, in the snapshot and in its undo/redo states, without mutating its input. Releases up to
// v0.7.4 seeded such a placeholder into every empty desktop; an empty desktop now has no tabs
// (#1799). A New Tab carries no content, so dropping it is lossless; one beside real tabs stays.
// The daemon runs it once per stored aggregate, before its first commit on this release. Remove it
// in v0.9.0.
func RetireEmptyDesktopPlaceholders(snapshot Snapshot) (Snapshot, bool) {
	retired := cloneSnapshot(snapshot)
	changed := retirePlaceholders(retired.Desktops, retired.Windows)
	for _, entries := range [][]HistoryEntry{retired.History.Undo, retired.History.Redo} {
		for index := range entries {
			for _, state := range []*State{&entries[index].Before, &entries[index].After} {
				changed = retirePlaceholders(state.Desktops, state.Windows) || changed
			}
		}
	}
	if !changed {
		return snapshot, false
	}
	return NormalizeSnapshot(retired), true
}

func retirePlaceholders(desktops []Desktop, windows map[WindowID]Window) bool {
	changed := false
	for i := range desktops {
		d := &desktops[i]
		if len(d.Tabs) == 0 || !desktopEmpty(&Snapshot{Windows: windows}, *d) {
			continue
		}
		for _, tab := range d.Tabs {
			for _, id := range nodeWindowIDs(tab.Root) {
				delete(windows, id)
			}
		}
		d.Tabs, d.ActiveTabID = []Tab{}, ""
		changed = true
	}
	return changed
}
