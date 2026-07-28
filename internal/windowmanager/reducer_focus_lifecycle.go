package windowmanager

import "slices"

// releaseDetachedFocusDesktops closes the focus lifecycle after every durable
// reduction. A focus desktop only exists while its owner belongs to it.
func (r *reducer) releaseDetachedFocusDesktops(snapshot *Snapshot) {
	detached := make([]DesktopID, 0)
	for _, desktop := range snapshot.Desktops {
		if desktop.Purpose != DesktopPurposeFocus {
			continue
		}
		if desktop.FocusOwner != nil {
			owner, exists := snapshot.Windows[*desktop.FocusOwner]
			if exists && owner.DesktopID == desktop.ID {
				continue
			}
		}
		detached = append(detached, desktop.ID)
	}
	for _, desktopID := range detached {
		r.releaseFocusDesktop(snapshot, desktopID)
	}
	if len(detached) > 0 {
		setDesktopOrders(snapshot)
	}
}

func (r *reducer) releaseFocusDesktop(snapshot *Snapshot, desktopID DesktopID) {
	index, exists := desktopIndexByID(snapshot, desktopID)
	if !exists {
		return
	}
	desktop := snapshot.Desktops[index]
	if desktop.Purpose != DesktopPurposeFocus {
		return
	}
	if len(desktop.Groups) > 0 || len(desktop.Floating) > 0 || len(snapshot.Desktops) == 1 {
		snapshot.Desktops[index].Purpose = DesktopPurposeStandard
		snapshot.Desktops[index].FocusOwner = nil
	} else {
		snapshot.Desktops = slices.Delete(snapshot.Desktops, index, index+1)
	}
	r.changes.desktop(desktop.ID)
}
