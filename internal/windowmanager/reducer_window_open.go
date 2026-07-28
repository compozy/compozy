package windowmanager

import (
	"fmt"
	"strings"
)

func (r *reducer) openWindow(snapshot *Snapshot, command OpenWindowCommand) (bool, error) {
	if command.RestoreWindowID != nil {
		return r.restoreWindow(snapshot, *command.RestoreWindowID)
	}
	window, err := r.newOpenWindow(snapshot, command.Window)
	if err != nil {
		return false, err
	}
	windowID := window.ID
	desktopID := window.DesktopID
	snapshot.Windows[windowID] = window
	insertTiled := command.Window.InsertTiled || r.config.NewWindowPolicy == NewWindowInsert
	if insertTiled && r.focusedWindow != nil {
		focused, exists := snapshot.Windows[*r.focusedWindow]
		if exists && focused.DesktopID == desktopID && focused.Placement != WindowPlacementFloating {
			if err := insertRelative(snapshot, focused.ID, windowID, DropAfter, r.generate); err != nil {
				delete(snapshot.Windows, windowID)
				return false, err
			}
			r.changes.window(focused.ID)
		} else {
			insertTiled = false
		}
	} else if insertTiled {
		desktopIndex, exists := desktopIndexByID(snapshot, desktopID)
		if !exists {
			delete(snapshot.Windows, windowID)
			return false, fmt.Errorf("desktop %q: %w", desktopID, ErrDesktopNotFound)
		}
		if len(snapshot.Desktops[desktopIndex].Groups) == 0 {
			leaf, leafErr := newLeaf(windowID, r.generate)
			if leafErr != nil {
				delete(snapshot.Windows, windowID)
				return false, leafErr
			}
			groupID, generateErr := r.generate("group")
			if generateErr != nil {
				delete(snapshot.Windows, windowID)
				return false, fmt.Errorf("generate group ID: %w", generateErr)
			}
			snapshot.Desktops[desktopIndex].Groups = append(
				snapshot.Desktops[desktopIndex].Groups,
				LayoutGroup{ID: GroupID(groupID), Frame: fullRect(), Root: leaf},
			)
			r.changes.group(GroupID(groupID))
			r.changes.node(leaf.ID)
		} else {
			insertTiled = false
		}
	}
	if !insertTiled {
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
	}
	r.changes.window(windowID)
	r.changes.desktop(desktopID)
	return true, nil
}

func (r *reducer) newOpenWindow(snapshot *Snapshot, spec WindowSpec) (Window, error) {
	windowID := spec.ID
	if windowID == "" {
		generated, err := r.generate("window")
		if err != nil {
			return Window{}, fmt.Errorf("generate window ID: %w", err)
		}
		windowID = WindowID(generated)
	}
	if _, exists := snapshot.Windows[windowID]; exists {
		return Window{}, fmt.Errorf("window %q already exists: %w", windowID, ErrInvalidCommand)
	}
	app := strings.TrimSpace(spec.App)
	if app == "" {
		return Window{}, fmt.Errorf("window app is required: %w", ErrInvalidCommand)
	}
	desktopID, err := r.resolveOpenDesktop(snapshot, spec.DesktopID)
	if err != nil {
		return Window{}, err
	}
	route, err := CanonicalRouteIntent(spec.Route)
	if err != nil {
		return Window{}, fmt.Errorf("window route: %w: %w", err, ErrInvalidCommand)
	}
	return Window{
		ID:           windowID,
		App:          app,
		InstanceKey:  clonePointer(spec.InstanceKey),
		Route:        route,
		DesktopID:    desktopID,
		Placement:    WindowPlacementFloating,
		FloatingRect: clampRect(spec.FloatingRect),
	}, nil
}

func (r *reducer) resolveOpenDesktop(snapshot *Snapshot, requested DesktopID) (DesktopID, error) {
	if requested != "" {
		if _, exists := desktopIndexByID(snapshot, requested); !exists {
			return "", fmt.Errorf("desktop %q: %w", requested, ErrDesktopNotFound)
		}
		return requested, nil
	}
	if r.focusedWindow != nil {
		if focused, exists := snapshot.Windows[*r.focusedWindow]; exists {
			return focused.DesktopID, nil
		}
	}
	for _, desktop := range snapshot.Desktops {
		if desktop.Purpose == DesktopPurposeStandard {
			return desktop.ID, nil
		}
	}
	if len(snapshot.Desktops) == 0 {
		return "", ErrFinalDesktop
	}
	return snapshot.Desktops[0].ID, nil
}
