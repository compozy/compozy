package windowmanager

// sourceGroupSlotID finds the node of the source island that held the window
// directly: its leaf, or the stack it was a tab of.
func sourceGroupSlotID(node LayoutNode, windowID WindowID) (NodeID, bool) {
	if nodeContainsWindowDirectly(node, windowID) {
		return node.ID, true
	}
	for _, child := range node.Children {
		if slotID, found := sourceGroupSlotID(child, windowID); found {
			return slotID, true
		}
	}
	return "", false
}

// sourceGroupResidueWithoutNode is the source island as it looks once the whole
// slot has left it, nil when the slot was all the island held.
func sourceGroupResidueWithoutNode(source LayoutGroup, slotID NodeID) (*LayoutGroup, bool) {
	root, removed := removeNodeFromTree(cloneNode(source.Root), slotID)
	if !removed {
		return nil, false
	}
	if root.Kind == "" {
		return nil, true
	}
	trimmed := cloneLayoutGroup(source)
	trimmed.Root = root
	normalized := NormalizeSnapshot(sourceGroupSnapshot(trimmed))
	if len(normalized.Desktops[0].Groups) == 0 {
		return nil, true
	}
	residue := cloneLayoutGroup(normalized.Desktops[0].Groups[0])
	return &residue, true
}

// restoreExactSourceNode puts a frame back into its source island when the
// island still looks exactly like it did without the frame's slot.
func restoreExactSourceNode(snapshot *Snapshot, ownerID WindowID, node LayoutNode, anchor *ReturnAnchor) bool {
	if anchor == nil || anchor.SourceGroup == nil {
		return false
	}
	desktopIndex, exists := desktopIndexByID(snapshot, anchor.DesktopID)
	if !exists {
		return false
	}
	slotID, found := sourceGroupSlotID(anchor.SourceGroup.Root, ownerID)
	if !found {
		return false
	}
	residue, removed := sourceGroupResidueWithoutNode(*anchor.SourceGroup, slotID)
	if !removed {
		return false
	}
	desktop := &snapshot.Desktops[desktopIndex]
	groupIndex, unchanged := unchangedSourceGroupIndex(desktop, *anchor.SourceGroup, residue)
	if !unchanged {
		return false
	}
	restored := cloneLayoutGroup(*anchor.SourceGroup)
	restored.Root = replaceNode(restored.Root, slotID, node)
	if groupIndex >= 0 {
		desktop.Groups[groupIndex] = restored
	} else {
		desktop.Groups = append(desktop.Groups, restored)
	}
	return true
}

func replaceNode(node LayoutNode, nodeID NodeID, replacement LayoutNode) LayoutNode {
	if node.ID == nodeID {
		return replacement
	}
	for index := range node.Children {
		node.Children[index] = replaceNode(node.Children[index], nodeID, replacement)
	}
	return node
}

// insertNodeAtStructuralAnchor splices a frame into the split its owner left,
// else beside the first neighbor that is still tiled on the anchor desktop.
func insertNodeAtStructuralAnchor(
	snapshot *Snapshot,
	node LayoutNode,
	anchor *ReturnAnchor,
	generate idGenerator,
) (bool, error) {
	desktopIndex, exists := desktopIndexByID(snapshot, anchor.DesktopID)
	if !exists {
		return false, nil
	}
	desktop := &snapshot.Desktops[desktopIndex]
	if anchor.GroupID != nil && anchor.ParentSplitID != nil && anchor.ChildIndex != nil {
		for groupIndex := range desktop.Groups {
			if desktop.Groups[groupIndex].ID != *anchor.GroupID {
				continue
			}
			split, found := findNode(&desktop.Groups[groupIndex].Root, *anchor.ParentSplitID)
			if !found || split.Kind != NodeKindSplit {
				break
			}
			insertNodeIntoSplit(split, node, anchor)
			return true, nil
		}
	}
	for _, neighborID := range anchor.NeighborIDs {
		if !tiledOnDesktop(snapshot, neighborID, anchor.DesktopID) {
			continue
		}
		return true, insertRelativeNode(snapshot, neighborID, node, DropAfter, generate)
	}
	return false, nil
}

// tiledOnDesktop reports whether the window sits in a tiled island of the desktop.
func tiledOnDesktop(snapshot *Snapshot, windowID WindowID, desktopID DesktopID) bool {
	window, exists := snapshot.Windows[windowID]
	if !exists || window.DesktopID != desktopID {
		return false
	}
	placement, placed := findWindowPlacement(snapshot, windowID)
	return placed && placement.groupIndex >= 0
}
