package windowmanager

type windowPlacement struct {
	desktopIndex  int
	groupIndex    int
	floatingIndex int
	nodeID        NodeID
	parentSplitID *NodeID
	childIndex    *int
	weight        *float64
	placement     WindowPlacement
	neighbors     []WindowID
}

func desktopIndexByID(snapshot *Snapshot, desktopID DesktopID) (int, bool) {
	for index := range snapshot.Desktops {
		if snapshot.Desktops[index].ID == desktopID {
			return index, true
		}
	}
	return -1, false
}

func findWindowPlacement(snapshot *Snapshot, windowID WindowID) (windowPlacement, bool) {
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return windowPlacement{}, false
	}
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return windowPlacement{}, false
	}
	desktop := &snapshot.Desktops[desktopIndex]
	for index, candidate := range desktop.Floating {
		if candidate == windowID {
			return windowPlacement{
				desktopIndex:  desktopIndex,
				groupIndex:    -1,
				floatingIndex: index,
				placement:     WindowPlacementFloating,
			}, true
		}
	}
	for groupIndex := range desktop.Groups {
		if placement, found := findInNode(desktop.Groups[groupIndex].Root, windowID, nil, nil); found {
			placement.desktopIndex = desktopIndex
			placement.groupIndex = groupIndex
			placement.floatingIndex = -1
			return placement, true
		}
	}
	return windowPlacement{}, false
}

func findInNode(
	node LayoutNode,
	windowID WindowID,
	parentSplitID *NodeID,
	parentChildIndex *int,
) (windowPlacement, bool) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil && *node.WindowID == windowID {
			return windowPlacement{
				nodeID:        node.ID,
				parentSplitID: clonePointer(parentSplitID),
				childIndex:    clonePointer(parentChildIndex),
				placement:     WindowPlacementTiled,
			}, true
		}
	case NodeKindStack:
		if containsWindowID(node.WindowIDs, windowID) {
			neighbors := make([]WindowID, 0, len(node.WindowIDs)-1)
			for _, candidate := range node.WindowIDs {
				if candidate != windowID {
					neighbors = append(neighbors, candidate)
				}
			}
			return windowPlacement{
				nodeID:        node.ID,
				parentSplitID: clonePointer(parentSplitID),
				childIndex:    clonePointer(parentChildIndex),
				placement:     WindowPlacementStacked,
				neighbors:     neighbors,
			}, true
		}
	case NodeKindSplit:
		for index, child := range node.Children {
			childIndex := index
			placement, found := findInNode(child, windowID, &node.ID, &childIndex)
			if found {
				if index < len(node.Weights) {
					weight := node.Weights[index]
					placement.weight = &weight
				}
				placement.neighbors = appendUniqueWindowIDs(placement.neighbors, nodeWindowIDsExcluding(node, index)...)
				return placement, true
			}
		}
	}
	return windowPlacement{}, false
}

func nodeWindowIDs(node LayoutNode) []WindowID {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID == nil {
			return nil
		}
		return []WindowID{*node.WindowID}
	case NodeKindStack:
		return append([]WindowID(nil), node.WindowIDs...)
	case NodeKindSplit:
		var result []WindowID
		for _, child := range node.Children {
			result = append(result, nodeWindowIDs(child)...)
		}
		return result
	default:
		return nil
	}
}

func nodeWindowIDsExcluding(node LayoutNode, childIndex int) []WindowID {
	var result []WindowID
	for index, child := range node.Children {
		if index != childIndex {
			result = append(result, nodeWindowIDs(child)...)
		}
	}
	return result
}

func appendUniqueWindowIDs(values []WindowID, candidates ...WindowID) []WindowID {
	for _, candidate := range candidates {
		if !containsWindowID(values, candidate) {
			values = append(values, candidate)
		}
	}
	return values
}

func findNode(node *LayoutNode, nodeID NodeID) (*LayoutNode, bool) {
	if node.ID == nodeID {
		return node, true
	}
	for index := range node.Children {
		if found, exists := findNode(&node.Children[index], nodeID); exists {
			return found, true
		}
	}
	return nil, false
}

func findNodeInSnapshot(snapshot *Snapshot, nodeID NodeID) (*LayoutNode, bool) {
	for desktopIndex := range snapshot.Desktops {
		for groupIndex := range snapshot.Desktops[desktopIndex].Groups {
			if node, found := findNode(&snapshot.Desktops[desktopIndex].Groups[groupIndex].Root, nodeID); found {
				return node, true
			}
		}
	}
	return nil, false
}

func captureReturnAnchor(snapshot *Snapshot, windowID WindowID) *ReturnAnchor {
	placement, found := findWindowPlacement(snapshot, windowID)
	if !found {
		return nil
	}
	anchor := &ReturnAnchor{
		DesktopID:      snapshot.Desktops[placement.desktopIndex].ID,
		ParentSplitID:  clonePointer(placement.parentSplitID),
		ChildIndex:     clonePointer(placement.childIndex),
		Weight:         clonePointer(placement.weight),
		NeighborIDs:    append([]WindowID(nil), placement.neighbors...),
		SourceRevision: snapshot.Revision,
	}
	if placement.groupIndex >= 0 {
		sourceGroup := cloneLayoutGroup(snapshot.Desktops[placement.desktopIndex].Groups[placement.groupIndex])
		groupID := sourceGroup.ID
		anchor.GroupID = &groupID
		anchor.SourceGroup = &sourceGroup
	}
	return anchor
}
