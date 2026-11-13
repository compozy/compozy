package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/windowmanager"
)

func windowList(raw string) []windowmanager.WindowID {
	ids := []windowmanager.WindowID{}
	for part := range strings.SplitSeq(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			ids = append(ids, windowmanager.WindowID(trimmed))
		}
	}
	return ids
}

// applyLegacyPlacements rewrites single-window tiled groups into the previous
// release's floating, minimized and snapped placements. A snapped pair only
// merges into one split when it is the desktop's sole tiled content, so the
// caller floats or minimizes every other window of that desktop.
func applyLegacyPlacements(snapshot *windowmanager.SnapshotV5, parsed options) error {
	for id := range snapshot.Windows {
		if slices.Contains(parsed.pinned, id) {
			window := snapshot.Windows[id]
			window.Pinned = true
			snapshot.Windows[id] = window
		}
	}
	for _, id := range parsed.floating {
		if err := floatWindow(snapshot, id, parsed.rect, false); err != nil {
			return err
		}
	}
	for _, id := range parsed.minimized {
		if err := floatWindow(snapshot, id, parsed.rect, true); err != nil {
			return err
		}
	}
	for index, id := range parsed.snapped {
		desktop, group, err := soleGroup(snapshot, id)
		if err != nil {
			return err
		}
		snapshot.Desktops[desktop].Groups[group].Frame = windowmanager.NormalizedRect{
			X: 0.5 * float64(index), Width: 0.5, Height: 1,
		}
	}
	return nil
}

func floatWindow(
	snapshot *windowmanager.SnapshotV5,
	id windowmanager.WindowID,
	rect windowmanager.NormalizedRect,
	minimized bool,
) error {
	desktop, group, err := soleGroup(snapshot, id)
	if err != nil {
		return err
	}
	groups := snapshot.Desktops[desktop].Groups
	snapshot.Desktops[desktop].Groups = slices.Delete(groups, group, group+1)
	snapshot.Desktops[desktop].Floating = append(snapshot.Desktops[desktop].Floating, id)
	window := snapshot.Windows[id]
	window.Placement = windowmanager.WindowPlacementFloating
	window.FloatingRect = rect
	window.Minimized = minimized
	snapshot.Windows[id] = window
	return nil
}

// soleGroup finds the group whose whole tree is the one window, as every
// single-window tab converts to.
func soleGroup(snapshot *windowmanager.SnapshotV5, id windowmanager.WindowID) (int, int, error) {
	for desktopIndex, desktop := range snapshot.Desktops {
		for groupIndex, group := range desktop.Groups {
			if group.Root.Kind == windowmanager.NodeKindLeaf && group.Root.WindowID != nil &&
				*group.Root.WindowID == id {
				return desktopIndex, groupIndex, nil
			}
		}
	}
	return 0, 0, fmt.Errorf("window %q is not alone in a tiled group", id)
}
