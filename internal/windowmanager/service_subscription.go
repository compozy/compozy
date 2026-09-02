package windowmanager

import (
	"context"
	"fmt"
)

// Subscribe returns one atomic topology/client fence followed by ordered updates.
func (m *Manager) Subscribe(ctx context.Context, request SubscriptionRequest) (Subscription, error) {
	if err := m.resolveWorkspace(ctx, request.WorkspaceID); err != nil {
		return nil, err
	}
	lock, err := m.lockFor(request.WorkspaceID)
	if err != nil {
		return nil, err
	}
	lock.Lock()
	defer m.releaseWorkspaceLock(request.WorkspaceID, lock)
	snapshot, err := m.loadSnapshot(ctx, request.WorkspaceID)
	if err != nil {
		return nil, err
	}
	// A client may remember a revision the authority no longer has: the stored
	// arrangement was discarded or replaced while it was away. The fence is the
	// truth, so such a client restarts from the current snapshot instead of
	// being refused until someone reloads it.
	fence := SubscriptionFence{Snapshot: snapshot}
	if request.ClientID != nil {
		m.mu.Lock()
		view, exists := m.clients[request.WorkspaceID][*request.ClientID]
		m.mu.Unlock()
		if !exists {
			return nil, fmt.Errorf("client %q: %w", *request.ClientID, ErrClientNotFound)
		}
		client := cloneClientView(view)
		fence.Client = &client
	}
	return m.addSubscription(ctx, request.WorkspaceID, fence)
}

func (m *Manager) addSubscription(
	ctx context.Context,
	workspaceID WorkspaceID,
	fence SubscriptionFence,
) (Subscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	hub := m.hubs[workspaceID]
	if hub == nil {
		hub = newSubscriptionHub(m.subscriptionBuffer)
		m.hubs[workspaceID] = hub
	}
	return hub.add(ctx, fence), nil
}

func (m *Manager) publishEvent(event Event) {
	m.mu.Lock()
	hub := m.hubs[event.WorkspaceID]
	m.mu.Unlock()
	if hub != nil {
		hub.publishEvent(event)
	}
}

func (m *Manager) publishClient(view ClientView) {
	m.mu.Lock()
	hub := m.hubs[view.WorkspaceID]
	m.mu.Unlock()
	if hub != nil {
		hub.publishClient(view)
	}
}

func (m *Manager) closeClientSubscriptions(workspaceID WorkspaceID, clientID ClientID) {
	m.mu.Lock()
	hub := m.hubs[workspaceID]
	m.mu.Unlock()
	if hub != nil {
		hub.closeClient(clientID, ErrClientNotFound)
	}
}

// DeleteWorkspace removes window state, clients, and subscriptions for an authorized workspace.
func (m *Manager) DeleteWorkspace(ctx context.Context, workspaceID WorkspaceID) error {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return err
	}
	lock, err := m.lockFor(workspaceID)
	if err != nil {
		return err
	}
	lock.Lock()
	defer m.releaseWorkspaceLock(workspaceID, lock)
	if err := m.repository.DeleteWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("delete window workspace: %w", err)
	}
	m.coalescer.discardWorkspaceLocked(workspaceID)
	m.retireWorkspaceLock(workspaceID, lock)
	m.teardownWorkspaceClients(workspaceID)
	return nil
}

// ForgetWorkspace tears down transient state after the workspace owner commits deletion.
func (m *Manager) ForgetWorkspace(workspaceID WorkspaceID) {
	if lock, err := m.lockFor(workspaceID); err == nil {
		lock.Lock()
		m.coalescer.discardWorkspaceLocked(workspaceID)
		m.retireWorkspaceLock(workspaceID, lock)
		m.releaseWorkspaceLock(workspaceID, lock)
	}
	m.teardownWorkspaceClients(workspaceID)
}

func (m *Manager) teardownWorkspaceClients(workspaceID WorkspaceID) {
	m.mu.Lock()
	delete(m.clients, workspaceID)
	delete(m.clientTokens, workspaceID)
	endpoints := m.takeCommandEndpointsLocked(workspaceID)
	hub := m.hubs[workspaceID]
	delete(m.hubs, workspaceID)
	m.mu.Unlock()
	closeClientCommandEndpoints(endpoints, ErrWorkspaceNotFound)
	if hub != nil {
		hub.closeAll(ErrWorkspaceNotFound)
	}
}
