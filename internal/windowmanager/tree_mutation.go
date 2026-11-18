package windowmanager

import (
	"fmt"
	"slices"
)

type idGenerator func(string) (string, error)

func removeWindow(snapshot *Snapshot, id WindowID) bool {
	p, ok := findWindowPlacement(snapshot, id)
	if !ok {
		return false
	}
	d := &snapshot.Desktops[p.desktopIndex]
	root, removed := removeWindowFromNode(d.Tabs[p.tabIndex].Root, id)
	if root.Kind == "" {
		removeTabAt(d, p.tabIndex)
	} else {
		d.Tabs[p.tabIndex].Root = flattenNode(root)
	}
	return removed
}

func removeTabAt(d *Desktop, index int) {
	id := d.Tabs[index].ID
	d.Tabs = slices.Delete(d.Tabs, index, index+1)
	if d.ActiveTabID == id {
		d.ActiveTabID = ""
		if len(d.Tabs) > 0 {
			d.ActiveTabID = d.Tabs[min(index, len(d.Tabs)-1)].ID
		}
	}
}

func removeWindowFromNode(node LayoutNode, id WindowID) (LayoutNode, bool) {
	if node.Kind == NodeKindLeaf && node.WindowID != nil && *node.WindowID == id {
		return LayoutNode{}, true
	}
	for i, child := range node.Children {
		next, removed := removeWindowFromNode(child, id)
		if !removed {
			continue
		}
		if next.Kind == "" {
			node.Children = slices.Delete(node.Children, i, i+1)
			if i < len(node.Weights) {
				node.Weights = slices.Delete(node.Weights, i, i+1)
			}
			normalizeWeights(node.Weights)
		} else {
			node.Children[i] = next
		}
		return flattenNode(node), true
	}
	return node, false
}

func flattenNode(node LayoutNode) LayoutNode {
	if node.Kind != NodeKindSplit {
		return node
	}
	if len(node.Children) == 0 {
		return LayoutNode{}
	}
	if len(node.Children) == 1 {
		return node.Children[0]
	}
	children := make([]LayoutNode, 0, len(node.Children))
	weights := make([]float64, 0, len(node.Weights))
	for i, child := range node.Children {
		child = flattenNode(child)
		weight := 1.0
		if i < len(node.Weights) {
			weight = node.Weights[i]
		}
		if child.Kind == NodeKindSplit && child.Axis != nil && node.Axis != nil && *child.Axis == *node.Axis {
			for j, nested := range child.Children {
				children = append(children, nested)
				weights = append(weights, weight*child.Weights[j])
			}
		} else {
			children = append(children, child)
			weights = append(weights, weight)
		}
	}
	node.Children = children
	node.Weights = weights
	return node
}

func newLeaf(id WindowID, generate idGenerator) (LayoutNode, error) {
	generated, err := generate("node")
	if err != nil {
		return LayoutNode{}, fmt.Errorf("generate leaf: %w", err)
	}
	return LayoutNode{ID: NodeID(generated), Kind: NodeKindLeaf, WindowID: new(id)}, nil
}

func placementAxis(p DropPlacement) (Axis, bool, bool) {
	switch p {
	case DropLeft, DropBefore:
		return AxisHorizontal, true, true
	case DropRight, DropAfter:
		return AxisHorizontal, false, true
	case DropTop:
		return AxisVertical, true, true
	case DropBottom:
		return AxisVertical, false, true
	}
	return "", false, false
}

func insertRelativeInNode(
	node LayoutNode,
	target WindowID,
	incoming LayoutNode,
	p DropPlacement,
	generate idGenerator,
) (LayoutNode, bool, error) {
	axis, before, ok := placementAxis(p)
	if !ok {
		return node, false, ErrInvalidCommand
	}
	if node.Kind == NodeKindLeaf && node.WindowID != nil && *node.WindowID == target {
		id, err := generate("node")
		if err != nil {
			return node, false, err
		}
		children := []LayoutNode{node, incoming}
		if before {
			children[0], children[1] = incoming, node
		}
		return flattenNode(
			LayoutNode{
				ID:       NodeID(id),
				Kind:     NodeKindSplit,
				Axis:     new(axis),
				Children: children,
				Weights:  []float64{.5, .5},
			},
		), true, nil
	}
	for i, child := range node.Children {
		if child.Kind == NodeKindLeaf && child.WindowID != nil && *child.WindowID == target && node.Axis != nil &&
			*node.Axis == axis {
			index := i
			if !before {
				index++
			}
			node.Children = slices.Insert(node.Children, index, incoming)
			node.Weights = equalWeights(len(node.Children))
			return flattenNode(node), true, nil
		}
		next, found, err := insertRelativeInNode(child, target, incoming, p, generate)
		if err != nil {
			return node, false, err
		}
		if found {
			node.Children[i] = next
			return flattenNode(node), true, nil
		}
	}
	return node, false, nil
}

func insertTab(d *Desktop, tab Tab, index int) {
	pins := 0
	for _, candidate := range d.Tabs {
		if candidate.Pinned {
			pins++
		}
	}
	lo, hi := pins, len(d.Tabs)
	if tab.Pinned {
		lo, hi = 0, pins
	}
	index = max(lo, min(index, hi))
	d.Tabs = slices.Insert(d.Tabs, index, tab)
	if d.ActiveTabID == "" {
		d.ActiveTabID = tab.ID
	}
}

func assignTabWindows(snapshot *Snapshot, d *Desktop, tab Tab) {
	for _, id := range nodeWindowIDs(tab.Root) {
		w := snapshot.Windows[id]
		w.DesktopID = d.ID
		w.TabID = tab.ID
		snapshot.Windows[id] = w
	}
}
