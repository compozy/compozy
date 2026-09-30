package windowmanager

import (
	"fmt"
	"math"
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
	if command.Window.StackTargetWindowID != nil {
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
		changed, groupErr := r.groupWindows(snapshot, GroupWindowsCommand{
			TargetWindowID: *command.Window.StackTargetWindowID,
			WindowIDs:      []WindowID{windowID},
		})
		if groupErr != nil {
			return false, groupErr
		}
		return changed, nil
	}
	insertTiled := command.Window.InsertTiled || r.config.NewWindowPolicy == NewWindowInsert
	if insertTiled {
		tiled, err := r.placeTiledOpen(snapshot, window, command.Window.BesideWindowID)
		if err != nil {
			delete(snapshot.Windows, windowID)
			return false, err
		}
		insertTiled = tiled
	}
	if !insertTiled {
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		window.FloatingRect = cascadeOpenRect(snapshot.Desktops[desktopIndex], snapshot.Windows, window.FloatingRect)
		snapshot.Windows[windowID] = window
		snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
	}
	r.revealPlacedWindow(snapshot, windowID)
	r.changes.window(windowID)
	r.changes.desktop(desktopID)
	return true, nil
}

// placeTiledOpen tiles a new window beside its anchor: the spec's explicit
// BesideWindowID, else the client's focused window. An anchor that floats keeps
// the new window floating; an anchor that is missing or on another desktop is
// no anchor at all, so an empty desktop still gets one full-frame pane instead
// of a stale focus pointer turning the open into a floating window. With no
// anchor on a desktop that already has tiles, the window floats. It reports
// whether the window was tiled.
func (r *reducer) placeTiledOpen(snapshot *Snapshot, window Window, besideID *WindowID) (bool, error) {
	anchorID := besideID
	if anchorID == nil {
		anchorID = r.focusedWindow
	}
	if anchorID != nil {
		anchor, exists := snapshot.Windows[*anchorID]
		if exists && anchor.DesktopID == window.DesktopID {
			if anchor.Placement == WindowPlacementFloating {
				return false, nil
			}
			if err := insertRelative(snapshot, anchor.ID, window.ID, DropAfter, r.generate); err != nil {
				return false, err
			}
			r.changes.window(anchor.ID)
			r.revealPlacedWindow(snapshot, window.ID)
			return true, nil
		}
	}
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", window.DesktopID, ErrDesktopNotFound)
	}
	if len(snapshot.Desktops[desktopIndex].Groups) > 0 {
		return false, nil
	}
	leaf, err := newLeaf(window.ID, r.generate)
	if err != nil {
		return false, err
	}
	groupID, err := r.generate("group")
	if err != nil {
		return false, fmt.Errorf("generate group ID: %w", err)
	}
	snapshot.Desktops[desktopIndex].Groups = append(
		snapshot.Desktops[desktopIndex].Groups,
		LayoutGroup{ID: GroupID(groupID), Frame: fullRect(), Root: leaf},
	)
	r.changes.group(GroupID(groupID))
	r.changes.node(leaf.ID)
	return true, nil
}

// cascadeOpenRect steps a new floating window down-right while its rect would
// sit exactly on a visible floating window of the same desktop, so a second
// open never hides the first one. The walk stops once the rect is free or can
// no longer move inside the desktop.
func cascadeOpenRect(desktop Desktop, windows map[WindowID]Window, rect NormalizedRect) NormalizedRect {
	occupied := func(candidate NormalizedRect) bool {
		for _, id := range desktop.Floating {
			other, exists := windows[id]
			if exists && !other.Minimized && sameRect(other.FloatingRect, candidate) {
				return true
			}
		}
		return false
	}
	for range len(desktop.Floating) {
		if !occupied(rect) {
			return rect
		}
		next := cascadeRect(rect)
		if sameRect(next, rect) {
			return rect
		}
		rect = next
	}
	return rect
}

func sameRect(left, right NormalizedRect) bool {
	const epsilon = 1e-9
	return math.Abs(left.X-right.X) < epsilon && math.Abs(left.Y-right.Y) < epsilon &&
		math.Abs(left.Width-right.Width) < epsilon && math.Abs(left.Height-right.Height) < epsilon
}

func (r *reducer) newOpenWindow(snapshot *Snapshot, spec WindowSpec) (Window, error) {
	windowID := spec.ID
	if windowID == "" {
		for range 128 {
			generated, err := r.generate("window")
			if err != nil {
				return Window{}, fmt.Errorf("generate window ID: %w", err)
			}
			candidate := WindowID(generated)
			if !reservedWindowID(snapshot, candidate) {
				windowID = candidate
				break
			}
		}
		if windowID == "" {
			return Window{}, fmt.Errorf("generate unique window ID: %w", ErrInvalidCommand)
		}
	}
	if reservedWindowID(snapshot, windowID) {
		return Window{}, fmt.Errorf("window %q already exists: %w", windowID, ErrInvalidCommand)
	}
	if spec.StackTargetWindowID != nil {
		target, exists := snapshot.Windows[*spec.StackTargetWindowID]
		if !exists {
			return Window{}, fmt.Errorf("stack target window %q: %w", *spec.StackTargetWindowID, ErrWindowNotFound)
		}
		if spec.DesktopID != "" && spec.DesktopID != target.DesktopID {
			return Window{}, fmt.Errorf("stack target belongs to another desktop: %w", ErrInvalidCommand)
		}
		spec.DesktopID = target.DesktopID
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
	if len(snapshot.Desktops) == 0 {
		return "", ErrFinalDesktop
	}
	return snapshot.Desktops[0].ID, nil
}
