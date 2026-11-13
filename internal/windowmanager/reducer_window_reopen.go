package windowmanager

import "slices"

func (r *reducer) reopenWindow(s *Snapshot) (bool, error) {
	if len(s.ClosedEntries) == 0 {
		return false, nil
	}
	entry := s.ClosedEntries[len(s.ClosedEntries)-1]
	s.ClosedEntries = s.ClosedEntries[:len(s.ClosedEntries)-1]
	di, ok := desktopIndexByID(s, entry.DesktopID)
	if !ok && r.activeDesktop != nil {
		di, ok = desktopIndexByID(s, *r.activeDesktop)
	}
	if !ok {
		di = 0
	}
	d := &s.Desktops[di]
	for _, w := range entry.Windows {
		if _, exists := s.Windows[w.ID]; exists {
			continue
		}
		w.DesktopID = d.ID
		s.Windows[w.ID] = cloneWindow(w)
		r.changes.window(w.ID)
	}
	if entry.Member != nil && len(entry.Windows) == 1 {
		id := entry.Windows[0].ID
		m := entry.Member
		td, ti, tabExists := findTab(s, m.TabID)
		if tabExists && td.ID == d.ID {
			tab := &td.Tabs[ti]
			leaf, err := newLeaf(id, r.generate)
			if err != nil {
				return false, err
			}
			if parent, exists := findNode(&tab.Root, m.ParentSplitID); exists && parent.Kind == NodeKindSplit {
				index := max(0, min(m.ChildIndex, len(parent.Children)))
				parent.Children = slices.Insert(parent.Children, index, leaf)
				weights := make([]float64, len(parent.Children))
				j := 0
				for i := range weights {
					if i == index {
						weights[i] = m.Weight
					} else {
						weights[i] = parent.Weights[j] * (1 - m.Weight)
						j++
					}
				}
				parent.Weights = weights
				tab.Root = flattenNode(tab.Root)
			} else if slices.Contains(nodeWindowIDs(tab.Root), m.NeighborID) {
				root, err := r.restoreNeighbor(tab.Root, m, leaf)
				if err != nil {
					return false, err
				}
				tab.Root = root
			} else {
				return r.reopenAsTab(s, d, id, ti+1)
			}
			assignTabWindows(s, td, *tab)
			td.ActiveTabID = tab.ID
			r.changes.tab(tab.ID)
			r.changes.tabSplit(tab.ID)
			return true, nil
		}
		return r.reopenAsTab(s, d, id, len(d.Tabs))
	}
	for _, closed := range entry.Tabs {
		tab := Tab{ID: closed.ID, Pinned: closed.Pinned, Root: cloneNode(closed.Root)}
		if _, _, exists := findTab(s, tab.ID); exists {
			generated, err := r.generate("tab")
			if err != nil {
				return false, err
			}
			tab.ID = TabID(generated)
		}
		insertTab(d, tab, closed.Index)
		assignTabWindows(s, d, tab)
		d.ActiveTabID = tab.ID
		r.changes.tab(tab.ID)
	}
	r.changes.desktop(d.ID)
	return len(entry.Windows) > 0, nil
}

func (r *reducer) restoreNeighbor(root LayoutNode, member *ClosedMember, leaf LayoutNode) (LayoutNode, error) {
	if root.Kind == NodeKindLeaf && root.WindowID != nil && *root.WindowID == member.NeighborID {
		axis, before, _ := placementAxis(DropPlacement(member.NeighborEdge))
		id, err := r.generate("node")
		if err != nil {
			return root, err
		}
		children, weights := []LayoutNode{root, leaf}, []float64{1 - member.Weight, member.Weight}
		if before {
			children[0], children[1] = leaf, root
			weights[0], weights[1] = member.Weight, 1-member.Weight
		}
		return LayoutNode{
			ID:       NodeID(id),
			Kind:     NodeKindSplit,
			Axis:     new(axis),
			Children: children,
			Weights:  weights,
		}, nil
	}
	for i, child := range root.Children {
		if slices.Contains(nodeWindowIDs(child), member.NeighborID) {
			next, err := r.restoreNeighbor(child, member, leaf)
			if err != nil {
				return root, err
			}
			root.Children[i] = next
			return flattenNode(root), nil
		}
	}
	return root, ErrWindowNotFound
}

func (r *reducer) reopenAsTab(s *Snapshot, d *Desktop, id WindowID, index int) (bool, error) {
	tab, err := r.newTab(id)
	if err != nil {
		return false, err
	}
	insertTab(d, tab, index)
	assignTabWindows(s, d, tab)
	d.ActiveTabID = tab.ID
	r.changes.tab(tab.ID)
	r.changes.desktop(d.ID)
	return true, nil
}
