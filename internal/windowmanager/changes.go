package windowmanager

import "slices"

func sortChangeSet(changes *ChangeSet) {
	slices.Sort(changes.TabIDs)
	slices.Sort(changes.TabSplit)
	slices.Sort(changes.TabUnsplit)
	slices.Sort(changes.DesktopIDs)
	slices.Sort(changes.WindowIDs)
	slices.Sort(changes.GroupIDs)
	slices.Sort(changes.NodeIDs)
	slices.Sort(changes.ClientIDs)
	slices.Sort(changes.StackGrouped)
	slices.Sort(changes.StackUngrouped)
}

func tabMemberCounts(snapshot *Snapshot) map[TabID]int {
	counts := make(map[TabID]int)
	for _, desktop := range snapshot.Desktops {
		for _, tab := range desktop.Tabs {
			counts[tab.ID] = len(nodeWindowIDs(tab.Root))
		}
	}
	return counts
}

func (r *reducer) recordTabTransitions(before, after map[TabID]int) {
	r.changes.split, r.changes.unsplit = nil, nil
	for id, count := range before {
		if count > 1 && after[id] <= 1 {
			r.changes.tabUnsplit(id)
		}
	}
	for id, count := range after {
		if count > 1 && before[id] <= 1 {
			r.changes.tabSplit(id)
		}
	}
}
