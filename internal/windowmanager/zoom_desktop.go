package windowmanager

import (
	"fmt"
	"slices"
)

func nextDesktopName(snapshot *Snapshot) string {
	return fmt.Sprintf("Desktop %d", len(snapshot.Desktops)+1)
}

func desktopEmpty(desktop Desktop) bool {
	return len(desktop.Groups) == 0 && len(desktop.Floating) == 0 && len(desktop.FloatingStacks) == 0
}

func minimizedZoomReferencesDesktop(snapshot *Snapshot, desktopID DesktopID) bool {
	for _, window := range snapshot.Windows {
		if window.DesktopID == desktopID && window.Minimized &&
			window.ReturnAnchor != nil && window.ReturnAnchor.Zoomed {
			return true
		}
	}
	return false
}

// insertDesktopAfter adds an empty desktop right after the given position.
func (r *reducer) insertDesktopAfter(snapshot *Snapshot, index int) (DesktopID, error) {
	generated, err := r.generate("desktop")
	if err != nil {
		return "", fmt.Errorf("generate desktop ID: %w", err)
	}
	desktopID := DesktopID(generated)
	desktop := Desktop{
		ID:       desktopID,
		Name:     nextDesktopName(snapshot),
		Groups:   []LayoutGroup{},
		Floating: []WindowID{},
	}
	snapshot.Desktops = slices.Insert(snapshot.Desktops, min(index+1, len(snapshot.Desktops)), desktop)
	setDesktopOrders(snapshot)
	r.changes.desktop(desktopID)
	return desktopID, nil
}

// releaseVacatedZoomDesktops drops a desktop that zoom created once the unit it
// hosted has left it empty. A desktop the user filled meanwhile is a regular
// desktop and stays; the last desktop always stays.
func (r *reducer) releaseVacatedZoomDesktops(snapshot *Snapshot, lifted map[DesktopID]struct{}) {
	if len(lifted) == 0 {
		return
	}
	remaining := make([]Desktop, 0, len(snapshot.Desktops))
	for _, desktop := range snapshot.Desktops {
		if _, wasLifted := lifted[desktop.ID]; wasLifted && desktopEmpty(desktop) &&
			!minimizedZoomReferencesDesktop(snapshot, desktop.ID) {
			continue
		}
		remaining = append(remaining, desktop)
	}
	if len(remaining) == len(snapshot.Desktops) {
		return
	}
	if len(remaining) == 0 {
		remaining = snapshot.Desktops[:1]
	}
	for _, desktop := range snapshot.Desktops {
		if !slices.ContainsFunc(remaining, func(kept Desktop) bool { return kept.ID == desktop.ID }) {
			r.changes.desktop(desktop.ID)
		}
	}
	snapshot.Desktops = remaining
	setDesktopOrders(snapshot)
}
