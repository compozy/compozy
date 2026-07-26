package windowmanager

import "fmt"

type clientViewProjection struct {
	selected *ClientView
	changed  []ClientView
}

func (m *Manager) projectClientViews(
	workspaceID WorkspaceID,
	previous Snapshot,
	snapshot Snapshot,
	request CommandRequest,
) (clientViewProjection, error) {
	if request.ClientID == nil && request.Payload.CommandID() == CommandWindowNavigate {
		return clientViewProjection{}, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	projection := clientViewProjection{changed: make([]ClientView, 0)}
	for clientID, current := range m.clients[workspaceID] {
		projected := projectDurableClientView(current, previous, snapshot, request, clientID)
		if !clientViewsEqual(current, projected) {
			next, err := nextPresentationRevision(current.PresentationRevision)
			if err != nil {
				return clientViewProjection{}, fmt.Errorf("advance client %q: %w", clientID, err)
			}
			projected.PresentationRevision = next
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
	previous Snapshot,
	snapshot Snapshot,
	request CommandRequest,
	clientID ClientID,
) ClientView {
	// The departing-zoom follow needs the active desktop from before generic repair re-homes it.
	originalActiveDesktopID := view.ActiveDesktopID
	view = repairClientView(view, snapshot)
	if request.ClientID != nil && clientID == *request.ClientID {
		switch command := request.Payload.(type) {
		case ZoomWindowCommand:
			if window, exists := snapshot.Windows[command.WindowID]; exists {
				view.ActiveDesktopID = window.DesktopID
				view.FocusedWindowID = &command.WindowID
				view.FocusOrder = prependFocus(view.FocusOrder, command.WindowID)
			}
		case CloseWindowCommand:
			view = followDepartingZoomOwner(view, previous, snapshot, command, originalActiveDesktopID)
		case OpenWindowCommand:
			windowID := command.Window.ID
			if command.RestoreWindowID != nil {
				windowID = *command.RestoreWindowID
			}
			if window, exists := snapshot.Windows[windowID]; exists && !window.Minimized {
				// Restoring a minimized window activates its desktop.
				if command.RestoreWindowID != nil {
					view.ActiveDesktopID = window.DesktopID
				}
				if window.DesktopID == view.ActiveDesktopID {
					view.FocusedWindowID = &windowID
					view.FocusOrder = prependFocus(view.FocusOrder, windowID)
				}
			}
		case NavigateWindowCommand:
			if window, exists := snapshot.Windows[command.WindowID]; exists && !window.Minimized {
				view.ActiveDesktopID = window.DesktopID
				view.FocusedWindowID = &command.WindowID
				view.FocusOrder = prependFocus(view.FocusOrder, command.WindowID)
			}
		}
	}
	return repairClientView(view, snapshot)
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
	previous Snapshot,
	snapshot Snapshot,
	request CommandRequest,
) (*ClientView, error) {
	projection, err := m.projectClientViews(request.WorkspaceID, previous, snapshot, request)
	if err != nil {
		return nil, err
	}
	return projection.selected, nil
}

// followDepartingZoomOwner keeps the issuing client with a zoomed window when it leaves its focus desktop.
func followDepartingZoomOwner(
	view ClientView,
	previous Snapshot,
	snapshot Snapshot,
	command CloseWindowCommand,
	originalActiveDesktopID DesktopID,
) ClientView {
	previousWindow, existed := previous.Windows[command.WindowID]
	if !existed || previousWindow.DesktopID != originalActiveDesktopID {
		return view
	}
	if _, owned := ownedFocusDesktopIndex(&previous, command.WindowID); !owned {
		return view
	}
	if command.Minimize {
		if window, exists := snapshot.Windows[command.WindowID]; exists &&
			window.DesktopID != originalActiveDesktopID {
			view.ActiveDesktopID = window.DesktopID
		}
		return view
	}
	if previousWindow.ReturnAnchor == nil {
		return view
	}
	if _, exists := desktopIndexByID(&snapshot, previousWindow.ReturnAnchor.DesktopID); exists {
		view.ActiveDesktopID = previousWindow.ReturnAnchor.DesktopID
	}
	return view
}

func repairClientView(view ClientView, snapshot Snapshot) ClientView {
	if _, exists := desktopIndexByID(&snapshot, view.ActiveDesktopID); !exists {
		view.ActiveDesktopID = defaultDesktopID(snapshot)
	}
	filtered := make([]WindowID, 0, len(view.FocusOrder))
	for _, windowID := range view.FocusOrder {
		window, exists := snapshot.Windows[windowID]
		if exists && window.DesktopID == view.ActiveDesktopID && !window.Minimized &&
			!containsWindowID(filtered, windowID) {
			filtered = append(filtered, windowID)
		}
	}
	view.FocusOrder = filtered
	if view.FocusedWindowID != nil {
		window, exists := snapshot.Windows[*view.FocusedWindowID]
		if !exists || window.DesktopID != view.ActiveDesktopID || window.Minimized {
			view.FocusedWindowID = nil
		}
	}
	if view.FocusedWindowID == nil {
		focused := focusForDesktop(view, snapshot)
		view.FocusedWindowID = focused
		if focused != nil {
			view.FocusOrder = prependFocus(view.FocusOrder, *focused)
		}
	}
	return view
}

func focusForDesktop(view ClientView, snapshot Snapshot) *WindowID {
	for _, windowID := range view.FocusOrder {
		if window, exists := snapshot.Windows[windowID]; exists &&
			window.DesktopID == view.ActiveDesktopID &&
			!window.Minimized {
			return &windowID
		}
	}
	windowID := firstVisibleWindow(snapshot, view.ActiveDesktopID)
	if windowID == "" {
		return nil
	}
	return &windowID
}

func firstVisibleWindow(snapshot Snapshot, desktopID DesktopID) WindowID {
	desktopIndex, exists := desktopIndexByID(&snapshot, desktopID)
	if !exists {
		return ""
	}
	for _, group := range snapshot.Desktops[desktopIndex].Groups {
		for _, windowID := range nodeWindowIDs(group.Root) {
			if !snapshot.Windows[windowID].Minimized {
				return windowID
			}
		}
	}
	for _, windowID := range snapshot.Desktops[desktopIndex].Floating {
		if !snapshot.Windows[windowID].Minimized {
			return windowID
		}
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
	if left.ActiveDesktopID != right.ActiveDesktopID ||
		valueOrZero(left.FocusedWindowID) != valueOrZero(right.FocusedWindowID) ||
		len(left.FocusOrder) != len(right.FocusOrder) {
		return false
	}
	for index := range left.FocusOrder {
		if left.FocusOrder[index] != right.FocusOrder[index] {
			return false
		}
	}
	return true
}

func defaultDesktopID(snapshot Snapshot) DesktopID {
	for _, desktop := range snapshot.Desktops {
		if desktop.Purpose == DesktopPurposeStandard {
			return desktop.ID
		}
	}
	if len(snapshot.Desktops) > 0 {
		return snapshot.Desktops[0].ID
	}
	return ""
}
