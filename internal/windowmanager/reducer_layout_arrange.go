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
	frames := windowArrangeFrames(command.WindowIDs)
	if command.KeepFrames {
		// Each named window brings its whole tab frame: a deck is arranged as
		// one participant, keeping its members, active tab and stack identity.
		frames = collectArrangeFrames(snapshot, command.WindowIDs)
	}
	for _, windowID := range arrangeFrameWindowIDs(frames) {
		removeWindow(snapshot, windowID)
		window := snapshot.Windows[windowID]
		window.Minimized = false
		window.Zoomed = false
		window.ReturnAnchor = nil
		snapshot.Windows[windowID] = window
		r.changes.window(windowID)
	}
	r.clearDesktopZoom(snapshot, command.DesktopID)
	root, err := r.buildArrangement(frames, command.Arrangement)
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
	displaced, err := displaceGroupsForFrame(snapshot.Desktops[desktopIndex].Groups, frame)
	if err != nil {
		return false, err
	}
	for _, displacedID := range displaced {
		r.changes.group(displacedID)
	}
	snapshot.Desktops[desktopIndex].Groups = append(
		snapshot.Desktops[desktopIndex].Groups,
		LayoutGroup{ID: groupID, Frame: frame, Root: root},
	)
	r.changes.desktop(command.DesktopID)
	r.changes.group(groupID)
	return true, nil
}

func (r *reducer) buildArrangement(frames []arrangeFrame, arrangement Arrangement) (LayoutNode, error) {
	// One participant is its own frame under every arrangement: a lone window
	// stays a leaf and a deck keeps its stack identity.
	if len(frames) == 1 {
		return r.frameNode(frames[0])
	}
	switch arrangement {
	case ArrangementHorizontal:
		return r.buildSplit(frames, AxisHorizontal)
	case ArrangementVertical:
		return r.buildSplit(frames, AxisVertical)
	case ArrangementStack:
		// Every participant joins one deck, led by the first frame's active tab.
		return r.frameNode(arrangeFrame{
			windowIDs: arrangeFrameWindowIDs(frames),
			activeID:  firstFrameActive(frames[0]),
		})
	case ArrangementMainStack:
		return r.buildMainStack(frames)
	case ArrangementGrid:
		columns := int(math.Ceil(math.Sqrt(float64(len(frames)))))
		rows := make([]LayoutNode, 0, (len(frames)+columns-1)/columns)
		for start := 0; start < len(frames); start += columns {
			end := min(start+columns, len(frames))
			row, err := r.buildArrangement(frames[start:end], ArrangementHorizontal)
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

// mainStackWeight is the main column's share of a main_stack arrangement.
const mainStackWeight = 0.6

func (r *reducer) buildMainStack(frames []arrangeFrame) (LayoutNode, error) {
	main, err := r.frameNode(frames[0])
	if err != nil {
		return LayoutNode{}, err
	}
	side, err := r.buildArrangement(frames[1:], ArrangementVertical)
	if err != nil {
		return LayoutNode{}, err
	}
	id, err := r.generate("node")
	if err != nil {
		return LayoutNode{}, fmt.Errorf("generate main stack ID: %w", err)
	}
	axis := AxisHorizontal
	return LayoutNode{
		ID:       NodeID(id),
		Kind:     NodeKindSplit,
		Axis:     &axis,
		Children: []LayoutNode{main, side},
		Weights:  []float64{mainStackWeight, 1 - mainStackWeight},
	}, nil
}

func (r *reducer) buildSplit(frames []arrangeFrame, axis Axis) (LayoutNode, error) {
	children := make([]LayoutNode, len(frames))
	for index, frame := range frames {
		child, err := r.frameNode(frame)
		if err != nil {
			return LayoutNode{}, err
		}
		children[index] = child
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

// firstFrameActive is the tab a merged deck opens on: the leading frame's
// active member, or its only window.
func firstFrameActive(frame arrangeFrame) *WindowID {
	if frame.activeID != nil {
		return clonePointer(frame.activeID)
	}
	return new(frame.windowIDs[0])
}
