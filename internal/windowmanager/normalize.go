package windowmanager

import (
	"cmp"
	"math"
	"slices"
)

const weightTolerance = 0.000001

func NormalizeSnapshot(snapshot Snapshot) Snapshot {
	normalized := cloneSnapshot(snapshot)
	slices.SortStableFunc(normalized.Desktops, func(a, b Desktop) int {
		if a.Order == b.Order {
			return cmp.Compare(a.ID, b.ID)
		}
		return cmp.Compare(a.Order, b.Order)
	})
	for i := range normalized.Desktops {
		d := &normalized.Desktops[i]
		d.Order = i
		tabs := make([]Tab, 0, len(d.Tabs))
		for _, tab := range d.Tabs {
			root, present := pruneNode(tab.Root, normalized.Windows)
			if present {
				tab.Root = root
				tabs = append(tabs, tab)
			}
		}
		d.Tabs = tabs
		if len(d.Tabs) > 0 && tabHoldingID(d.Tabs, d.ActiveTabID) < 0 {
			d.ActiveTabID = d.Tabs[0].ID
		}
	}
	return normalized
}

func tabHoldingID(tabs []Tab, id TabID) int {
	for i, t := range tabs {
		if t.ID == id {
			return i
		}
	}
	return -1
}

func pruneNode(node LayoutNode, windows map[WindowID]Window) (LayoutNode, bool) {
	if node.Kind == NodeKindLeaf {
		if node.WindowID == nil {
			return LayoutNode{}, false
		}
		_, ok := windows[*node.WindowID]
		return node, ok
	}
	if node.Kind != NodeKindSplit {
		return node, false
	}
	children := []LayoutNode{}
	weights := []float64{}
	for i, child := range node.Children {
		next, ok := pruneNode(child, windows)
		if !ok {
			continue
		}
		children = append(children, next)
		if i < len(node.Weights) {
			weights = append(weights, node.Weights[i])
		} else {
			weights = append(weights, 1)
		}
	}
	removed := len(children) != len(node.Children)
	node.Children = children
	node.Weights = weights
	if removed {
		normalizeWeights(node.Weights)
	}
	node = flattenNode(node)
	return node, node.Kind != ""
}

func normalizeWeights(weights []float64) {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	if math.Abs(total-1) <= weightTolerance {
		return
	}
	if total > 0 && finite(total) {
		for i := range weights {
			weights[i] /= total
		}
	}
}
