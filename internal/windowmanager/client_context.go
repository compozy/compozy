package windowmanager

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

func normalizeClientKind(kind ClientKind) (ClientKind, error) {
	switch kind {
	case "", ClientKindBrowser:
		return ClientKindBrowser, nil
	case ClientKindShell:
		return ClientKindShell, nil
	default:
		return "", fmt.Errorf("client kind %q: %w", kind, ErrInvalidCommand)
	}
}

// UpdateClientContext replaces client-owned palette context and derives topology-owned fields.
func (m *Manager) UpdateClientContext(
	ctx context.Context,
	update ClientContextUpdate,
) (ClientView, error) {
	if err := m.resolveWorkspace(ctx, update.WorkspaceID); err != nil {
		return ClientView{}, err
	}
	lock, err := m.lockFor(update.WorkspaceID)
	if err != nil {
		return ClientView{}, err
	}
	lock.Lock()
	defer m.releaseWorkspaceLock(update.WorkspaceID, lock)
	snapshot, err := m.loadSnapshot(ctx, update.WorkspaceID)
	if err != nil {
		return ClientView{}, err
	}
	m.mu.Lock()
	view, exists := m.clients[update.WorkspaceID][update.ClientID]
	if !exists {
		m.mu.Unlock()
		return ClientView{}, fmt.Errorf("client %q: %w", update.ClientID, ErrClientNotFound)
	}
	before := cloneClientView(view)
	view.PaletteContext.ScopeGlobal = update.Context.ScopeGlobal
	view.PaletteContext.FocusedSessionState = strings.TrimSpace(update.Context.FocusedSessionState)
	view.PaletteContext.WorkspaceTrusted = update.Context.WorkspaceTrusted
	view.PaletteContext.DestinationIntent = cloneRouteIntentPointer(update.Context.DestinationIntent)
	registrations, err := normalizeGlobalShortcutRegistrations(update.Context.GlobalShortcuts)
	if err != nil {
		m.mu.Unlock()
		return ClientView{}, fmt.Errorf("client %q global shortcuts: %w", update.ClientID, err)
	}
	if view.Kind != ClientKindShell && len(registrations) > 0 {
		m.mu.Unlock()
		return ClientView{}, fmt.Errorf("browser client %q global shortcuts: %w", update.ClientID, ErrInvalidCommand)
	}
	view.GlobalShortcuts = registrations
	view = repairClientView(view, snapshot)
	if paletteContextsEqual(before.PaletteContext, view.PaletteContext) &&
		globalShortcutRegistrationsEqual(before.GlobalShortcuts, view.GlobalShortcuts) {
		m.mu.Unlock()
		return cloneClientView(view), nil
	}
	presentationRevision, err := nextPresentationRevision(before.PresentationRevision)
	if err != nil {
		m.mu.Unlock()
		return ClientView{}, fmt.Errorf("advance client %q: %w", update.ClientID, err)
	}
	contextRevision, err := nextContextRevision(before.ContextRevision)
	if err != nil {
		m.mu.Unlock()
		return ClientView{}, fmt.Errorf("advance client %q context: %w", update.ClientID, err)
	}
	view.PresentationRevision = presentationRevision
	view.ContextRevision = contextRevision
	m.clients[update.WorkspaceID][update.ClientID] = cloneClientView(view)
	m.mu.Unlock()
	m.publishClient(view)
	if m.globalShortcutObserver != nil {
		for _, registration := range view.GlobalShortcuts {
			if registration.Status == GlobalShortcutRegistered ||
				!globalShortcutRegistrationChanged(before.GlobalShortcuts, registration) {
				continue
			}
			m.globalShortcutObserver(
				ctx,
				update.WorkspaceID,
				update.ClientID,
				registration,
			)
		}
	}
	return cloneClientView(view), nil
}

func globalShortcutRegistrationChanged(
	before []GlobalShortcutRegistration,
	current GlobalShortcutRegistration,
) bool {
	for _, previous := range before {
		if previous.CommandID == current.CommandID {
			return !reflect.DeepEqual(previous, current)
		}
	}
	return true
}

func derivePaletteContext(view ClientView, snapshot Snapshot) PaletteContext {
	context := view.PaletteContext
	context.WindowFocused = view.FocusedWindowID != nil
	context.WindowFloating = false
	context.WindowStacked = false
	context.DesktopWindowCount = 0
	context.ShellDesktop = view.Kind == ClientKindShell
	for _, window := range snapshot.Windows {
		if window.DesktopID == view.ActiveDesktopID && !window.Minimized {
			context.DesktopWindowCount++
		}
	}
	if view.FocusedWindowID != nil {
		if window, exists := snapshot.Windows[*view.FocusedWindowID]; exists {
			context.WindowFloating = window.Placement == WindowPlacementFloating
			_, context.WindowStacked = findStackByWindow(&snapshot, *view.FocusedWindowID)
		}
	}
	return context
}

func paletteContextsEqual(left, right PaletteContext) bool {
	return reflect.DeepEqual(left, right)
}

func globalShortcutRegistrationsEqual(left, right []GlobalShortcutRegistration) bool {
	return reflect.DeepEqual(left, right)
}

func cloneRouteIntentPointer(intent *RouteIntent) *RouteIntent {
	if intent == nil {
		return nil
	}
	cloned := cloneRouteIntent(*intent)
	return &cloned
}
