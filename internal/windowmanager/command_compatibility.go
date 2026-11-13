package windowmanager

type reuseWindowCommand struct{ id WindowID }

func (reuseWindowCommand) CommandID() CommandID { return CommandWindowOpen }

type adjacentTabsCommand struct {
	id      CommandID
	target  *WindowID
	desktop DesktopID
	ids     []WindowID
}

func (c adjacentTabsCommand) CommandID() CommandID { return c.id }

// canonicalCommandSurfaces keeps the v0.6 request vocabulary through v0.7; remove in v0.8.0.
func canonicalCommandSurfaces(request CommandRequest) (CommandRequest, []Deprecation) {
	request.CommandID = request.Payload.CommandID()
	deprecations := []Deprecation{}
	report := func(surface string) {
		deprecations = append(deprecations, SurfaceDeprecation(surface))
	}
	switch c := request.Payload.(type) {
	case ToggleFloatingCommand, ZoomWindowCommand, ResizeWindowCommand, FrameResizeLayoutCommand:
		report(string(c.CommandID()))
		request.Payload = nil
	case CloseWindowCommand:
		if c.Minimize {
			report("window.close.minimize")
			request.Payload = nil
		}
	case OpenWindowCommand:
		request.Payload = canonicalOpenCommand(c, report)
	case MoveWindowCommand:
		request.Payload = canonicalMoveCommand(c, report)
	case ReorderStackCommand:
		report(string(c.CommandID()))
		request.Payload = ReorderTabCommand(c)
	case SetStackActiveCommand:
		report(string(c.CommandID()))
		request.Payload = ActivateTabCommand(c)
	case GroupWindowsCommand:
		report(string(c.CommandID()))
		request.Payload = adjacentTabsCommand{id: c.CommandID(), target: new(c.TargetWindowID), ids: c.WindowIDs}
	case ArrangeLayoutCommand:
		request.Payload = canonicalArrangeCommand(c, report)
	case ReplaceLayoutCommand:
		deprecations = append(deprecations, c.Document.Overrides.Deprecations()...)
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
	if request.Payload != nil {
		request.CommandID = request.Payload.CommandID()
	}
	return request, deprecations
}

// canonicalProjectionSurfaces translates restore activation without a durable focus command; remove in v0.8.0.
func canonicalProjectionSurfaces(request CommandRequest) CommandRequest {
	if c, reuse := request.Payload.(reuseWindowCommand); reuse {
		request.Payload = FocusWindowCommand{WindowID: new(c.id)}
	}
	return request
}

// reduceCommandSurfaces resolves v0.6 shims before the v6 reducer; remove in v0.8.0.
func reduceCommandSurfaces(s *Snapshot, command Command, r *reducer) (reduction, error) {
	switch c := command.(type) {
	case reuseWindowCommand:
		if _, ok := s.Windows[c.id]; !ok {
			return reduction{}, ErrWindowNotFound
		}
		return reduction{}, nil
	case nil:
		return reduction{}, nil
	case adjacentTabsCommand:
		beforeMembers := tabMemberCounts(s)
		changed, err := placeAdjacentTabs(s, c, r)
		if err != nil {
			return reduction{}, err
		}
		return r.finishReduction(s, beforeMembers, changed)
	default:
		return r.reduce(s, command)
	}
}

func placeAdjacentTabs(s *Snapshot, c adjacentTabsCommand, r *reducer) (bool, error) {
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
		r.placeTab(s, d, tab, index)
		r.changes.window(id)
		r.changes.tab(source)
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
	if c.RestoreWindowID != nil && *c.RestoreWindowID != "" {
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
	switch c.Placement {
	case DropFloating:
		report("window.move.placement=floating")
		c.Placement = DropTab
	case "":
		report("window.move.placement=empty")
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
		payload = adjacentTabsCommand{id: c.CommandID(), desktop: c.DesktopID, ids: c.WindowIDs}
	} else {
		payload = c
	}
	return payload
}
