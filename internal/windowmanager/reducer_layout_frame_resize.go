package windowmanager

import (
	"fmt"
	"math"
)

const minGroupFrameExtent = 0.01

func (r *reducer) frameResize(snapshot *Snapshot, command FrameResizeLayoutCommand) (bool, error) {
	desktopIndex, exists := desktopIndexByID(snapshot, command.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
	}
	if len(command.Edits) == 0 {
		return false, fmt.Errorf("frame edits are required: %w", ErrInvalidCommand)
	}
	desktop := &snapshot.Desktops[desktopIndex]
	seen := make(map[GroupID]struct{}, len(command.Edits))
	changed := false
	for _, edit := range command.Edits {
		if _, duplicate := seen[edit.GroupID]; duplicate {
			return false, fmt.Errorf("group %q is duplicated: %w", edit.GroupID, ErrInvalidCommand)
		}
		seen[edit.GroupID] = struct{}{}
		if err := validateGroupFrame(edit.Frame); err != nil {
			return false, fmt.Errorf("group %q: %w", edit.GroupID, err)
		}
		groupIndex, found := groupIndexByID(desktop, edit.GroupID)
		if !found {
			return false, fmt.Errorf(
				"group %q not in desktop %q: %w",
				edit.GroupID,
				command.DesktopID,
				ErrInvalidCommand,
			)
		}
		if rectsAlmostEqual(desktop.Groups[groupIndex].Frame, edit.Frame) {
			continue
		}
		desktop.Groups[groupIndex].Frame = edit.Frame
		r.changes.group(edit.GroupID)
		changed = true
	}
	if !changed {
		return false, nil
	}
	if layoutGroupsOverlap(desktop.Groups) {
		return false, fmt.Errorf("group frames must not overlap: %w", ErrInvalidCommand)
	}
	r.changes.desktop(command.DesktopID)
	return true, nil
}

func validateGroupFrame(frame NormalizedRect) error {
	if !validRect(frame) {
		return fmt.Errorf("frame must be normalized: %w", ErrInvalidCommand)
	}
	if frame.Width < minGroupFrameExtent || frame.Height < minGroupFrameExtent {
		return fmt.Errorf("frame is below the minimum extent: %w", ErrInvalidCommand)
	}
	return nil
}

func groupIndexByID(desktop *Desktop, groupID GroupID) (int, bool) {
	for index := range desktop.Groups {
		if desktop.Groups[index].ID == groupID {
			return index, true
		}
	}
	return -1, false
}

func rectsAlmostEqual(left, right NormalizedRect) bool {
	return math.Abs(left.X-right.X) <= weightTolerance &&
		math.Abs(left.Y-right.Y) <= weightTolerance &&
		math.Abs(left.Width-right.Width) <= weightTolerance &&
		math.Abs(left.Height-right.Height) <= weightTolerance
}
