package windowmanager

import "slices"

func sortChangeSet(changes *ChangeSet) {
	slices.Sort(changes.DesktopIDs)
	slices.Sort(changes.WindowIDs)
	slices.Sort(changes.GroupIDs)
	slices.Sort(changes.NodeIDs)
	slices.Sort(changes.ClientIDs)
}
