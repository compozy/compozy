package windowmanager

import (
	"cmp"
	"math"
	"slices"
)

func fullRect() NormalizedRect { return NormalizedRect{Width: 1, Height: 1} }

func projectedRects(snapshot Snapshot, desktopID DesktopID) map[WindowID]NormalizedRect {
	result := make(map[WindowID]NormalizedRect)
	desktopIndex, exists := desktopIndexByID(&snapshot, desktopID)
	if !exists {
		return result
	}
	desktop := snapshot.Desktops[desktopIndex]
	for _, group := range desktop.Tabs {
		projectNode(group.Root, fullRect(), result)
	}

	return result
}

func projectNode(node LayoutNode, frame NormalizedRect, output map[WindowID]NormalizedRect) {
	switch node.Kind {
	case NodeKindLeaf:
		if node.WindowID != nil {
			output[*node.WindowID] = frame
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

// directionalWindow picks the neighbour of current in direction, the way a tiling window
// manager moves focus: only windows past current's edge qualify, a window sharing
// current's span on the other axis beats one diagonal to it, then the nearest edge wins,
// then the closest centre. Comparing centres alone let a full-height window beside a
// stacked pair count as "below" the top pane and steal focus from the pane under it.
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
	type candidate struct {
		id WindowID
		directionalFit
	}
	var candidates []candidate
	for windowID, rect := range rects {
		if windowID == current || snapshot.Windows[windowID].TabID != snapshot.Windows[current].TabID {
			continue
		}
		next, ok := directionalCandidate(currentRect, rect, direction)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate{id: windowID, directionalFit: next})
	}
	if len(candidates) == 0 {
		return "", false
	}
	slices.SortFunc(candidates, func(a, b candidate) int {
		if a.overlaps != b.overlaps {
			if a.overlaps {
				return -1
			}
			return 1
		}
		if math.Abs(a.gap-b.gap) > weightTolerance {
			return cmp.Compare(a.gap, b.gap)
		}
		if math.Abs(a.centreDistance-b.centreDistance) > weightTolerance {
			return cmp.Compare(a.centreDistance, b.centreDistance)
		}
		return cmp.Compare(a.id, b.id)
	})
	return candidates[0].id, true
}

type directionalFit struct {
	overlaps            bool
	gap, centreDistance float64
}

// directionalCandidate measures rect from current along direction; ok is false when rect
// does not lie past current's edge in that direction.
func directionalCandidate(current, rect NormalizedRect, direction FocusDirection) (directionalFit, bool) {
	cx, cy := center(current)
	x, y := center(rect)
	var gap, overlap, centreDistance float64
	switch direction {
	case FocusLeft:
		gap = current.X - (rect.X + rect.Width)
		overlap = spanOverlap(current.Y, current.Height, rect.Y, rect.Height)
		centreDistance = math.Abs(cy - y)
	case FocusRight:
		gap = rect.X - (current.X + current.Width)
		overlap = spanOverlap(current.Y, current.Height, rect.Y, rect.Height)
		centreDistance = math.Abs(cy - y)
	case FocusUp:
		gap = current.Y - (rect.Y + rect.Height)
		overlap = spanOverlap(current.X, current.Width, rect.X, rect.Width)
		centreDistance = math.Abs(cx - x)
	case FocusDown:
		gap = rect.Y - (current.Y + current.Height)
		overlap = spanOverlap(current.X, current.Width, rect.X, rect.Width)
		centreDistance = math.Abs(cx - x)
	default:
		return directionalFit{}, false
	}
	if gap < -weightTolerance {
		return directionalFit{}, false
	}
	return directionalFit{overlaps: overlap > weightTolerance, gap: math.Max(gap, 0), centreDistance: centreDistance}, true
}

func spanOverlap(start, length, otherStart, otherLength float64) float64 {
	return min(start+length, otherStart+otherLength) - max(start, otherStart)
}

func center(rect NormalizedRect) (float64, float64) {
	return rect.X + rect.Width/2, rect.Y + rect.Height/2
}
