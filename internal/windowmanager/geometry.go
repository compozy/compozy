package windowmanager

import (
	"math"
	"sort"
)

func clampRect(rect NormalizedRect) NormalizedRect {
	if !finite(rect.Width) || rect.Width <= 0 {
		rect.Width = 0.56
	}
	if !finite(rect.Height) || rect.Height <= 0 {
		rect.Height = 0.62
	}
	rect.Width = min(rect.Width, 1)
	rect.Height = min(rect.Height, 1)
	if !finite(rect.X) {
		rect.X = 0.22
	}
	if !finite(rect.Y) {
		rect.Y = 0.18
	}
	rect.X = min(max(rect.X, 0), 1-rect.Width)
	rect.Y = min(max(rect.Y, 0), 1-rect.Height)
	return rect
}

func fullRect() NormalizedRect { return NormalizedRect{Width: 1, Height: 1} }

func projectedRects(snapshot Snapshot, desktopID DesktopID) map[WindowID]NormalizedRect {
	result := make(map[WindowID]NormalizedRect)
	desktopIndex, exists := desktopIndexByID(&snapshot, desktopID)
	if !exists {
		return result
	}
	desktop := snapshot.Desktops[desktopIndex]
	for _, group := range desktop.Groups {
		projectNode(group.Root, group.Frame, result)
	}
	for _, windowID := range desktop.Floating {
		result[windowID] = snapshot.Windows[windowID].FloatingRect
	}
	return result
}

func projectNode(node LayoutNode, frame NormalizedRect, output map[WindowID]NormalizedRect) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil {
			output[*node.WindowID] = frame
		}
	case NodeKindStack:
		for _, windowID := range node.WindowIDs {
			output[windowID] = frame
		}
	case NodeKindSplit:
		offset := 0.0
		for index, child := range node.Children {
			weight := node.Weights[index]
			childFrame := frame
			if node.Axis != nil && *node.Axis == AxisHorizontal {
				childFrame.X = frame.X + frame.Width*offset
				childFrame.Width = frame.Width * weight
			} else {
				childFrame.Y = frame.Y + frame.Height*offset
				childFrame.Height = frame.Height * weight
			}
			projectNode(child, childFrame, output)
			offset += weight
		}
	}
}

func directionalWindow(
	snapshot Snapshot,
	desktopID DesktopID,
	current WindowID,
	direction FocusDirection,
) (WindowID, bool) {
	rects := projectedRects(snapshot, desktopID)
	currentRect, exists := rects[current]
	if !exists {
		return "", false
	}
	cx, cy := center(currentRect)
	type candidate struct {
		id                 WindowID
		primary, secondary float64
	}
	var candidates []candidate
	for windowID, rect := range rects {
		if windowID == current || snapshot.Windows[windowID].Minimized {
			continue
		}
		x, y := center(rect)
		var primary, secondary float64
		switch direction {
		case FocusLeft:
			primary, secondary = cx-x, math.Abs(cy-y)
		case FocusRight:
			primary, secondary = x-cx, math.Abs(cy-y)
		case FocusUp:
			primary, secondary = cy-y, math.Abs(cx-x)
		case FocusDown:
			primary, secondary = y-cy, math.Abs(cx-x)
		default:
			return "", false
		}
		if primary > weightTolerance {
			candidates = append(candidates, candidate{id: windowID, primary: primary, secondary: secondary})
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	sort.Slice(candidates, func(left, right int) bool {
		if math.Abs(candidates[left].primary-candidates[right].primary) > weightTolerance {
			return candidates[left].primary < candidates[right].primary
		}
		if math.Abs(candidates[left].secondary-candidates[right].secondary) > weightTolerance {
			return candidates[left].secondary < candidates[right].secondary
		}
		return candidates[left].id < candidates[right].id
	})
	return candidates[0].id, true
}

func center(rect NormalizedRect) (float64, float64) {
	return rect.X + rect.Width/2, rect.Y + rect.Height/2
}
