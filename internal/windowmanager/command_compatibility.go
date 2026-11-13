package windowmanager

import "fmt"

type noTopologyCommand struct{ id CommandID }

func (c noTopologyCommand) CommandID() CommandID { return c.id }

type reuseWindowCommand struct{ id WindowID }

func (reuseWindowCommand) CommandID() CommandID { return CommandWindowOpen }

type adjacentTabsCommand struct {
	target  *WindowID
	desktop DesktopID
	ids     []WindowID
}

func (adjacentTabsCommand) CommandID() CommandID { return CommandWindowStackGroup }

// canonicalCommandSurfaces keeps the v0.6 request vocabulary through v0.7; remove in v0.8.0.
func canonicalCommandSurfaces(request CommandRequest) (CommandRequest, []Deprecation) {
	deprecations := []Deprecation{}
	report := func(surface string) {
		deprecations = append(
			deprecations,
			Deprecation{
				Code:           "window_manager_surface_deprecated",
				Surface:        surface,
				RemovalRelease: "v0.8.0",
				Message:        fmt.Sprintf("%s is deprecated and will be removed in v0.8.0.", surface),
			},
		)
	}
	switch c := request.Payload.(type) {
	case ToggleFloatingCommand, ZoomWindowCommand, ResizeWindowCommand, FrameResizeLayoutCommand:
		report(string(c.CommandID()))
		request.Payload = noTopologyCommand{id: c.CommandID()}
	case CloseWindowCommand:
		if c.Minimize {
			report("window.close.minimize")
			request.Payload = noTopologyCommand{id: c.CommandID()}
		}
	case OpenWindowCommand:
		request.Payload = canonicalOpenCommand(c, report)
	case MoveWindowCommand:
		request.Payload = canonicalMoveCommand(c, report)
	case ReorderStackCommand:
		report(string(c.CommandID()))
		request.Payload = ReorderTabCommand{WindowID: c.WindowID, Index: c.Index}
	case SetStackActiveCommand:
		report(string(c.CommandID()))
		request.Payload = ActivateTabCommand{WindowID: c.WindowID}
	case GroupWindowsCommand:
		report(string(c.CommandID()))
		request.Payload = adjacentTabsCommand{target: new(c.TargetWindowID), ids: c.WindowIDs}
	case ArrangeLayoutCommand:
		request.Payload = canonicalArrangeCommand(c, report)
	case BalanceLayoutCommand:
		if c.GroupID != nil && *c.GroupID != "" {
			report("layout.balance.group_id")
			if c.TabID == nil {
				c.TabID = new(TabID(*c.GroupID))
			}
		}
		c.GroupID = nil
		request.Payload = c
	}
	request.CommandID = request.Payload.CommandID()
	return request, deprecations
}

func (r *reducer) placeAdjacentTabs(s *Snapshot, c adjacentTabsCommand) (bool, error) {
	if len(c.ids) == 0 {
		return false, ErrInvalidCommand
	}
	var desktop DesktopID
	var after *WindowID
	if c.target != nil {
		w, ok := s.Windows[*c.target]
		if !ok {
			return false, ErrWindowNotFound
		}
		desktop = w.DesktopID
		after = c.target
	} else {
		desktop = c.desktop
	}
	di, ok := desktopIndexByID(s, desktop)
	if !ok {
		return false, ErrDesktopNotFound
	}
	seen := map[WindowID]bool{}
	for _, id := range c.ids {
		w, exists := s.Windows[id]
		if !exists {
			return false, ErrWindowNotFound
		}
		if seen[id] || (after != nil && id == *after) || w.DesktopID != desktop {
			return false, ErrInvalidCommand
		}
		seen[id] = true
	}
	changed := false
	for _, id := range c.ids {
		w := s.Windows[id]
		source := w.TabID
		removeWindow(s, id)
		tab, err := r.newTab(id)
		if err != nil {
			return false, err
		}
		d := &s.Desktops[di]
		index := len(d.Tabs)
		if after != nil {
			index = tabHoldingID(d.Tabs, s.Windows[*after].TabID) + 1
		}
		insertTab(d, tab, index)
		assignTabWindows(s, d, tab)
		d.ActiveTabID = tab.ID
		r.changes.window(id)
		r.changes.tab(source)
		r.changes.tab(tab.ID)
		after = new(id)
		changed = true
	}
	r.changes.desktop(desktop)
	return changed, nil
}

func canonicalOpenCommand(c OpenWindowCommand, report func(string)) Command {
	var payload Command
	if c.Window.Floating {
		report("window.open.window.floating")
	}
	if c.Window.FloatingRect != (NormalizedRect{}) {
		report("window.open.window.floating_rect")
	}
	if c.RestoreWindowID != nil {
		report("window.open.restore_window_id")
	}
	if c.Window.Floating {
		c.Window.InsertTiled = false
		c.policy = NewWindowTab
	}
	c.Window.Floating = false
	c.Window.FloatingRect = NormalizedRect{}
	payload = c
	if c.RestoreWindowID != nil {
		payload = reuseWindowCommand{id: *c.RestoreWindowID}
	}
	return payload
}

func canonicalMoveCommand(c MoveWindowCommand, report func(string)) Command {
	var payload Command
	if c.Placement == DropFloating || c.Placement == "" {
		report("window.move.placement=floating")
		c.Placement = DropTab
	}
	if c.FloatingRect != nil && *c.FloatingRect != (NormalizedRect{}) {
		report("window.move.floating_rect")
	}
	c.FloatingRect = nil
	payload = c
	return payload
}

func canonicalArrangeCommand(c ArrangeLayoutCommand, report func(string)) Command {
	var payload Command
	if c.Frame != (NormalizedRect{}) {
		report("layout.arrange.frame")
	}
	if c.GroupID != "" {
		report("layout.arrange.group_id")
	}
	if c.KeepFrames {
		report("layout.arrange.keep_frames")
	}
	c.Frame = NormalizedRect{}
	c.GroupID = ""
	c.KeepFrames = false
	if c.Arrangement == ArrangementStack {
		report("layout.arrange.arrangement=stack")
		payload = adjacentTabsCommand{desktop: c.DesktopID, ids: c.WindowIDs}
	} else {
		payload = c
	}
	return payload
}
