package windowmanager

import "slices"

type windowPlacement struct {
	desktopIndex  int
	tabIndex      int
	nodeID        NodeID
	parentSplitID *NodeID
	childIndex    *int
	weight        *float64
}

func desktopIndexByID(snapshot *Snapshot, id DesktopID) (int, bool) {
	for i, d := range snapshot.Desktops {
		if d.ID == id {
			return i, true
		}
	}
	return -1, false
}

func findTab(snapshot *Snapshot, id TabID) (*Desktop, int, bool) {
	for di := range snapshot.Desktops {
		d := &snapshot.Desktops[di]
		for ti := range d.Tabs {
			if d.Tabs[ti].ID == id {
				return d, ti, true
			}
		}
	}
	return nil, -1, false
}

func findWindowPlacement(snapshot *Snapshot, id WindowID) (windowPlacement, bool) {
	w, ok := snapshot.Windows[id]
	if !ok {
		return windowPlacement{}, false
	}
	di, ok := desktopIndexByID(snapshot, w.DesktopID)
	if !ok {
		return windowPlacement{}, false
	}
	for ti := range snapshot.Desktops[di].Tabs {
		if p, ok := findInNode(snapshot.Desktops[di].Tabs[ti].Root, id, nil, nil); ok {
			p.desktopIndex = di
			p.tabIndex = ti
			return p, true
		}
	}
	return windowPlacement{}, false
}

func findInNode(node LayoutNode, id WindowID, parent *NodeID, index *int) (windowPlacement, bool) {
	if node.Kind == NodeKindLeaf && node.WindowID != nil && *node.WindowID == id {
		return windowPlacement{
			nodeID:        node.ID,
			parentSplitID: clonePointer(parent),
			childIndex:    clonePointer(index),
		}, true
	}
	for i, child := range node.Children {
		if p, ok := findInNode(child, id, &node.ID, new(i)); ok {
			if p.parentSplitID != nil && *p.parentSplitID == node.ID && i < len(node.Weights) {
				p.weight = new(node.Weights[i])
			}
			return p, true
		}
	}
	return windowPlacement{}, false
}

func nodeWindowIDs(node LayoutNode) []WindowID {
	if node.Kind == NodeKindLeaf && node.WindowID != nil {
		return []WindowID{*node.WindowID}
	}
	var ids []WindowID
	for _, child := range node.Children {
		ids = append(ids, nodeWindowIDs(child)...)
	}
	return ids
}

func findNode(node *LayoutNode, id NodeID) (*LayoutNode, bool) {
	if node.ID == id {
		return node, true
	}
	for i := range node.Children {
		if n, ok := findNode(&node.Children[i], id); ok {
			return n, true
		}
	}
	return nil, false
}

func findNodeInSnapshot(snapshot *Snapshot, id NodeID) (*LayoutNode, bool) {
	for di := range snapshot.Desktops {
		for ti := range snapshot.Desktops[di].Tabs {
			if n, ok := findNode(&snapshot.Desktops[di].Tabs[ti].Root, id); ok {
				return n, true
			}
		}
	}
	return nil, false
}

func containsWindowID(ids []WindowID, id WindowID) bool { return slices.Contains(ids, id) }
func appendUniqueWindowIDs(ids []WindowID, candidates ...WindowID) []WindowID {
	for _, id := range candidates {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids
}
