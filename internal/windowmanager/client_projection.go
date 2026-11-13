package windowmanager

import "fmt"

type clientViewProjection struct {
	selected *ClientView
	changed  []ClientView
}

func (m *Manager) projectClientViews(
	workspaceID WorkspaceID,
	snapshot Snapshot,
	request CommandRequest,
) (clientViewProjection, error) {
	lifecycle := false
	switch request.Payload.(type) {
	case terminalRetirementCommand, sessionDeletionCommand, workspaceUnregisteredCommand:
		lifecycle = true
	}
	if request.ClientID == nil && request.Payload.CommandID() == CommandWindowNavigate && !lifecycle {
		return clientViewProjection{}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	projection := clientViewProjection{changed: make([]ClientView, 0)}
	for clientID, current := range m.clients[workspaceID] {
		projected := projectDurableClientView(current, snapshot, request, clientID)
		if !clientViewsEqual(current, projected) {
			next, err := nextPresentationRevision(current.PresentationRevision)
			if err != nil {
				return clientViewProjection{}, fmt.Errorf("advance client %q: %w", clientID, err)
			}
			projected.PresentationRevision = next
			if !paletteContextsEqual(current.PaletteContext, projected.PaletteContext) {
				contextRevision, contextErr := nextContextRevision(current.ContextRevision)
				if contextErr != nil {
					return clientViewProjection{}, fmt.Errorf("advance client %q context: %w", clientID, contextErr)
				}
				projected.ContextRevision = contextRevision
			}
			projection.changed = append(projection.changed, cloneClientView(projected))
		}
		if request.ClientID != nil && clientID == *request.ClientID {
			selected := cloneClientView(projected)
			projection.selected = &selected
		}
	}
	return projection, nil
}

func projectDurableClientView(
	view ClientView,
	snapshot Snapshot,
	request CommandRequest,
	clientID ClientID,
) ClientView {
	view = cloneClientView(view)
	issuing := request.ClientID != nil && clientID == *request.ClientID
	if issuing {
		if _, closing := request.Payload.(CloseWindowCommand); closing && len(snapshot.ClosedEntries) > 0 {
			entry := snapshot.ClosedEntries[len(snapshot.ClosedEntries)-1]
			if di, ok := desktopIndexByID(&snapshot, view.ActiveDesktopID); ok {
				d := snapshot.Desktops[di]
				active := view.ActiveTab[d.ID]
				for _, tab := range entry.Tabs {
					if tab.ID == active && len(d.Tabs) > 0 {
						if view.ActiveTab == nil {
							view.ActiveTab = map[DesktopID]TabID{}
						}
						view.ActiveTab[d.ID] = d.Tabs[min(tab.Index, len(d.Tabs)-1)].ID
						view.FocusedWindowID = nil
					}
				}
			}
		}
	}
	view = repairClientView(view, snapshot)
	if issuing {
		view = projectIssuedCommand(view, snapshot, request.Payload)
	}
	return repairClientView(view, snapshot)
}

func projectIssuedCommand(view ClientView, snapshot Snapshot, payload Command) ClientView {
	var id WindowID
	switch c := payload.(type) {
	case reuseWindowCommand:
		id = c.id
	case OpenWindowCommand:
		id = c.Window.ID
		if c.RestoreWindowID != nil {
			id = *c.RestoreWindowID
		}
	case NavigateWindowCommand:
		id = c.WindowID
	case MoveWindowCommand:
		id = c.WindowID
	case ReopenCommand:
		if len(view.FocusOrder) > 0 {
			id = view.FocusOrder[0]
		}
	case ActivateTabCommand:
		if w, ok := snapshot.Windows[c.WindowID]; ok {
			if d, ti, ok := findTab(&snapshot, w.TabID); ok {
				id = tabLeadWindow(view, d.Tabs[ti])
			}
		}
	}
	switch payload.(type) {
	case OpenWindowCommand, MoveWindowCommand:
		if window, exists := snapshot.Windows[id]; exists && window.DesktopID != view.ActiveDesktopID {
			return view
		}
	}
	if id != "" {
		view = projectWindowActivation(view, snapshot, id)
	}
	return view
}

func projectWindowActivation(view ClientView, snapshot Snapshot, id WindowID) ClientView {
	w, ok := snapshot.Windows[id]
	if !ok {
		return view
	}
	view.ActiveDesktopID = w.DesktopID
	view.FocusedWindowID = new(id)
	view.FocusOrder = prependFocus(view.FocusOrder, id)
	if view.ActiveTab == nil {
		view.ActiveTab = map[DesktopID]TabID{}
	}
	view.ActiveTab[w.DesktopID] = w.TabID
	return view
}

func clientActiveTabs(view *ClientView) map[DesktopID]TabID {
	if view == nil {
		return nil
	}
	return view.ActiveTab
}

func (m *Manager) applyClientProjection(
	workspaceID WorkspaceID,
	projection clientViewProjection,
) *ClientView {
	m.mu.Lock()
	for _, view := range projection.changed {
		if workspaceClients := m.clients[workspaceID]; workspaceClients != nil {
			workspaceClients[view.ClientID] = cloneClientView(view)
		}
	}
	m.mu.Unlock()
	if projection.selected == nil {
		return nil
	}
	selected := cloneClientView(*projection.selected)
	return &selected
}

func (m *Manager) previewDurableClient(
	snapshot Snapshot,
	request CommandRequest,
) (*ClientView, error) {
	projection, err := m.projectClientViews(request.WorkspaceID, snapshot, request)
	if err != nil {
		return nil, err
	}
	return projection.selected, nil
}

func repairClientView(view ClientView, snapshot Snapshot) ClientView {
	repaired := make(map[DesktopID]TabID, len(snapshot.Desktops))
	for _, d := range snapshot.Desktops {
		active := view.ActiveTab[d.ID]
		if tabHoldingID(d.Tabs, active) < 0 {
			active = d.ActiveTabID
		}
		if tabHoldingID(d.Tabs, active) < 0 && len(d.Tabs) > 0 {
			active = d.Tabs[0].ID
		}
		repaired[d.ID] = active
	}
	view.ActiveTab = repaired
	view.StackActive = map[NodeID]WindowID{}
	if _, ok := desktopIndexByID(&snapshot, view.ActiveDesktopID); !ok {
		view.ActiveDesktopID = defaultDesktopID(snapshot)
	}
	filtered := make([]WindowID, 0, len(view.FocusOrder))
	for _, id := range view.FocusOrder {
		if _, ok := snapshot.Windows[id]; ok && !containsWindowID(filtered, id) {
			filtered = append(filtered, id)
		}
	}
	view.FocusOrder = filtered
	if view.FocusedWindowID != nil {
		w, ok := snapshot.Windows[*view.FocusedWindowID]
		if !ok || w.DesktopID != view.ActiveDesktopID || w.TabID != view.ActiveTab[w.DesktopID] {
			view.FocusedWindowID = nil
		}
	}
	if view.FocusedWindowID == nil {
		view.FocusedWindowID = focusForDesktop(view, snapshot)
	}
	view.PaletteContext = derivePaletteContext(view, snapshot)
	return view
}

func tabLeadWindow(view ClientView, tab Tab) WindowID {
	ids := nodeWindowIDs(tab.Root)
	for _, id := range view.FocusOrder {
		if containsWindowID(ids, id) {
			return id
		}
	}
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func focusForDesktop(view ClientView, snapshot Snapshot) *WindowID {
	if d, ti, ok := findTab(&snapshot, view.ActiveTab[view.ActiveDesktopID]); ok && d.ID == view.ActiveDesktopID {
		id := tabLeadWindow(view, d.Tabs[ti])
		if id != "" {
			return new(id)
		}
	}
	return nil
}

func firstVisibleWindow(view ClientView, snapshot Snapshot) WindowID {
	di, ok := desktopIndexByID(&snapshot, view.ActiveDesktopID)
	if !ok {
		return ""
	}
	d := snapshot.Desktops[di]
	ti := tabHoldingID(d.Tabs, view.ActiveTab[d.ID])
	if ti < 0 {
		return ""
	}
	return firstVisibleNodeWindow(d.Tabs[ti].Root)
}

func firstVisibleNodeWindow(node LayoutNode) WindowID {
	ids := nodeWindowIDs(node)
	if len(ids) > 0 {
		return ids[0]
	}
	return ""
}

func prependFocus(order []WindowID, windowID WindowID) []WindowID {
	result := []WindowID{windowID}
	for _, candidate := range order {
		if candidate != windowID {
			result = append(result, candidate)
		}
	}
	return result
}

func clientViewsEqual(left, right ClientView) bool {
	if left.Kind != right.Kind ||
		left.ActiveDesktopID != right.ActiveDesktopID ||
		valueOrZero(left.FocusedWindowID) != valueOrZero(right.FocusedWindowID) ||
		len(left.FocusOrder) != len(right.FocusOrder) ||
		!paletteContextsEqual(left.PaletteContext, right.PaletteContext) ||
		!globalShortcutRegistrationsEqual(left.GlobalShortcuts, right.GlobalShortcuts) {
		return false
	}
	if len(left.ActiveTab) != len(right.ActiveTab) {
		return false
	}
	for desktopID, tabID := range left.ActiveTab {
		if right.ActiveTab[desktopID] != tabID {
			return false
		}
	}
	for index := range left.FocusOrder {
		if left.FocusOrder[index] != right.FocusOrder[index] {
			return false
		}
	}
	return true
}

func defaultDesktopID(snapshot Snapshot) DesktopID {
	if len(snapshot.Desktops) > 0 {
		return snapshot.Desktops[0].ID
	}
	return ""
}

func resolvedProjectionRequest(before, after Snapshot, request CommandRequest) CommandRequest {
	var id WindowID
	switch c := request.Payload.(type) {
	case OpenWindowCommand:
		if c.Window.ID != "" || c.RestoreWindowID != nil {
			return request
		}
		for _, candidate := range sortedWindowIDs(after.Windows) {
			if _, existed := before.Windows[candidate]; !existed && after.Windows[candidate].App == c.Window.App {
				id = candidate
				break
			}
		}
	case ReopenCommand:
		if len(before.ClosedEntries) > 0 {
			entry := before.ClosedEntries[len(before.ClosedEntries)-1]
			if len(entry.Windows) > 0 {
				id = entry.Windows[0].ID
			}
		}
	}
	if id != "" {
		if command, opening := request.Payload.(OpenWindowCommand); opening {
			command.Window.ID = id
			request.Payload = command
		} else {
			request.Payload = NavigateWindowCommand{WindowID: id}
		}
	}
	return request
}

func explicitClientActivation(snapshot Snapshot, client *ClientView, payload Command) (ChangeSet, bool) {
	command, ok := payload.(ActivateTabCommand)
	if !ok || client == nil {
		return ChangeSet{}, false
	}
	window, ok := snapshot.Windows[command.WindowID]
	if !ok || (client.ActiveDesktopID == window.DesktopID && client.ActiveTab[window.DesktopID] == window.TabID) {
		return ChangeSet{}, false
	}
	return ChangeSet{
		ClientIDs:  []ClientID{client.ClientID},
		DesktopIDs: []DesktopID{window.DesktopID},
		TabIDs:     []TabID{window.TabID},
	}, true
}
