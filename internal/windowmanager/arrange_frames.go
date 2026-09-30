package windowmanager

import (
	"fmt"
	"slices"
)

// arrangeFrame is one participant of layout.arrange: a lone window, or a whole
// tab frame (a tiled stack or a floating deck) that moves as a unit so its
// hidden tabs are never pulled out of the deck.
type arrangeFrame struct {
	windowIDs []WindowID
	activeID  *WindowID
	stackID   NodeID
}

// collectArrangeFrames resolves each named window to its frame, in order. A
// window names its whole tab frame; later names inside an already-claimed
// frame are absorbed into it.
func collectArrangeFrames(snapshot *Snapshot, windowIDs []WindowID) []arrangeFrame {
	frames := make([]arrangeFrame, 0, len(windowIDs))
	claimed := make(map[WindowID]struct{}, len(windowIDs))
	for _, windowID := range windowIDs {
		if _, taken := claimed[windowID]; taken {
			continue
		}
		frame := arrangeFrame{windowIDs: []WindowID{windowID}}
		if location, stacked := findStackByWindow(snapshot, windowID); stacked {
			frame = arrangeFrame{
				windowIDs: slices.Clone(location.members()),
				activeID:  clonePointer(location.activeID()),
				stackID:   location.id(),
			}
		}
		for _, memberID := range frame.windowIDs {
			claimed[memberID] = struct{}{}
		}
		frames = append(frames, frame)
	}
	return frames
}

// windowArrangeFrames makes every named window its own participant, pulling a
// named tab out of its deck — the arrange semantics without KeepFrames.
func windowArrangeFrames(windowIDs []WindowID) []arrangeFrame {
	frames := make([]arrangeFrame, 0, len(windowIDs))
	for _, windowID := range windowIDs {
		frames = append(frames, arrangeFrame{windowIDs: []WindowID{windowID}})
	}
	return frames
}

func arrangeFrameWindowIDs(frames []arrangeFrame) []WindowID {
	windowIDs := make([]WindowID, 0, len(frames))
	for _, frame := range frames {
		windowIDs = append(windowIDs, frame.windowIDs...)
	}
	return windowIDs
}

// frameNode is the node a frame occupies in the new arrangement: a leaf for a
// lone window, or the same stack — its ID, members and active tab — for a deck.
func (r *reducer) frameNode(frame arrangeFrame) (LayoutNode, error) {
	if frame.stackID == "" && len(frame.windowIDs) == 1 {
		return newLeaf(frame.windowIDs[0], r.generate)
	}
	stackID := frame.stackID
	if stackID == "" {
		generated, err := r.generate("node")
		if err != nil {
			return LayoutNode{}, fmt.Errorf("generate stack ID: %w", err)
		}
		stackID = NodeID(generated)
	}
	active := frame.activeID
	if active == nil || !slices.Contains(frame.windowIDs, *active) {
		first := frame.windowIDs[0]
		active = &first
	}
	return LayoutNode{
		ID:        stackID,
		Kind:      NodeKindStack,
		WindowIDs: slices.Clone(frame.windowIDs),
		ActiveID:  clonePointer(active),
	}, nil
}
