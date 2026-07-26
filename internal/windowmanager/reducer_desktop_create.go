package windowmanager

import (
	"fmt"
	"strings"
)

func (r *reducer) createDesktop(snapshot *Snapshot, command CreateDesktopCommand) (bool, error) {
	desktopID := command.DesktopID
	if desktopID == "" {
		generated, err := r.generate("desktop")
		if err != nil {
			return false, fmt.Errorf("generate desktop ID: %w", err)
		}
		desktopID = DesktopID(generated)
	}
	if _, exists := desktopIndexByID(snapshot, desktopID); exists {
		return false, fmt.Errorf("desktop %q already exists: %w", desktopID, ErrInvalidCommand)
	}
	purpose := command.Purpose
	if purpose == "" {
		purpose = DesktopPurposeStandard
	}
	if purpose != DesktopPurposeStandard && purpose != DesktopPurposeFocus {
		return false, fmt.Errorf("desktop purpose %q: %w", purpose, ErrInvalidCommand)
	}
	name := strings.TrimSpace(command.Name)
	if name == "" {
		name = fmt.Sprintf("Desktop %d", len(snapshot.Desktops)+1)
	}
	desktop := Desktop{
		ID:         desktopID,
		Name:       name,
		Order:      len(snapshot.Desktops),
		Purpose:    purpose,
		FocusOwner: clonePointer(command.FocusOwner),
		Groups:     []LayoutGroup{},
		Floating:   []WindowID{},
	}
	insertIndex := len(snapshot.Desktops)
	if command.AfterID != nil {
		index, exists := desktopIndexByID(snapshot, *command.AfterID)
		if !exists {
			return false, fmt.Errorf("desktop %q: %w", *command.AfterID, ErrDesktopNotFound)
		}
		insertIndex = index + 1
	}
	snapshot.Desktops = append(snapshot.Desktops, Desktop{})
	copy(snapshot.Desktops[insertIndex+1:], snapshot.Desktops[insertIndex:])
	snapshot.Desktops[insertIndex] = desktop
	setDesktopOrders(snapshot)
	r.changes.desktop(desktopID)
	return true, nil
}
