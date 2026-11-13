package windowmanager

import "slices"

func (r *reducer) reorderTab(s *Snapshot, c ReorderTabCommand) (bool, error) {
	p, ok := findWindowPlacement(s, c.WindowID)
	if !ok {
		return false, ErrWindowNotFound
	}
	d := &s.Desktops[p.desktopIndex]
	tab := d.Tabs[p.tabIndex]
	before := append([]TabID(nil), tabIDs(d.Tabs)...)
	d.Tabs = slices.Delete(d.Tabs, p.tabIndex, p.tabIndex+1)
	insertTab(d, tab, c.Index)
	if slices.Equal(before, tabIDs(d.Tabs)) {
		return false, nil
	}
	r.changes.desktop(d.ID)
	r.changes.tab(tab.ID)
	return true, nil
}

func tabIDs(tabs []Tab) []TabID {
	ids := make([]TabID, len(tabs))
	for i, t := range tabs {
		ids[i] = t.ID
	}
	return ids
}

func (r *reducer) activateTab(s *Snapshot, c ActivateTabCommand) (bool, error) {
	p, ok := findWindowPlacement(s, c.WindowID)
	if !ok {
		return false, ErrWindowNotFound
	}
	d := &s.Desktops[p.desktopIndex]
	tab := d.Tabs[p.tabIndex]
	if d.ActiveTabID == tab.ID {
		return false, nil
	}
	d.ActiveTabID = tab.ID
	r.changes.desktop(d.ID)
	r.changes.tab(tab.ID)
	return true, nil
}

func (r *reducer) pinWindow(s *Snapshot, c PinWindowCommand) (bool, error) {
	p, ok := findWindowPlacement(s, c.WindowID)
	if !ok {
		return false, ErrWindowNotFound
	}
	d := &s.Desktops[p.desktopIndex]
	tab := d.Tabs[p.tabIndex]
	if tab.Pinned == c.Pinned {
		return false, nil
	}
	d.Tabs = slices.Delete(d.Tabs, p.tabIndex, p.tabIndex+1)
	tab.Pinned = c.Pinned
	pins := 0
	for _, t := range d.Tabs {
		if t.Pinned {
			pins++
		}
	}
	insertTab(d, tab, pins)
	r.changes.tab(tab.ID)
	r.changes.desktop(d.ID)
	return true, nil
}

func (r *reducer) placeTab(s *Snapshot, d *Desktop, tab Tab, index int) {
	insertTab(d, tab, index)
	assignTabWindows(s, d, tab)
	d.ActiveTabID = tab.ID
	r.changes.tab(tab.ID)
}
