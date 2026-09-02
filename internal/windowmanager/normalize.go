package windowmanager

import (
	"cmp"
	"math"
	"slices"
)

const weightTolerance = 0.000001

// NormalizeSnapshot deterministically repairs unambiguous representational residue.
func NormalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := cloneSnapshot(snapshot)
	slices.SortStableFunc(normalized.Desktops, func(left, right Desktop) int {
		if left.Order == right.Order {
			return cmp.Compare(left.ID, right.ID)
		}
		return cmp.Compare(left.Order, right.Order)
	})
	desktopIndexes := make(map[DesktopID]int, len(normalized.Desktops))
	for index := range normalized.Desktops {
		normalized.Desktops[index].Order = index
		desktopIndexes[normalized.Desktops[index].ID] = index
	}
	seen := make(map[WindowID]struct{}, len(normalized.Windows))
	for desktopIndex := range normalized.Desktops {
		normalizeDesktop(&normalized.Desktops[desktopIndex], normalized.Windows, seen)
	}
	windowIDs := make([]WindowID, 0, len(normalized.Windows))
	for windowID := range normalized.Windows {
		windowIDs = append(windowIDs, windowID)
	}
	slices.Sort(windowIDs)
	for _, windowID := range windowIDs {
		if _, present := seen[windowID]; present {
			continue
		}
		window := normalized.Windows[windowID]
		desktopIndex, exists := desktopIndexes[window.DesktopID]
		if !exists {
			continue
		}
		window.Placement = WindowPlacementFloating
		normalized.Windows[windowID] = window
		normalized.Desktops[desktopIndex].Floating = append(normalized.Desktops[desktopIndex].Floating, windowID)
		seen[windowID] = struct{}{}
	}
	for desktopIndex := range normalized.Desktops {
		normalizeDesktopZoom(&normalized.Desktops[desktopIndex], normalized.Windows)
	}
	return normalized
}

func normalizeDesktop(desktop *Desktop, windows map[WindowID]Window, seen map[WindowID]struct{}) {
	groups := make([]LayoutGroup, 0, len(desktop.Groups))
	for _, group := range desktop.Groups {
		root, present := normalizeNode(group.Root, desktop.ID, windows, seen)
		if !present {
			continue
		}
		group.Root = root
		groups = append(groups, group)
	}
	desktop.Groups = groups
	dissolved := normalizeFloatingStacks(desktop, windows, seen)
	normalizeFloatingWindows(desktop, dissolved, windows, seen)
}

// normalizeDesktopZoom keeps zoom only when its unit is the desktop's sole
// visible unit, and always drops zoom from minimized windows.
func normalizeDesktopZoom(desktop *Desktop, windows map[WindowID]Window) {
	visibleUnits := 0
	zoomedUnits := make([][]WindowID, 0, 1)
	forEachDesktopUnit(desktop, func(members []WindowID) {
		visible := false
		zoomed := false
		for _, windowID := range members {
			window, exists := windows[windowID]
			if !exists {
				continue
			}
			if window.Minimized {
				if window.Zoomed {
					window.Zoomed = false
					windows[windowID] = window
				}
				continue
			}
			visible = true
			zoomed = zoomed || window.Zoomed
		}
		if visible {
			visibleUnits++
		}
		if zoomed {
			zoomedUnits = append(zoomedUnits, slices.Clone(members))
		}
	})
	if visibleUnits == 1 && len(zoomedUnits) == 1 {
		return
	}
	for _, members := range zoomedUnits {
		for _, windowID := range members {
			window := windows[windowID]
			if window.Zoomed {
				window.Zoomed = false
				windows[windowID] = window
			}
		}
	}
}

func normalizeFloatingStacks(
	desktop *Desktop,
	windows map[WindowID]Window,
	seen map[WindowID]struct{},
) []WindowID {
	floatingStacks := make([]FloatingStack, 0, len(desktop.FloatingStacks))
	dissolved := make([]WindowID, 0)
	for _, stack := range desktop.FloatingStacks {
		windowIDs := make([]WindowID, 0, len(stack.WindowIDs))
		for _, windowID := range stack.WindowIDs {
			if claimNodeWindow(windowID, desktop.ID, WindowPlacementStacked, windows, seen) {
				window := windows[windowID]
				window.Minimized = false
				windows[windowID] = window
				windowIDs = append(windowIDs, windowID)
			}
		}
		windowIDs = collatePinned(windowIDs, windows)
		switch len(windowIDs) {
		case 0:
			continue
		case 1:
			windowID := windowIDs[0]
			window := windows[windowID]
			window.Placement = WindowPlacementFloating
			window.FloatingRect = clampRect(stack.Rect)
			windows[windowID] = window
			dissolved = append(dissolved, windowID)
			continue
		}
		stack.WindowIDs = windowIDs
		if stack.ActiveID == nil || !containsWindowID(windowIDs, *stack.ActiveID) {
			activeID := windowIDs[0]
			stack.ActiveID = &activeID
		}
		floatingStacks = append(floatingStacks, stack)
	}
	desktop.FloatingStacks = floatingStacks
	return dissolved
}

func normalizeFloatingWindows(
	desktop *Desktop,
	dissolved []WindowID,
	windows map[WindowID]Window,
	seen map[WindowID]struct{},
) {
	source := desktop.Floating
	floating := append(make([]WindowID, 0, len(dissolved)+len(source)), dissolved...)
	for _, windowID := range source {
		window, exists := windows[windowID]
		if !exists || window.DesktopID != desktop.ID {
			continue
		}
		if _, duplicate := seen[windowID]; duplicate {
			continue
		}
		seen[windowID] = struct{}{}
		window.Placement = WindowPlacementFloating
		windows[windowID] = window
		floating = append(floating, windowID)
	}
	desktop.Floating = floating
}

func normalizeNode(
	node LayoutNode,
	desktopID DesktopID,
	windows map[WindowID]Window,
	seen map[WindowID]struct{},
) (LayoutNode, bool) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID == nil || !claimNodeWindow(*node.WindowID, desktopID, WindowPlacementTiled, windows, seen) {
			return LayoutNode{}, false
		}
		return LayoutNode{ID: node.ID, Kind: NodeKindLeaf, WindowID: clonePointer(node.WindowID)}, true
	case NodeKindStack:
		windowIDs := make([]WindowID, 0, len(node.WindowIDs))
		for _, windowID := range node.WindowIDs {
			if claimNodeWindow(windowID, desktopID, WindowPlacementStacked, windows, seen) {
				windowIDs = append(windowIDs, windowID)
			}
		}
		windowIDs = collatePinned(windowIDs, windows)
		if len(windowIDs) == 0 {
			return LayoutNode{}, false
		}
		if len(windowIDs) == 1 {
			windowID := windowIDs[0]
			window := windows[windowID]
			window.Placement = WindowPlacementTiled
			windows[windowID] = window
			return LayoutNode{ID: node.ID, Kind: NodeKindLeaf, WindowID: &windowID}, true
		}
		activeID := windowIDs[0]
		if node.ActiveID != nil && containsWindowID(windowIDs, *node.ActiveID) {
			activeID = *node.ActiveID
		}
		return LayoutNode{ID: node.ID, Kind: NodeKindStack, WindowIDs: windowIDs, ActiveID: &activeID}, true
	case NodeKindSplit:
		return normalizeSplit(node, desktopID, windows, seen)
	default:
		return LayoutNode{}, false
	}
}

func normalizeSplit(
	node LayoutNode,
	desktopID DesktopID,
	windows map[WindowID]Window,
	seen map[WindowID]struct{},
) (LayoutNode, bool) {
	if node.Axis == nil {
		return LayoutNode{}, false
	}
	children := make([]LayoutNode, 0, len(node.Children))
	weights := make([]float64, 0, len(node.Children))
	for index, child := range node.Children {
		normalized, present := normalizeNode(child, desktopID, windows, seen)
		if !present {
			continue
		}
		weight := 1.0
		if index < len(node.Weights) && finite(node.Weights[index]) && node.Weights[index] > 0 {
			weight = node.Weights[index]
		}
		if normalized.Kind == NodeKindSplit && normalized.Axis != nil && *normalized.Axis == *node.Axis {
			for nestedIndex, nestedChild := range normalized.Children {
				children = append(children, nestedChild)
				weights = append(weights, weight*normalized.Weights[nestedIndex])
			}
			continue
		}
		children = append(children, normalized)
		weights = append(weights, weight)
	}
	if len(children) == 0 {
		return LayoutNode{}, false
	}
	if len(children) == 1 {
		return children[0], true
	}
	normalizeWeights(weights)
	axis := *node.Axis
	return LayoutNode{ID: node.ID, Kind: NodeKindSplit, Axis: &axis, Children: children, Weights: weights}, true
}

func claimNodeWindow(
	windowID WindowID,
	desktopID DesktopID,
	placement WindowPlacement,
	windows map[WindowID]Window,
	seen map[WindowID]struct{},
) bool {
	window, exists := windows[windowID]
	if !exists || window.DesktopID != desktopID {
		return false
	}
	if _, duplicate := seen[windowID]; duplicate {
		return false
	}
	seen[windowID] = struct{}{}
	window.Placement = placement
	windows[windowID] = window
	return true
}

func normalizeWeights(weights []float64) {
	total := 0.0
	repaired := false
	for index, weight := range weights {
		if !finite(weight) || weight <= 0 {
			weights[index] = 1
			repaired = true
		}
		total += weights[index]
	}
	if total <= 0 || !finite(total) {
		total = float64(len(weights))
		repaired = true
		for index := range weights {
			weights[index] = 1
		}
	}
	// An already-normalized vector stays byte-identical so repeated normalization never drifts weights.
	if !repaired && math.Abs(total-1) <= weightTolerance {
		return
	}
	for index := range weights {
		weights[index] /= total
	}
}

func containsWindowID(values []WindowID, target WindowID) bool {
	return slices.Contains(values, target)
}
