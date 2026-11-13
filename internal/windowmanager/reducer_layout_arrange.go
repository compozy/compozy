package windowmanager

import (
	"math"
	"slices"
)

func (r *reducer) arrange(s *Snapshot, c ArrangeLayoutCommand) (bool, error) {
	di, ok := desktopIndexByID(s, c.DesktopID)
	if !ok {
		return false, ErrDesktopNotFound
	}
	if len(c.WindowIDs) == 0 {
		return false, ErrInvalidCommand
	}
	seen := map[WindowID]bool{}
	nodes := make([]LayoutNode, 0, len(c.WindowIDs))
	first := s.Windows[c.WindowIDs[0]]
	index := tabHoldingID(s.Desktops[di].Tabs, first.TabID)
	if index < 0 {
		return false, ErrWindowNotFound
	}
	predecessors := tabIDs(s.Desktops[di].Tabs[:index])
	for _, id := range c.WindowIDs {
		w, ok := s.Windows[id]
		if !ok {
			return false, ErrWindowNotFound
		}
		if seen[id] || w.DesktopID != c.DesktopID {
			return false, ErrInvalidCommand
		}
		seen[id] = true
		leaf, err := newLeaf(id, r.generate)
		if err != nil {
			return false, err
		}
		nodes = append(nodes, leaf)
	}
	root, err := r.arrangeNodes(nodes, c.Arrangement)
	if err != nil {
		return false, err
	}
	generated, err := r.generate("tab")
	if err != nil {
		return false, err
	}
	for _, id := range c.WindowIDs {
		w := s.Windows[id]
		r.changes.tab(w.TabID)
		removeWindow(s, id)
		r.changes.window(id)
	}
	d := &s.Desktops[di]
	index = 0
	for _, id := range predecessors {
		if tabHoldingID(d.Tabs, id) >= 0 {
			index++
		}
	}
	tab := Tab{ID: TabID(generated), Root: root}
	insertTab(d, tab, index)
	assignTabWindows(s, d, tab)
	d.ActiveTabID = tab.ID
	r.changes.tab(tab.ID)
	r.changes.desktop(d.ID)
	markNodeChanges(root, &r.changes)
	return true, nil
}

func (r *reducer) arrangeNodes(nodes []LayoutNode, a Arrangement) (LayoutNode, error) {
	switch a {
	case ArrangementHorizontal, ArrangementVertical, ArrangementMainStack, ArrangementGrid:
	default:
		return LayoutNode{}, ErrInvalidCommand
	}
	if len(nodes) == 1 {
		return nodes[0], nil
	}
	switch a {
	case ArrangementHorizontal:
		return r.splitNodes(nodes, AxisHorizontal, nil)
	case ArrangementVertical:
		return r.splitNodes(nodes, AxisVertical, nil)
	case ArrangementMainStack:
		side, err := r.arrangeNodes(nodes[1:], ArrangementVertical)
		if err != nil {
			return LayoutNode{}, err
		}
		return r.splitNodes([]LayoutNode{nodes[0], side}, AxisHorizontal, []float64{.6, .4})
	case ArrangementGrid:
		columns := int(math.Ceil(math.Sqrt(float64(len(nodes)))))
		rows := []LayoutNode{}
		for start := 0; start < len(nodes); start += columns {
			row, err := r.arrangeNodes(slices.Clone(nodes[start:min(start+columns, len(nodes))]), ArrangementHorizontal)
			if err != nil {
				return LayoutNode{}, err
			}
			rows = append(rows, row)
		}
		return r.arrangeNodes(rows, ArrangementVertical)
	default:
		return LayoutNode{}, ErrInvalidCommand
	}
}

func (r *reducer) splitNodes(nodes []LayoutNode, axis Axis, weights []float64) (LayoutNode, error) {
	id, err := r.generate("node")
	if err != nil {
		return LayoutNode{}, err
	}
	if weights == nil {
		weights = equalWeights(len(nodes))
	}
	return flattenNode(
		LayoutNode{ID: NodeID(id), Kind: NodeKindSplit, Axis: new(axis), Children: nodes, Weights: weights},
	), nil
}
