package windowmanager

import "slices"

func (r *reducer) closeWindow(s *Snapshot, c CloseWindowCommand) (bool, error) {
	p, ok := findWindowPlacement(s, c.WindowID)
	if !ok {
		return false, ErrWindowNotFound
	}
	entry, err := r.captureClosedEntry(s, c, p)
	if err != nil {
		return false, err
	}
	if len(entry.Windows) == 0 {
		return false, nil
	}
	for _, w := range entry.Windows {
		removeWindow(s, w.ID)
		delete(s.Windows, w.ID)
		r.changes.window(w.ID)
		r.changes.tab(w.TabID)
	}
	entry.Windows = slices.DeleteFunc(entry.Windows, func(w Window) bool {
		if w.App != newTabApp {
			return false
		}
		for _, tab := range entry.Tabs {
			if tab.ID == w.TabID && len(nodeWindowIDs(tab.Root)) == 1 {
				return true
			}
		}
		return false
	})
	if len(entry.Windows) > 0 {
		pushClosedEntry(s, pruneClosedEntry(entry), r.config.ClosedEntryLimit)
	}
	r.changes.desktop(entry.DesktopID)
	return true, nil
}

func (r *reducer) captureClosedEntry(s *Snapshot, c CloseWindowCommand, p windowPlacement) (ClosedEntry, error) {
	d := &s.Desktops[p.desktopIndex]
	tab := d.Tabs[p.tabIndex]
	entry := ClosedEntry{DesktopID: d.ID, ClosedAt: r.now().UTC()}
	if c.Scope == CloseScopeTab {
		ids := nodeWindowIDs(tab.Root)
		if len(ids) == 1 && tab.Pinned {
			return ClosedEntry{}, ErrWindowPinned
		}
		entry.Windows = []Window{cloneWindow(s.Windows[c.WindowID])}
		if len(ids) > 1 {
			entry.Member = captureClosedMember(tab, c.WindowID)
		} else {
			entry.Tabs = []ClosedTab{{ID: tab.ID, Pinned: tab.Pinned, Root: cloneNode(tab.Root), Index: p.tabIndex}}
		}
	} else {
		for i, t := range d.Tabs {
			selected := false
			switch c.Scope {
			case CloseScopeGroup:
				selected = i == p.tabIndex
			case CloseScopeOthers:
				selected = i != p.tabIndex
			case CloseScopeRight:
				selected = i > p.tabIndex
			case CloseScopeLeft:
				selected = i < p.tabIndex
			case CloseScopeAll:
				selected = true
			default:
				return ClosedEntry{}, ErrInvalidCommand
			}
			if !selected || (c.Scope != CloseScopeGroup && t.Pinned) {
				continue
			}
			entry.Tabs = append(entry.Tabs, ClosedTab{ID: t.ID, Pinned: t.Pinned, Root: cloneNode(t.Root), Index: i})
			for _, id := range nodeWindowIDs(t.Root) {
				entry.Windows = append(entry.Windows, cloneWindow(s.Windows[id]))
			}
		}
	}
	return entry, nil
}

func captureClosedMember(tab Tab, id WindowID) *ClosedMember {
	var walk func(LayoutNode) *ClosedMember
	walk = func(n LayoutNode) *ClosedMember {
		for i, child := range n.Children {
			if child.Kind == NodeKindLeaf && child.WindowID != nil && *child.WindowID == id {
				neighborIndex := i + 1
				if neighborIndex >= len(n.Children) {
					neighborIndex = i - 1
				}
				if neighborIndex < 0 {
					return nil
				}
				ids := nodeWindowIDs(n.Children[neighborIndex])
				if len(ids) == 0 {
					return nil
				}
				edge := Edge(DropLeft)
				if n.Axis != nil && *n.Axis == AxisVertical {
					edge = Edge(DropTop)
				}
				if neighborIndex < i {
					edge = Edge(DropRight)
					if n.Axis != nil && *n.Axis == AxisVertical {
						edge = Edge(DropBottom)
					}
				}
				neighbor := ids[0]
				if neighborIndex < i {
					neighbor = ids[len(ids)-1]
				}
				return &ClosedMember{
					TabID:         tab.ID,
					ParentSplitID: n.ID,
					ChildIndex:    i,
					Weight:        n.Weights[i],
					NeighborID:    neighbor,
					NeighborEdge:  edge,
				}
			}
			if m := walk(child); m != nil {
				return m
			}
		}
		return nil
	}
	return walk(tab.Root)
}

func pruneClosedEntry(entry ClosedEntry) ClosedEntry {
	live := make(map[WindowID]Window, len(entry.Windows))
	for _, w := range entry.Windows {
		live[w.ID] = w
	}
	entry.Tabs = slices.DeleteFunc(entry.Tabs, func(t ClosedTab) bool { _, ok := pruneNode(t.Root, live); return !ok })
	for i := range entry.Tabs {
		entry.Tabs[i].Root, _ = pruneNode(entry.Tabs[i].Root, live)
	}
	return entry
}
