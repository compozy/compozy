package windowmanager

import (
	"fmt"
	"math"
)

func (r *reducer) arrange(snapshot *Snapshot, command ArrangeLayoutCommand) (bool, error) {
	desktopIndex, exists := desktopIndexByID(snapshot, command.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
	}
	if command.ResourceID != "" {
		return false, fmt.Errorf("unresolved layout resource %q: %w", command.ResourceID, ErrInvalidCommand)
	}
	if len(command.WindowIDs) == 0 {
		return false, fmt.Errorf("arrange participants are required: %w", ErrInvalidCommand)
	}
	seen := make(map[WindowID]struct{}, len(command.WindowIDs))
	for _, windowID := range command.WindowIDs {
		window, found := snapshot.Windows[windowID]
		if !found {
			return false, fmt.Errorf("window %q: %w", windowID, ErrWindowNotFound)
		}
		if window.DesktopID != command.DesktopID {
			return false, fmt.Errorf("window %q belongs to another desktop: %w", windowID, ErrInvalidCommand)
		}
		if _, duplicate := seen[windowID]; duplicate {
			return false, fmt.Errorf("window %q is duplicated: %w", windowID, ErrInvalidCommand)
		}
		seen[windowID] = struct{}{}
	}
	for _, windowID := range command.WindowIDs {
		removeWindow(snapshot, windowID)
		window := snapshot.Windows[windowID]
		window.Minimized = false
		window.ReturnAnchor = nil
		snapshot.Windows[windowID] = window
		r.changes.window(windowID)
	}
	root, err := r.buildArrangement(command.WindowIDs, command.Arrangement)
	if err != nil {
		return false, err
	}
	markNodeChanges(root, &r.changes)
	groupID := command.GroupID
	if groupID == "" {
		generated, generateErr := r.generate("group")
		if generateErr != nil {
			return false, fmt.Errorf("generate group ID: %w", generateErr)
		}
		groupID = GroupID(generated)
	}
	frame := command.Frame
	if frame.Width == 0 && frame.Height == 0 {
		frame = fullRect()
	}
	snapshot.Desktops[desktopIndex].Groups = append(
		snapshot.Desktops[desktopIndex].Groups,
		LayoutGroup{ID: groupID, Frame: frame, Root: root},
	)
	r.changes.desktop(command.DesktopID)
	r.changes.group(groupID)
	return true, nil
}

func (r *reducer) buildArrangement(windowIDs []WindowID, arrangement Arrangement) (LayoutNode, error) {
	if len(windowIDs) == 1 {
		return newLeaf(windowIDs[0], r.generate)
	}
	switch arrangement {
	case ArrangementHorizontal:
		return r.buildSplit(windowIDs, AxisHorizontal)
	case ArrangementVertical:
		return r.buildSplit(windowIDs, AxisVertical)
	case ArrangementStack:
		id, err := r.generate("node")
		if err != nil {
			return LayoutNode{}, fmt.Errorf("generate stack ID: %w", err)
		}
		active := windowIDs[0]
		return LayoutNode{
			ID:        NodeID(id),
			Kind:      NodeKindStack,
			WindowIDs: append([]WindowID(nil), windowIDs...),
			ActiveID:  &active,
		}, nil
	case ArrangementGrid:
		columns := int(math.Ceil(math.Sqrt(float64(len(windowIDs)))))
		rows := make([]LayoutNode, 0, (len(windowIDs)+columns-1)/columns)
		for start := 0; start < len(windowIDs); start += columns {
			end := min(start+columns, len(windowIDs))
			row, err := r.buildArrangement(windowIDs[start:end], ArrangementHorizontal)
			if err != nil {
				return LayoutNode{}, err
			}
			rows = append(rows, row)
		}
		if len(rows) == 1 {
			return rows[0], nil
		}
		id, err := r.generate("node")
		if err != nil {
			return LayoutNode{}, fmt.Errorf("generate grid root ID: %w", err)
		}
		weights := equalWeights(len(rows))
		axis := AxisVertical
		return LayoutNode{ID: NodeID(id), Kind: NodeKindSplit, Axis: &axis, Children: rows, Weights: weights}, nil
	default:
		return LayoutNode{}, fmt.Errorf("arrangement %q: %w", arrangement, ErrInvalidCommand)
	}
}

func (r *reducer) buildSplit(windowIDs []WindowID, axis Axis) (LayoutNode, error) {
	children := make([]LayoutNode, len(windowIDs))
	for index, windowID := range windowIDs {
		leaf, err := newLeaf(windowID, r.generate)
		if err != nil {
			return LayoutNode{}, err
		}
		children[index] = leaf
	}
	id, err := r.generate("node")
	if err != nil {
		return LayoutNode{}, fmt.Errorf("generate split ID: %w", err)
	}
	return LayoutNode{
		ID:       NodeID(id),
		Kind:     NodeKindSplit,
		Axis:     &axis,
		Children: children,
		Weights:  equalWeights(len(children)),
	}, nil
}
