package windowmanager

import (
	"fmt"
	"strings"
)

func (r *reducer) updateDesktop(snapshot *Snapshot, command UpdateDesktopCommand) (bool, error) {
	index, exists := desktopIndexByID(snapshot, command.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
	}
	name := strings.TrimSpace(command.Name)
	if name == "" {
		return false, fmt.Errorf("desktop name is required: %w", ErrInvalidCommand)
	}
	if snapshot.Desktops[index].Name == name {
		return false, nil
	}
	snapshot.Desktops[index].Name = name
	r.changes.desktop(command.DesktopID)
	return true, nil
}
