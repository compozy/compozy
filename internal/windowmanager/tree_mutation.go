package windowmanager

import "fmt"

type idGenerator func(string) (string, error)

func removeWindow(snapshot *Snapshot, windowID WindowID) bool {
	placement, found := findWindowPlacement(snapshot, windowID)
	if !found {
		return false
	}
	bequeathZoom(snapshot, windowID)
	desktop := &snapshot.Desktops[placement.desktopIndex]
	if placement.floatingStackIndex >= 0 {
		stack := desktop.FloatingStacks[placement.floatingStackIndex]
		removedIndex := -1
		for index, memberID := range stack.WindowIDs {
			if memberID == windowID {
				removedIndex = index
				stack.WindowIDs = append(stack.WindowIDs[:index], stack.WindowIDs[index+1:]...)
				break
			}
		}
		if removedIndex < 0 {
			return false
		}
		if stack.ActiveID == nil || *stack.ActiveID == windowID {
			stack.ActiveID = nextActiveAfterRemoval(stack.WindowIDs, removedIndex)
		}
		switch len(stack.WindowIDs) {
		case 1:
			survivorID := stack.WindowIDs[0]
			survivor := snapshot.Windows[survivorID]
			survivor.Placement = WindowPlacementFloating
			survivor.FloatingRect = stack.Rect
			snapshot.Windows[survivorID] = survivor
			desktop.Floating = append(desktop.Floating, survivorID)
			desktop.FloatingStacks = append(
				desktop.FloatingStacks[:placement.floatingStackIndex],
				desktop.FloatingStacks[placement.floatingStackIndex+1:]...,
			)
		case 0:
			desktop.FloatingStacks = append(
				desktop.FloatingStacks[:placement.floatingStackIndex],
				desktop.FloatingStacks[placement.floatingStackIndex+1:]...,
			)
		default:
			desktop.FloatingStacks[placement.floatingStackIndex] = stack
		}
		return true
	}
	if placement.floatingIndex >= 0 {
		desktop.Floating = append(
			desktop.Floating[:placement.floatingIndex],
			desktop.Floating[placement.floatingIndex+1:]...)
		return true
	}
	root, removed := removeWindowFromNode(desktop.Groups[placement.groupIndex].Root, windowID)
	if !removed {
		return false
	}
	if root.Kind == "" {
		desktop.Groups = append(desktop.Groups[:placement.groupIndex], desktop.Groups[placement.groupIndex+1:]...)
	} else {
		desktop.Groups[placement.groupIndex].Root = root
	}
	return true
}

// bequeathZoom keeps a zoomed tab frame zoomed when the zoomed member leaves
// it; the heir also takes over the slot the frame returns to on unzoom.
func bequeathZoom(snapshot *Snapshot, windowID WindowID) {
	window, exists := snapshot.Windows[windowID]
	if !exists || !window.Zoomed {
		return
	}
	location, stacked := findStackByWindow(snapshot, windowID)
	if !stacked {
		return
	}
	heirID, found := zoomHeir(location.members(), location.activeID(), windowID)
	if !found {
		return
	}
	heir := snapshot.Windows[heirID]
	heir.Zoomed = true
	heir.ReturnAnchor = cloneReturnAnchor(window.ReturnAnchor)
	snapshot.Windows[heirID] = heir
}

func removeWindowFromNode(node LayoutNode, windowID WindowID) (LayoutNode, bool) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil && *node.WindowID == windowID {
			return LayoutNode{}, true
		}
		return node, false
	case NodeKindStack:
		for index, candidate := range node.WindowIDs {
			if candidate != windowID {
				continue
			}
			node.WindowIDs = append(node.WindowIDs[:index], node.WindowIDs[index+1:]...)
			if len(node.WindowIDs) == 0 {
				return LayoutNode{}, true
			}
			if node.ActiveID == nil || *node.ActiveID == windowID {
				node.ActiveID = nextActiveAfterRemoval(node.WindowIDs, index)
			}
			if len(node.WindowIDs) == 1 {
				survivor := node.WindowIDs[0]
				return LayoutNode{ID: node.ID, Kind: NodeKindLeaf, WindowID: &survivor}, true
			}
			return node, true
		}
		return node, false
	case NodeKindSplit:
		for index, child := range node.Children {
			updated, removed := removeWindowFromNode(child, windowID)
			if !removed {
				continue
			}
			if updated.Kind == "" {
				node.Children = append(node.Children[:index], node.Children[index+1:]...)
				if index < len(node.Weights) {
					node.Weights = append(node.Weights[:index], node.Weights[index+1:]...)
				}
				if len(node.Children) == 0 {
					// The last member left: the split is gone, so its island is
					// gone too and cannot block a frame placed where it stood.
					return LayoutNode{}, true
				}
			} else {
				node.Children[index] = updated
			}
			return node, true
		}
	}
	return node, false
}

func insertAtAnchor(
	snapshot *Snapshot,
	windowID WindowID,
	anchor *ReturnAnchor,
	fallback *WindowID,
	generate idGenerator,
) error {
	if rejoinAnchorStack(snapshot, windowID, anchor) {
		return nil
	}
	inserted, err := insertAtStructuralAnchor(snapshot, windowID, anchor, generate)
	if err != nil || inserted {
		return err
	}
	if fallback != nil && *fallback != windowID {
		placement, exists := findWindowPlacement(snapshot, *fallback)
		if exists && placement.groupIndex >= 0 &&
			snapshot.Desktops[placement.desktopIndex].ID == snapshot.Windows[windowID].DesktopID {
			return insertRelative(snapshot, *fallback, windowID, DropAfter, generate)
		}
	}
	return appendFloating(snapshot, windowID)
}

// rejoinAnchorStack returns a window to its original stack when that stack still exists.
func rejoinAnchorStack(snapshot *Snapshot, windowID WindowID, anchor *ReturnAnchor) bool {
	if anchor == nil || anchor.SourceGroup == nil {
		return false
	}
	stackID, found := sourceGroupStackID(anchor.SourceGroup.Root, windowID)
	if !found {
		return false
	}
	desktopIndex, exists := desktopIndexByID(snapshot, anchor.DesktopID)
	if !exists {
		return false
	}
	desktop := &snapshot.Desktops[desktopIndex]
	for groupIndex := range desktop.Groups {
		node, ok := findNode(&desktop.Groups[groupIndex].Root, stackID)
		if !ok || node.Kind != NodeKindStack || containsWindowID(node.WindowIDs, windowID) {
			continue
		}
		node.WindowIDs = append(node.WindowIDs, windowID)
		active := windowID
		node.ActiveID = &active
		window := snapshot.Windows[windowID]
		window.DesktopID = anchor.DesktopID
		window.Placement = WindowPlacementStacked
		snapshot.Windows[windowID] = window
		return true
	}
	return false
}

func sourceGroupStackID(node LayoutNode, windowID WindowID) (NodeID, bool) {
	switch node.Kind {
	case NodeKindStack:
		if containsWindowID(node.WindowIDs, windowID) {
			return node.ID, true
		}
	case NodeKindSplit:
		for _, child := range node.Children {
			if id, found := sourceGroupStackID(child, windowID); found {
				return id, true
			}
		}
	case NodeKindLeaf:
	}
	return "", false
}

func insertAtStructuralAnchor(
	snapshot *Snapshot,
	windowID WindowID,
	anchor *ReturnAnchor,
	generate idGenerator,
) (bool, error) {
	if anchor == nil {
		return false, nil
	}
	desktopIndex, exists := desktopIndexByID(snapshot, anchor.DesktopID)
	if !exists {
		return false, nil
	}
	desktop := &snapshot.Desktops[desktopIndex]
	if inserted, err := insertIntoAnchorSplit(desktop, windowID, anchor, generate); err != nil || inserted {
		return inserted, err
	}
	for _, neighborID := range anchor.NeighborIDs {
		if tiledOnDesktop(snapshot, neighborID, anchor.DesktopID) {
			return true, insertRelative(snapshot, neighborID, windowID, DropAfter, generate)
		}
	}
	return false, nil
}

func insertIntoAnchorSplit(
	desktop *Desktop,
	windowID WindowID,
	anchor *ReturnAnchor,
	generate idGenerator,
) (bool, error) {
	if anchor.GroupID == nil || anchor.ParentSplitID == nil || anchor.ChildIndex == nil {
		return false, nil
	}
	for groupIndex := range desktop.Groups {
		if desktop.Groups[groupIndex].ID != *anchor.GroupID {
			continue
		}
		split, found := findNode(&desktop.Groups[groupIndex].Root, *anchor.ParentSplitID)
		if !found || split.Kind != NodeKindSplit {
			return false, nil
		}
		return true, insertLeafIntoSplit(split, windowID, anchor, generate)
	}
	return false, nil
}

func insertLeafIntoSplit(
	split *LayoutNode,
	windowID WindowID,
	anchor *ReturnAnchor,
	generate idGenerator,
) error {
	leaf, err := newLeaf(windowID, generate)
	if err != nil {
		return err
	}
	insertNodeIntoSplit(split, leaf, anchor)
	return nil
}

// insertNodeIntoSplit puts a node back at the child index it left, with the
// weight it had when the anchor still knows it.
func insertNodeIntoSplit(split *LayoutNode, node LayoutNode, anchor *ReturnAnchor) {
	index := min(max(*anchor.ChildIndex, 0), len(split.Children))
	split.Children = append(split.Children, LayoutNode{})
	copy(split.Children[index+1:], split.Children[index:])
	split.Children[index] = node
	weight := 1 / float64(len(split.Children))
	if anchor.Weight != nil && finite(*anchor.Weight) && *anchor.Weight > 0 {
		weight = *anchor.Weight
	}
	split.Weights = append(split.Weights, 0)
	copy(split.Weights[index+1:], split.Weights[index:])
	split.Weights[index] = weight
	normalizeWeights(split.Weights)
}

func appendFloating(snapshot *Snapshot, windowID WindowID) error {
	window := snapshot.Windows[windowID]
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return fmt.Errorf("desktop %q: %w", window.DesktopID, ErrDesktopNotFound)
	}
	snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
	return nil
}

func insertRelative(
	snapshot *Snapshot,
	targetID, windowID WindowID,
	placement DropPlacement,
	generate idGenerator,
) error {
	targetPlacement, found := findWindowPlacement(snapshot, targetID)
	if !found || targetPlacement.groupIndex < 0 {
		return fmt.Errorf("target window %q is not tiled: %w", targetID, ErrInvalidCommand)
	}
	leaf, err := newLeaf(windowID, generate)
	if err != nil {
		return err
	}
	root := &snapshot.Desktops[targetPlacement.desktopIndex].Groups[targetPlacement.groupIndex].Root
	updated, inserted, err := insertRelativeInNode(*root, targetID, leaf, placement, generate)
	if err != nil {
		return err
	}
	if !inserted {
		return fmt.Errorf("target window %q: %w", targetID, ErrWindowNotFound)
	}
	*root = updated
	return nil
}

func insertRelativeInNode(
	node LayoutNode,
	targetID WindowID,
	leaf LayoutNode,
	placement DropPlacement,
	generate idGenerator,
) (LayoutNode, bool, error) {
	if nodeContainsWindowDirectly(node, targetID) {
		if placement == DropCenter {
			if node.Kind == NodeKindStack {
				node.WindowIDs = append(node.WindowIDs, *leaf.WindowID)
				active := *leaf.WindowID
				node.ActiveID = &active
				return node, true, nil
			}
			id, err := generate("node")
			if err != nil {
				return LayoutNode{}, false, fmt.Errorf("generate stack ID: %w", err)
			}
			active := *leaf.WindowID
			return LayoutNode{
				ID:        NodeID(id),
				Kind:      NodeKindStack,
				WindowIDs: []WindowID{targetID, *leaf.WindowID},
				ActiveID:  &active,
			}, true, nil
		}
		axis, before, ok := placementAxis(placement)
		if !ok {
			return LayoutNode{}, false, fmt.Errorf("placement %q: %w", placement, ErrInvalidCommand)
		}
		id, err := generate("node")
		if err != nil {
			return LayoutNode{}, false, fmt.Errorf("generate split ID: %w", err)
		}
		children := []LayoutNode{node, leaf}
		if before {
			children[0], children[1] = leaf, node
		}
		return LayoutNode{
			ID:       NodeID(id),
			Kind:     NodeKindSplit,
			Axis:     &axis,
			Children: children,
			Weights:  []float64{0.5, 0.5},
		}, true, nil
	}
	for index, child := range node.Children {
		updated, inserted, err := insertRelativeInNode(child, targetID, leaf, placement, generate)
		if err != nil {
			return LayoutNode{}, false, err
		}
		if inserted {
			node.Children[index] = updated
			return node, true, nil
		}
	}
	return node, false, nil
}

func nodeContainsWindowDirectly(node LayoutNode, windowID WindowID) bool {
	return (node.Kind == NodeKindLeaf && node.WindowID != nil && *node.WindowID == windowID) ||
		(node.Kind == NodeKindStack && containsWindowID(node.WindowIDs, windowID))
}

func placementAxis(placement DropPlacement) (Axis, bool, bool) {
	switch placement {
	case DropBefore, DropLeft:
		return AxisHorizontal, true, true
	case DropAfter, DropRight:
		return AxisHorizontal, false, true
	case DropTop:
		return AxisVertical, true, true
	case DropBottom:
		return AxisVertical, false, true
	default:
		return "", false, false
	}
}

func newLeaf(windowID WindowID, generate idGenerator) (LayoutNode, error) {
	id, err := generate("node")
	if err != nil {
		return LayoutNode{}, fmt.Errorf("generate leaf ID: %w", err)
	}
	return LayoutNode{ID: NodeID(id), Kind: NodeKindLeaf, WindowID: &windowID}, nil
}
