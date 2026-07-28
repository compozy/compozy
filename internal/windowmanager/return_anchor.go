package windowmanager

import (
	"math"
	"slices"
)

func restoreExactSourceGroup(snapshot *Snapshot, windowID WindowID, anchor *ReturnAnchor) bool {
	if anchor == nil || anchor.SourceGroup == nil {
		return false
	}
	desktopIndex, exists := desktopIndexByID(snapshot, anchor.DesktopID)
	if !exists {
		return false
	}
	residue, removed := sourceGroupResidue(*anchor.SourceGroup, windowID)
	if !removed {
		return false
	}
	desktop := &snapshot.Desktops[desktopIndex]
	groupIndex, unchanged := unchangedSourceGroupIndex(desktop, *anchor.SourceGroup, residue)
	if !unchanged {
		return false
	}
	window, exists := snapshot.Windows[windowID]
	if !exists {
		return false
	}
	placement, exists := sourceGroupWindowPlacement(anchor.SourceGroup.Root, windowID)
	if !exists {
		return false
	}
	sourceGroup := cloneLayoutGroup(*anchor.SourceGroup)
	if groupIndex >= 0 {
		desktop.Groups[groupIndex] = sourceGroup
	} else {
		desktop.Groups = append(desktop.Groups, sourceGroup)
	}
	window.DesktopID = anchor.DesktopID
	window.Placement = placement
	snapshot.Windows[windowID] = window
	return true
}

func unchangedSourceGroupIndex(desktop *Desktop, source LayoutGroup, residue *LayoutGroup) (int, bool) {
	for index := range desktop.Groups {
		group := desktop.Groups[index]
		if group.ID == source.ID {
			return index, residue != nil && layoutGroupsEqual(group, *residue)
		}
		if residue == nil && rectsOverlap(group.Frame, source.Frame) {
			return -1, false
		}
	}
	return -1, residue == nil
}

func sourceGroupResidue(source LayoutGroup, windowID WindowID) (*LayoutGroup, bool) {
	snapshot := sourceGroupSnapshot(source)
	if !removeWindow(&snapshot, windowID) {
		return nil, false
	}
	delete(snapshot.Windows, windowID)
	normalized := NormalizeSnapshot(snapshot)
	if len(normalized.Desktops[0].Groups) == 0 {
		return nil, true
	}
	residue := cloneLayoutGroup(normalized.Desktops[0].Groups[0])
	return &residue, true
}

func sourceGroupSnapshot(source LayoutGroup) Snapshot {
	const sourceDesktopID DesktopID = "return-anchor-source"
	windows := make(map[WindowID]Window)
	collectSourceGroupWindows(source.Root, sourceDesktopID, windows)
	return Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: WorkspaceID(sourceDesktopID),
		Desktops: []Desktop{{
			ID: sourceDesktopID, Purpose: DesktopPurposeStandard,
			Groups: []LayoutGroup{cloneLayoutGroup(source)}, Floating: []WindowID{},
		}},
		Windows: windows,
	}
}

func collectSourceGroupWindows(node LayoutNode, desktopID DesktopID, windows map[WindowID]Window) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil {
			windows[*node.WindowID] = anchorWindow(*node.WindowID, desktopID, WindowPlacementTiled)
		}
	case NodeKindStack:
		for _, windowID := range node.WindowIDs {
			windows[windowID] = anchorWindow(windowID, desktopID, WindowPlacementStacked)
		}
	case NodeKindSplit:
		for _, child := range node.Children {
			collectSourceGroupWindows(child, desktopID, windows)
		}
	}
}

func anchorWindow(windowID WindowID, desktopID DesktopID, placement WindowPlacement) Window {
	return Window{
		ID: windowID, App: "Return anchor", Route: RouteIntent{Pathname: "/", Search: RouteSearch{}},
		DesktopID: desktopID, Placement: placement, FloatingRect: fullRect(),
	}
}

func sourceGroupWindowPlacement(node LayoutNode, windowID WindowID) (WindowPlacement, bool) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil && *node.WindowID == windowID {
			return WindowPlacementTiled, true
		}
	case NodeKindStack:
		if containsWindowID(node.WindowIDs, windowID) {
			return WindowPlacementStacked, true
		}
	case NodeKindSplit:
		for _, child := range node.Children {
			if placement, exists := sourceGroupWindowPlacement(child, windowID); exists {
				return placement, true
			}
		}
	}
	return "", false
}

func layoutGroupsEqual(left, right LayoutGroup) bool {
	return left.ID == right.ID && left.Frame == right.Frame && layoutNodesEqual(left.Root, right.Root)
}

func layoutNodesEqual(left, right LayoutNode) bool {
	if left.ID != right.ID ||
		left.Kind != right.Kind ||
		!pointerValuesEqual(left.WindowID, right.WindowID) ||
		!pointerValuesEqual(left.Axis, right.Axis) ||
		!weightVectorsEqual(left.Weights, right.Weights) ||
		!slices.Equal(left.WindowIDs, right.WindowIDs) ||
		!pointerValuesEqual(left.ActiveID, right.ActiveID) ||
		len(left.Children) != len(right.Children) {
		return false
	}
	for index := range left.Children {
		if !layoutNodesEqual(left.Children[index], right.Children[index]) {
			return false
		}
	}
	return true
}

// weightVectorsEqual treats sub-tolerance weight drift as unchanged.
func weightVectorsEqual(left, right []float64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if math.Abs(left[index]-right[index]) > weightTolerance {
			return false
		}
	}
	return true
}

func pointerValuesEqual[T comparable](left, right *T) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
