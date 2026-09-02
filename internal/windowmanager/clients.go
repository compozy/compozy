package windowmanager

import (
	"context"
	"fmt"
	"sort"
	"strings"
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
	defer m.releaseWorkspaceLock(registration.WorkspaceID, lock)
	snapshot, err := m.loadSnapshot(ctx, registration.WorkspaceID)
	if err != nil {
		return ClientView{}, err
	}
	prepared, err := m.prepareClientRegistration(snapshot, registration)
	if err != nil {
		return ClientView{}, err
	}
	stored, previousShortcuts, changed, err := m.storeRegisteredClient(snapshot, registration, prepared)
	if err != nil {
		return ClientView{}, err
	}
	if changed {
		m.publishClient(stored)
		m.observeGlobalShortcutFailures(
			ctx,
			registration.WorkspaceID,
			stored.ClientID,
			previousShortcuts,
			stored.GlobalShortcuts,
		)
	}
	result := cloneClientView(stored)
	result.AttachmentToken = prepared.token
	return result, nil
}

type preparedClientRegistration struct {
	clientID ClientID
	kind     ClientKind
	token    string
	digest   [32]byte
}

func (m *Manager) prepareClientRegistration(
	snapshot Snapshot,
	registration ClientRegistration,
) (preparedClientRegistration, error) {
	clientID := registration.ClientID
	if clientID == "" {
		generated, generateErr := m.generate("client")
		if generateErr != nil {
			return preparedClientRegistration{}, fmt.Errorf("generate client ID: %w", generateErr)
		}
		clientID = ClientID(generated)
	}
	if registration.ActiveDesktopID != "" {
		if _, exists := desktopIndexByID(&snapshot, registration.ActiveDesktopID); !exists {
			return preparedClientRegistration{}, fmt.Errorf(
				"desktop %q: %w",
				registration.ActiveDesktopID,
				ErrDesktopNotFound,
			)
		}
	}
	kind, err := normalizeClientKind(registration.Kind)
	if err != nil {
		return preparedClientRegistration{}, err
	}
	token, digest, err := newAttachmentToken()
	if err != nil {
		return preparedClientRegistration{}, fmt.Errorf("mint client attachment token: %w", err)
	}
	return preparedClientRegistration{clientID: clientID, kind: kind, token: token, digest: digest}, nil
}

func (m *Manager) storeRegisteredClient(
	snapshot Snapshot,
	registration ClientRegistration,
	prepared preparedClientRegistration,
) (ClientView, []GlobalShortcutRegistration, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	workspaceClients := m.clients[registration.WorkspaceID]
	if workspaceClients == nil {
		workspaceClients = make(map[ClientID]ClientView)
		m.clients[registration.WorkspaceID] = workspaceClients
	}
	view, exists := workspaceClients[prepared.clientID]
	var before ClientView
	if !exists {
		view = ClientView{
			WorkspaceID:          registration.WorkspaceID,
			ClientID:             prepared.clientID,
			Kind:                 prepared.kind,
			PresentationRevision: 1,
			ContextRevision:      1,
			ConnectedAt:          m.now().UTC(),
			FocusOrder:           []WindowID{},
			StackActive:          map[NodeID]WindowID{},
		}
	} else {
		before = cloneClientView(view)
	}
	view.Kind = prepared.kind
	activeID := registration.ActiveDesktopID
	if activeID == "" {
		if exists {
			activeID = view.ActiveDesktopID
		} else {
			activeID = defaultDesktopID(snapshot)
		}
	}
	view.ActiveDesktopID = activeID
	view.PaletteContext.ScopeGlobal = registration.Context.ScopeGlobal
	view.PaletteContext.FocusedSessionState = strings.TrimSpace(registration.Context.FocusedSessionState)
	view.PaletteContext.WorkspaceTrusted = registration.Context.WorkspaceTrusted
	view.PaletteContext.DestinationIntent = cloneRouteIntentPointer(registration.Context.DestinationIntent)
	registrations, err := normalizeGlobalShortcutRegistrations(
		CloneGlobalShortcutRegistrations(registration.Context.GlobalShortcuts),
	)
	if err != nil {
		return ClientView{}, nil, false, fmt.Errorf("client %q global shortcuts: %w", prepared.clientID, err)
	}
	if prepared.kind != ClientKindShell && len(registrations) > 0 {
		return ClientView{}, nil, false, fmt.Errorf(
			"browser client %q global shortcuts: %w",
			prepared.clientID,
			ErrInvalidCommand,
		)
	}
	view.GlobalShortcuts = registrations
	view = repairClientView(view, snapshot)
	changed := !exists || !clientViewsEqual(before, view)
	contextChanged := !exists || before.Kind != view.Kind ||
		!paletteContextsEqual(before.PaletteContext, view.PaletteContext) ||
		!globalShortcutRegistrationsEqual(before.GlobalShortcuts, view.GlobalShortcuts)
	view, err = advanceRegisteredClientRevisions(prepared.clientID, before, view, exists, changed, contextChanged)
	if err != nil {
		return ClientView{}, nil, false, err
	}
	stored := cloneClientView(view)
	stored.AttachmentToken = ""
	workspaceClients[prepared.clientID] = stored
	workspaceTokens := m.clientTokens[registration.WorkspaceID]
	if workspaceTokens == nil {
		workspaceTokens = make(map[ClientID][32]byte)
		m.clientTokens[registration.WorkspaceID] = workspaceTokens
	}
	workspaceTokens[prepared.clientID] = prepared.digest
	return stored, CloneGlobalShortcutRegistrations(before.GlobalShortcuts), changed, nil
}

func advanceRegisteredClientRevisions(
	clientID ClientID,
	before ClientView,
	view ClientView,
	exists bool,
	changed bool,
	contextChanged bool,
) (ClientView, error) {
	if exists && changed {
		next, err := nextPresentationRevision(before.PresentationRevision)
		if err != nil {
			return ClientView{}, fmt.Errorf("advance client %q: %w", clientID, err)
		}
		view.PresentationRevision = next
	}
	if exists && contextChanged {
		next, err := nextContextRevision(before.ContextRevision)
		if err != nil {
			return ClientView{}, fmt.Errorf("advance client %q context: %w", clientID, err)
		}
		view.ContextRevision = next
	}
	return view, nil
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
	defer m.releaseWorkspaceLock(workspaceID, lock)
	if _, err := m.flushStackActiveLocked(ctx, workspaceID); err != nil {
		return fmt.Errorf("flush stack activation before unregister client %q: %w", clientID, err)
	}
	m.mu.Lock()
	workspaceClients := m.clients[workspaceID]
	if _, exists := workspaceClients[clientID]; !exists {
		m.mu.Unlock()
		return fmt.Errorf("client %q: %w", clientID, ErrClientNotFound)
	}
	delete(workspaceClients, clientID)
	delete(m.clientTokens[workspaceID], clientID)
	endpoint := m.commandEndpoints[workspaceID][clientID]
	delete(m.commandEndpoints[workspaceID], clientID)
	m.mu.Unlock()
	if endpoint != nil {
		endpoint.closeWithError(ErrClientNotFound)
	}
	m.closeClientSubscriptions(workspaceID, clientID)
	if m.clientObserver != nil {
		if err := m.clientObserver(ctx, workspaceID, clientID); err != nil {
			return fmt.Errorf("unregister client %q observer: %w", clientID, err)
		}
	}
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
	defer m.releaseWorkspaceLock(workspaceID, lock)
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

func (m *Manager) clientForRequest(request CommandRequest) (*ClientView, error) {
	if request.ClientID == nil {
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
	view = cloneClientView(view)
	before := cloneClientView(view)
	view, err := m.applyPresentationCommand(snapshot, request, view, persist)
	if err != nil {
		return ClientView{}, false, ChangeSet{}, err
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
		if !paletteContextsEqual(before.PaletteContext, view.PaletteContext) {
			contextRevision, contextErr := nextContextRevision(before.ContextRevision)
			if contextErr != nil {
				return ClientView{}, false, ChangeSet{}, fmt.Errorf(
					"advance client %q context: %w",
					view.ClientID,
					contextErr,
				)
			}
			view.ContextRevision = contextRevision
		}
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

func (m *Manager) applyPresentationCommand(
	snapshot Snapshot,
	request CommandRequest,
	view ClientView,
	persist bool,
) (ClientView, error) {
	switch command := request.Payload.(type) {
	case SwitchDesktopCommand:
		if _, exists := desktopIndexByID(&snapshot, command.DesktopID); !exists {
			return ClientView{}, fmt.Errorf("desktop %q: %w", command.DesktopID, ErrDesktopNotFound)
		}
		view.ActiveDesktopID = command.DesktopID
		view = repairClientView(view, snapshot)
	case FocusWindowCommand:
		focused, focusErr := resolveFocusTarget(view, snapshot, command)
		if focusErr != nil {
			return ClientView{}, focusErr
		}
		if focused != "" {
			// Focusing a window on another desktop activates that desktop for the client.
			if window, exists := snapshot.Windows[focused]; exists &&
				window.DesktopID != view.ActiveDesktopID {
				view.ActiveDesktopID = window.DesktopID
			}
			view.FocusedWindowID = &focused
			view.FocusOrder = prependFocus(view.FocusOrder, focused)
			if location, stacked := findStackByWindow(&snapshot, focused); stacked {
				if view.StackActive == nil {
					view.StackActive = make(map[NodeID]WindowID)
				}
				view.StackActive[location.id()] = focused
				if persist {
					m.noteStackActive(request.WorkspaceID, location.id(), focused)
				}
			}
			view = repairClientView(view, snapshot)
		}
	default:
		return ClientView{}, fmt.Errorf(
			"command %T is not presentation state: %w",
			request.Payload,
			ErrInvalidCommand,
		)
	}
	return view, nil
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
