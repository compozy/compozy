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
	if entry.Member != nil {
		if destination, _, exists := findTab(s, entry.Member.TabID); exists {
			di, _ = desktopIndexByID(s, destination.ID)
		}
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
		return r.reopenSplitMember(s, d, entry.Windows[0].ID, entry.Member)
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

func (r *reducer) reopenSplitMember(s *Snapshot, d *Desktop, id WindowID, m *ClosedMember) (bool, error) {
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
		} else if root, restored, err := r.restoreBranch(tab.Root, m, leaf); err != nil {
			return false, err
		} else if restored {
			tab.Root = root
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
		return true, nil
	}
	return r.reopenAsTab(s, d, id, len(d.Tabs))
}

func (r *reducer) restoreNeighbor(root LayoutNode, member *ClosedMember, leaf LayoutNode) (LayoutNode, error) {
	if root.Kind == NodeKindLeaf && root.WindowID != nil && *root.WindowID == member.NeighborID {
		return r.splitBeside(root, member, leaf)
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

// restoreBranch re-splits the member beside its former sibling branch when the
// branch's windows still sit together, either as one node or as a contiguous
// run of children after flattening merged the branch into a same-axis parent.
// It reports false, leaving root untouched, when the branch no longer exists.
func (r *reducer) restoreBranch(root LayoutNode, member *ClosedMember, leaf LayoutNode) (LayoutNode, bool, error) {
	if len(member.NeighborIDs) < 2 {
		return root, false, nil
	}
	if sameWindowSet(nodeWindowIDs(root), member.NeighborIDs) {
		next, err := r.splitBeside(root, member, leaf)
		return next, err == nil, err
	}
	if root.Kind != NodeKindSplit {
		return root, false, nil
	}
	for i, child := range root.Children {
		ids := nodeWindowIDs(child)
		if !slices.ContainsFunc(ids, func(id WindowID) bool { return slices.Contains(member.NeighborIDs, id) }) {
			continue
		}
		next, restored, err := r.restoreBranch(child, member, leaf)
		if err != nil {
			return root, false, err
		}
		if restored {
			root.Children[i] = next
			return flattenNode(root), true, nil
		}
		end := i
		for len(ids) < len(member.NeighborIDs) && end+1 < len(root.Children) {
			end++
			ids = append(ids, nodeWindowIDs(root.Children[end])...)
		}
		if end == i || !sameWindowSet(ids, member.NeighborIDs) {
			return root, false, nil
		}
		return r.restoreBranchRun(root, i, end, member, leaf)
	}
	return root, false, nil
}

// restoreBranchRun regroups children start..end of root into their own split
// with root's axis and re-splits the member beside that group.
func (r *reducer) restoreBranchRun(
	root LayoutNode,
	start, end int,
	member *ClosedMember,
	leaf LayoutNode,
) (LayoutNode, bool, error) {
	if root.Axis == nil || len(root.Weights) != len(root.Children) {
		return root, false, nil
	}
	total := 0.0
	for _, weight := range root.Weights[start : end+1] {
		total += weight
	}
	if total <= 0 {
		return root, false, nil
	}
	id, err := r.generate("node")
	if err != nil {
		return root, false, err
	}
	branch := LayoutNode{
		ID:       NodeID(id),
		Kind:     NodeKindSplit,
		Axis:     new(*root.Axis),
		Children: slices.Clone(root.Children[start : end+1]),
		Weights:  make([]float64, 0, end-start+1),
	}
	for _, weight := range root.Weights[start : end+1] {
		branch.Weights = append(branch.Weights, weight/total)
	}
	wrapped, err := r.splitBeside(branch, member, leaf)
	if err != nil {
		return root, false, err
	}
	root.Children = slices.Concat(root.Children[:start], []LayoutNode{wrapped}, root.Children[end+1:])
	root.Weights = slices.Concat(root.Weights[:start], []float64{total}, root.Weights[end+1:])
	return flattenNode(root), true, nil
}

// splitBeside wraps target and leaf in a new split on the member's recorded edge.
func (r *reducer) splitBeside(target LayoutNode, member *ClosedMember, leaf LayoutNode) (LayoutNode, error) {
	axis, before, _ := placementAxis(DropPlacement(member.NeighborEdge))
	id, err := r.generate("node")
	if err != nil {
		return target, err
	}
	children, weights := []LayoutNode{target, leaf}, []float64{1 - member.Weight, member.Weight}
	if before {
		children[0], children[1] = leaf, target
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

func sameWindowSet(a, b []WindowID) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
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
