package windowmanager

import (
	"fmt"
	"slices"
)

func (v *snapshotValidator) validateClosedTopology(entry ClosedEntry, path string) {
	if entry.Member != nil {
		m := entry.Member
		_, _, edge := placementAxis(DropPlacement(m.NeighborEdge))
		if len(entry.Tabs) != 0 || len(entry.Windows) != 1 || m.TabID == "" || m.ParentSplitID == "" ||
			m.ChildIndex < 0 ||
			!finite(m.Weight) ||
			m.Weight <= 0 ||
			m.Weight >= 1 ||
			m.NeighborID == "" ||
			!edge ||
			(len(m.NeighborIDs) > 0 && (len(m.NeighborIDs) < 2 || !slices.Contains(m.NeighborIDs, m.NeighborID))) {
			v.add(
				"topology.closed_member",
				path+".member",
				"closed member requires one window and a valid insertion anchor",
			)
		}
		return
	}
	if len(entry.Tabs) == 0 {
		v.add("topology.closed_tabs", path+".tabs", "closed entry requires tabs or a member")
		return
	}
	candidate := Snapshot{Version: SnapshotVersion, WorkspaceID: v.snapshot.WorkspaceID, Windows: map[WindowID]Window{}}
	desktop := Desktop{ID: entry.DesktopID, ActiveTabID: entry.Tabs[0].ID}
	for _, closed := range entry.Tabs {
		if closed.Index < 0 {
			v.add("topology.closed_index", path, "closed tab index must be nonnegative")
		}
		desktop.Tabs = append(desktop.Tabs, Tab{ID: closed.ID, Pinned: closed.Pinned, Root: closed.Root})
	}
	// Original indices need not be contiguous, and pin order is restored by insertion.
	for i := range desktop.Tabs {
		desktop.Tabs[i].Pinned = false
	}
	candidate.Desktops = []Desktop{desktop}
	for _, window := range entry.Windows {
		if _, duplicate := candidate.Windows[window.ID]; duplicate {
			v.add("topology.closed_window_duplicate", path, "closed window IDs must be unique")
		}
		candidate.Windows[window.ID] = window
	}
	validator := newSnapshotValidator(candidate)
	validator.validate()
	for _, diagnostic := range validator.diagnostics {
		diagnostic.Path = fmt.Sprintf("%s.%s", path, diagnostic.Path)
		v.diagnostics = append(v.diagnostics, diagnostic)
	}
}
