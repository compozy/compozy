package windowmanager

import "fmt"

func (r *reducer) reorderDesktop(snapshot *Snapshot, command ReorderDesktopCommand) (bool, error) {
	index, exists := desktopIndexByID(snapshot, command.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
	}
	if command.Order < 0 || command.Order >= len(snapshot.Desktops) {
		return false, fmt.Errorf("desktop order %d: %w", command.Order, ErrInvalidCommand)
	}
	if index == command.Order {
		return false, nil
	}
	desktop := snapshot.Desktops[index]
	snapshot.Desktops = append(snapshot.Desktops[:index], snapshot.Desktops[index+1:]...)
	snapshot.Desktops = append(snapshot.Desktops, Desktop{})
	copy(snapshot.Desktops[command.Order+1:], snapshot.Desktops[command.Order:])
	snapshot.Desktops[command.Order] = desktop
	setDesktopOrders(snapshot)
	r.changes.desktop(command.DesktopID)
	return true, nil
}
