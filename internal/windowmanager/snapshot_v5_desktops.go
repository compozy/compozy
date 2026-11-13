package windowmanager

import (
	"cmp"
	"fmt"
	"slices"
)

func (c *snapshotConverter) convertDesktops(legacy SnapshotV5, out *Snapshot) error {
	mergedZoom := map[DesktopID]DesktopID{}
	zoomOwners := map[DesktopID]WindowID{}
	for _, d := range legacy.Desktops {
		if owner, target, ok := legacyZoomTarget(d, legacy); ok {
			mergedZoom[d.ID], zoomOwners[d.ID] = target, owner
		}
	}
	resolveZoomTargets(mergedZoom)
	for _, d := range legacy.Desktops {
		if _, merged := mergedZoom[d.ID]; merged {
			continue
		}
		converted := Desktop{ID: d.ID, Name: d.Name, Order: len(out.Desktops), Tabs: []Tab{}}
		c.convertGroups(d, &converted)
		c.appendZoomTabs(legacy, d.ID, mergedZoom, zoomOwners, &converted)
		c.appendFloatingTabs(d, &converted)
		if err := c.finishDesktop(&converted, out); err != nil {
			return err
		}
		out.Desktops = append(out.Desktops, converted)
	}
	return nil
}

func (c *snapshotConverter) convertGroups(d DesktopV5, converted *Desktop) {
	groups := slices.Clone(d.Groups)
	slices.SortStableFunc(groups, func(a, b LayoutGroupV5) int {
		if order := cmp.Compare(a.Frame.X, b.Frame.X); order != 0 {
			return order
		}
		return cmp.Compare(a.Frame.Y, b.Frame.Y)
	})
	if len(groups) > 1 {
		if root, ok := c.mergeIslands(d.ID, groups); ok {
			converted.Tabs = append(converted.Tabs, c.treeTabs(TabID(groups[0].ID), root)...)
			c.summary.IslandsMerged += len(groups) - 1
		} else {
			for _, g := range groups {
				converted.Tabs = append(converted.Tabs, c.treeTabs(TabID(g.ID), g.Root)...)
			}
		}
	} else {
		for _, g := range groups {
			converted.Tabs = append(converted.Tabs, c.rootTabs(TabID(g.ID), g.Root)...)
		}
	}
	if len(groups) == 1 && groups[0].Root.Kind == legacyNodeKindStack && groups[0].Root.ActiveID != nil {
		converted.ActiveTabID = tabHolding(converted.Tabs, *groups[0].Root.ActiveID)
	}
}

func (c *snapshotConverter) appendZoomTabs(
	legacy SnapshotV5,
	destination DesktopID,
	mergedZoom map[DesktopID]DesktopID,
	zoomOwners map[DesktopID]WindowID,
	converted *Desktop,
) {
	for _, source := range legacy.Desktops {
		if mergedZoom[source.ID] != destination {
			continue
		}
		for _, g := range source.Groups {
			converted.Tabs = append(converted.Tabs, c.treeTabs(TabID(g.ID), g.Root)...)
		}
		for _, stack := range source.FloatingStacks {
			for _, id := range stack.WindowIDs {
				converted.Tabs = append(converted.Tabs, c.solo(id))
			}
		}
		for _, id := range source.Floating {
			converted.Tabs = append(converted.Tabs, c.solo(id))
		}
		converted.ActiveTabID = tabHolding(converted.Tabs, zoomOwners[source.ID])
		c.summary.ZoomDesktopsMerged++
	}
}

func (c *snapshotConverter) appendFloatingTabs(d DesktopV5, converted *Desktop) {
	for _, stack := range d.FloatingStacks {
		for _, id := range stack.WindowIDs {
			converted.Tabs = append(converted.Tabs, c.solo(id))
			c.summary.FloatingToTabs++
		}
	}
	for _, id := range d.Floating {
		converted.Tabs = append(converted.Tabs, c.solo(id))
		if c.windows[id].Minimized {
			c.summary.MinimizedToTabs++
		} else {
			c.summary.FloatingToTabs++
		}
	}
	if len(converted.Tabs) == 0 {
		id := WindowID(c.allocate("win-newtab-" + string(d.ID)))
		c.windows[id] = WindowV5{
			ID:        id,
			App:       newTabApp,
			DesktopID: d.ID,
			Route:     RouteIntent{Pathname: newTabPath, Search: RouteSearch{}},
			NavStack:  []RouteIntent{},
		}
		converted.Tabs = append(converted.Tabs, c.solo(id))
	}
}

func (c *snapshotConverter) finishDesktop(converted *Desktop, out *Snapshot) error {
	for i := range converted.Tabs {
		tab := &converted.Tabs[i]
		ids := nodeWindowIDs(tab.Root)
		if len(ids) == 1 {
			tab.Pinned = c.windows[ids[0]].Pinned
		} else {
			for _, id := range ids {
				if c.windows[id].Pinned {
					c.summary.PinsDropped++
				}
			}
		}
		for _, id := range ids {
			old, exists := c.windows[id]
			if !exists {
				return fmt.Errorf("missing legacy window %q: %w", id, ErrInvalidTopology)
			}
			if _, duplicate := out.Windows[id]; duplicate {
				return fmt.Errorf("duplicate legacy window %q: %w", id, ErrInvalidTopology)
			}
			out.Windows[id] = currentWindow(old, converted.ID, tab.ID)
		}
	}
	slices.SortStableFunc(converted.Tabs, func(a, b Tab) int {
		if a.Pinned == b.Pinned {
			return 0
		}
		if a.Pinned {
			return -1
		}
		return 1
	})
	if converted.ActiveTabID == "" {
		converted.ActiveTabID = converted.Tabs[0].ID
	}
	return nil
}
