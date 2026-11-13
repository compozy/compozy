package windowmanager

import "fmt"

func (r *reducer) deleteDesktop(snapshot *Snapshot, command DeleteDesktopCommand) (bool, error) {
	index, exists := desktopIndexByID(snapshot, command.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
	}
	if len(snapshot.Desktops) == 1 {
		return false, ErrFinalDesktop
	}
	source := snapshot.Desktops[index]
	if !desktopEmpty(snapshot, source) && command.DestinationID == nil {
		return false, ErrDestinationRequired
	}
	if command.DestinationID != nil && *command.DestinationID == command.DesktopID {
		return false, fmt.Errorf("destination must differ from source: %w", ErrDestinationRequired)
	}
	retained := source.Tabs[:0]
	for _, tab := range source.Tabs {
		if !placeholderTab(snapshot, tab) {
			retained = append(retained, tab)
			continue
		}
		id := nodeWindowIDs(tab.Root)[0]
		delete(snapshot.Windows, id)
		r.changes.window(id)
		r.changes.tab(tab.ID)
	}
	source.Tabs = retained
	if command.DestinationID != nil {
		if err := r.transferDesktopContents(snapshot, source, *command.DestinationID); err != nil {
			return false, err
		}
	}
	snapshot.Desktops = append(snapshot.Desktops[:index], snapshot.Desktops[index+1:]...)
	setDesktopOrders(snapshot)
	r.changes.desktop(source.ID)
	return true, nil
}

func (r *reducer) transferDesktopContents(snapshot *Snapshot, source Desktop, destinationID DesktopID) error {
	index, ok := desktopIndexByID(snapshot, destinationID)
	if !ok {
		return ErrDesktopNotFound
	}
	d := &snapshot.Desktops[index]
	for _, tab := range source.Tabs {
		insertTab(d, tab, len(d.Tabs))
		assignTabWindows(snapshot, d, tab)
		r.changes.tab(tab.ID)
		for _, id := range nodeWindowIDs(tab.Root) {
			r.changes.window(id)
		}
	}
	r.changes.desktop(d.ID)
	return nil
}
