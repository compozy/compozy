package windowmanager

import (
	"context"
	"fmt"
	"sort"
)

// RegisterClient creates or refreshes one explicit client-local view.
func (m *Manager) RegisterClient(ctx context.Context, registration ClientRegistration) (ClientView, error) {
	if err := m.resolveWorkspace(ctx, registration.WorkspaceID); err != nil {
		return ClientView{}, err
	}
	lock, err := m.lockFor(registration.WorkspaceID)
	if err != nil {
		return ClientView{}, err
	}
	lock.Lock()
	defer lock.Unlock()
	snapshot, err := m.loadSnapshot(ctx, registration.WorkspaceID)
	if err != nil {
		return ClientView{}, err
	}
	clientID := registration.ClientID
	if clientID == "" {
		generated, generateErr := m.generate("client")
		if generateErr != nil {
			return ClientView{}, fmt.Errorf("generate client ID: %w", generateErr)
		}
		clientID = ClientID(generated)
	}
	if registration.ActiveDesktopID != "" {
		if _, exists := desktopIndexByID(&snapshot, registration.ActiveDesktopID); !exists {
			return ClientView{}, fmt.Errorf(
				"desktop %q: %w",
				registration.ActiveDesktopID,
				ErrDesktopNotFound,
			)
		}
	}
	m.mu.Lock()
	workspaceClients := m.clients[registration.WorkspaceID]
	if workspaceClients == nil {
		workspaceClients = make(map[ClientID]ClientView)
		m.clients[registration.WorkspaceID] = workspaceClients
	}
	view, exists := workspaceClients[clientID]
	var before ClientView
	if !exists {
		view = ClientView{
			WorkspaceID:          registration.WorkspaceID,
			ClientID:             clientID,
			PresentationRevision: 1,
			ConnectedAt:          m.now().UTC(),
			FocusOrder:           []WindowID{},
		}
	} else {
		before = cloneClientView(view)
	}
	activeID := registration.ActiveDesktopID
	if activeID == "" {
		if exists {
			activeID = view.ActiveDesktopID
		} else {
			activeID = defaultDesktopID(snapshot)
		}
	}
	view.ActiveDesktopID = activeID
	view = repairClientView(view, snapshot)
	changed := !exists || !clientViewsEqual(before, view)
	if exists && changed {
		next, revisionErr := nextPresentationRevision(before.PresentationRevision)
		if revisionErr != nil {
			m.mu.Unlock()
			return ClientView{}, fmt.Errorf("advance client %q: %w", clientID, revisionErr)
		}
		view.PresentationRevision = next
	}
	workspaceClients[clientID] = cloneClientView(view)
	m.mu.Unlock()
	if changed {
		m.publishClient(view)
	}
	return cloneClientView(view), nil
}

// UnregisterClient removes transient presentation state only.
func (m *Manager) UnregisterClient(ctx context.Context, workspaceID WorkspaceID, clientID ClientID) error {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	lock, err := m.lockFor(workspaceID)
	if err != nil {
		return err
	}
	lock.Lock()
	defer lock.Unlock()
	m.mu.Lock()
	workspaceClients := m.clients[workspaceID]
	if _, exists := workspaceClients[clientID]; !exists {
		m.mu.Unlock()
		return fmt.Errorf("client %q: %w", clientID, ErrClientNotFound)
	}
	delete(workspaceClients, clientID)
	m.mu.Unlock()
	m.closeClientSubscriptions(workspaceID, clientID)
	return nil
}

// Clients lists connected views without leaking another workspace partition.
func (m *Manager) Clients(ctx context.Context, workspaceID WorkspaceID) ([]ClientView, error) {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return nil, err
	}
	lock, err := m.lockFor(workspaceID)
	if err != nil {
		return nil, err
	}
	lock.Lock()
	defer lock.Unlock()
	m.mu.Lock()
	views := make([]ClientView, 0, len(m.clients[workspaceID]))
	for _, view := range m.clients[workspaceID] {
		views = append(views, cloneClientView(view))
	}
	m.mu.Unlock()
	sort.Slice(views, func(left, right int) bool {
		if views[left].ConnectedAt.Equal(views[right].ConnectedAt) {
			return views[left].ClientID < views[right].ClientID
		}
		return views[left].ConnectedAt.Before(views[right].ConnectedAt)
	})
	return views, nil
}

func (m *Manager) clientForRequest(request CommandRequest, commandID CommandID) (*ClientView, error) {
	if request.ClientID == nil {
		if commandID == CommandWindowZoom {
			return nil, fmt.Errorf("command %q requires client ID: %w", commandID, ErrClientNotFound)
		}
		return nil, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	view, exists := m.clients[request.WorkspaceID][*request.ClientID]
	if !exists {
		return nil, fmt.Errorf("client %q: %w", *request.ClientID, ErrClientNotFound)
	}
	cloned := cloneClientView(view)
	return &cloned, nil
}

func (m *Manager) executePresentation(
	snapshot Snapshot,
	request CommandRequest,
	rebasedFrom *Revision,
) (Result, error) {
	view, changed, changes, err := m.applyPresentation(snapshot, request, true)
	if err != nil {
		return Result{}, err
	}
	if changed {
		m.publishClient(view)
	}
	return Result{
		Snapshot:    cloneSnapshot(snapshot),
		Applied:     changed,
		Changes:     changes,
		Client:      &view,
		RebasedFrom: rebasedFrom,
	}, nil
}

func (m *Manager) previewPresentation(snapshot Snapshot, request CommandRequest) (Preview, error) {
	view, changed, changes, err := m.applyPresentation(snapshot, request, false)
	if err != nil {
		return Preview{}, err
	}
	return Preview{Snapshot: cloneSnapshot(snapshot), Changed: changed, Changes: changes, Client: &view}, nil
}

func (m *Manager) applyPresentation(
	snapshot Snapshot,
	request CommandRequest,
	persist bool,
) (ClientView, bool, ChangeSet, error) {
	if request.ClientID == nil {
		return ClientView{}, false, ChangeSet{}, fmt.Errorf(
			"presentation command requires client ID: %w",
			ErrClientNotFound,
		)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	view, exists := m.clients[request.WorkspaceID][*request.ClientID]
	if !exists {
		return ClientView{}, false, ChangeSet{}, fmt.Errorf("client %q: %w", *request.ClientID, ErrClientNotFound)
	}
	before := cloneClientView(view)
	switch command := request.Payload.(type) {
	case SwitchDesktopCommand:
		if _, exists := desktopIndexByID(&snapshot, command.DesktopID); !exists {
			return ClientView{}, false, ChangeSet{}, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
		}
		view.ActiveDesktopID = command.DesktopID
		view = repairClientView(view, snapshot)
	case FocusWindowCommand:
		focused, focusErr := resolveFocusTarget(view, snapshot, command)
		if focusErr != nil {
			return ClientView{}, false, ChangeSet{}, focusErr
		}
		if focused != "" {
			// Focusing a window on another desktop activates that desktop for the client.
			if window, exists := snapshot.Windows[focused]; exists &&
				window.DesktopID != view.ActiveDesktopID {
				view.ActiveDesktopID = window.DesktopID
			}
			view.FocusedWindowID = &focused
			view.FocusOrder = prependFocus(view.FocusOrder, focused)
			view = repairClientView(view, snapshot)
		}
	default:
		return ClientView{}, false, ChangeSet{}, fmt.Errorf(
			"command %T is not presentation state: %w",
			request.Payload,
			ErrInvalidCommand,
		)
	}
	changed := !clientViewsEqual(before, view)
	if changed {
		next, revisionErr := nextPresentationRevision(before.PresentationRevision)
		if revisionErr != nil {
			return ClientView{}, false, ChangeSet{}, fmt.Errorf(
				"advance client %q: %w",
				view.ClientID,
				revisionErr,
			)
		}
		view.PresentationRevision = next
		if persist {
			m.clients[request.WorkspaceID][view.ClientID] = cloneClientView(view)
		}
	}
	changes := ChangeSet{}
	if changed {
		changes.ClientIDs = []ClientID{view.ClientID}
	}
	return cloneClientView(view), changed, changes, nil
}

func resolveFocusTarget(view ClientView, snapshot Snapshot, command FocusWindowCommand) (WindowID, error) {
	if command.WindowID != nil {
		window, exists := snapshot.Windows[*command.WindowID]
		if !exists {
			return "", fmt.Errorf("window %q: %w", *command.WindowID, ErrWindowNotFound)
		}
		if window.Minimized {
			return "", fmt.Errorf(
				"window %q is not focusable while minimized: %w",
				*command.WindowID,
				ErrInvalidCommand,
			)
		}
		return *command.WindowID, nil
	}
	if command.Direction == "" {
		return "", fmt.Errorf("focus target or direction is required: %w", ErrInvalidCommand)
	}
	if view.FocusedWindowID == nil {
		return firstVisibleWindow(snapshot, view.ActiveDesktopID), nil
	}
	windowID, exists := directionalWindow(snapshot, view.ActiveDesktopID, *view.FocusedWindowID, command.Direction)
	if !exists {
		return "", nil
	}
	return windowID, nil
}
