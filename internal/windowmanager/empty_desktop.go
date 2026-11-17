package windowmanager

const (
	newTabApp  = "new-tab"
	newTabPath = "/new-tab"
)

func (r *reducer) ensureDesktopTabs(s *Snapshot) error {
	for i := range s.Desktops {
		d := &s.Desktops[i]
		if len(d.Tabs) > 0 {
			continue
		}
		window, err := r.newOpenWindow(
			s,
			WindowSpec{
				App:       newTabApp,
				DesktopID: d.ID,
				Route:     RouteIntent{Pathname: newTabPath, Search: RouteSearch{}},
			},
		)
		if err != nil {
			return err
		}
		tab, err := r.newTab(window.ID)
		if err != nil {
			return err
		}
		window.TabID = tab.ID
		s.Windows[window.ID] = window
		d.Tabs = []Tab{tab}
		d.ActiveTabID = tab.ID
		r.changes.desktop(d.ID)
		r.changes.window(window.ID)
		r.changes.tab(tab.ID)
		r.changes.node(tab.Root.ID)
	}
	return nil
}

// emptyDesktopPlaceholders names, per desktop that holds only New Tab placeholders, those
// placeholder windows.
func emptyDesktopPlaceholders(s *Snapshot) map[DesktopID][]WindowID {
	empty := make(map[DesktopID][]WindowID)
	for _, d := range s.Desktops {
		if !desktopEmpty(s, d) {
			continue
		}
		ids := []WindowID{}
		for _, tab := range d.Tabs {
			ids = append(ids, nodeWindowIDs(tab.Root)...)
		}
		empty[d.ID] = ids
	}
	return empty
}

// dropSupersededPlaceholders removes the New Tab placeholders of a desktop that was empty before
// this command and now holds real content, whether the content landed as a tab or split beside
// one: a placeholder only stands in for an empty desktop, so the first real window replaces it
// (#1799). A New Tab this command opened itself is not a stand-in and stays.
func (r *reducer) dropSupersededPlaceholders(s *Snapshot, emptyBefore map[DesktopID][]WindowID) {
	for i := range s.Desktops {
		d := &s.Desktops[i]
		placeholders, wasEmpty := emptyBefore[d.ID]
		if !wasEmpty || desktopEmpty(s, *d) {
			continue
		}
		for _, id := range placeholders {
			w, ok := s.Windows[id]
			if !ok || w.App != newTabApp || w.DesktopID != d.ID {
				continue
			}
			removeWindow(s, id)
			delete(s.Windows, id)
			r.changes.window(id)
			r.changes.tab(w.TabID)
			r.changes.desktop(d.ID)
		}
	}
}

// DefaultSnapshot is the stable empty-workspace topology before the first commit.
func DefaultSnapshot(id WorkspaceID) Snapshot {
	windowID := WindowID("win-newtab-desktop-default")
	tabID := TabID("tab-" + string(windowID))
	return Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: id,
		Desktops: []Desktop{
			{
				ID:   "desktop-default",
				Name: defaultDesktopName,
				Tabs: []Tab{
					{
						ID: tabID,
						Root: LayoutNode{
							ID:       NodeID("leaf-" + string(windowID)),
							Kind:     NodeKindLeaf,
							WindowID: new(windowID),
						},
					},
				},
				ActiveTabID: tabID,
			},
		},
		Windows: map[WindowID]Window{
			windowID: {
				ID:        windowID,
				App:       newTabApp,
				Route:     RouteIntent{Pathname: newTabPath, Search: RouteSearch{}},
				NavStack:  []RouteIntent{},
				DesktopID: "desktop-default",
				TabID:     tabID,
			},
		},
		History: History{Undo: []HistoryEntry{}, Redo: []HistoryEntry{}},
	}
}

// Reconciliation allocates deterministic repair IDs so retries preserve the same topology.
func repairEmptyDesktops(s *Snapshot) {
	used := snapshotConverter{used: map[string]bool{}}
	for id := range s.Windows {
		used.used[string(id)] = true
	}
	for _, entry := range s.ClosedEntries {
		for _, w := range entry.Windows {
			used.used[string(w.ID)] = true
		}
	}
	var reserve func(LayoutNode)
	reserve = func(n LayoutNode) {
		used.used[string(n.ID)] = true
		for _, child := range n.Children {
			reserve(child)
		}
	}
	for _, d := range s.Desktops {
		for _, tab := range d.Tabs {
			used.used[string(tab.ID)] = true
			reserve(tab.Root)
		}
	}
	for i := range s.Desktops {
		d := &s.Desktops[i]
		if len(d.Tabs) > 0 {
			continue
		}
		id := WindowID(used.allocate("win-newtab-" + string(d.ID)))
		tab := Tab{
			ID:   TabID(used.allocate("tab-" + string(id))),
			Root: LayoutNode{ID: NodeID(used.allocate("leaf-" + string(id))), Kind: NodeKindLeaf, WindowID: new(id)},
		}
		s.Windows[id] = Window{
			ID:        id,
			App:       newTabApp,
			Route:     RouteIntent{Pathname: newTabPath, Search: RouteSearch{}},
			NavStack:  []RouteIntent{},
			DesktopID: d.ID,
			TabID:     tab.ID,
		}
		d.Tabs, d.ActiveTabID = []Tab{tab}, tab.ID
	}
}
