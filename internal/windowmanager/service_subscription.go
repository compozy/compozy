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
	defer lock.Unlock()
	snapshot, err := m.loadSnapshot(ctx, request.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if request.AfterRevision > snapshot.Revision {
		return nil, &RevisionConflictError{
			Expected: request.AfterRevision,
			Current:  snapshot.Revision,
			Details:  []Conflict{{Code: "subscription.after_ahead"}},
		}
	}
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
	defer lock.Unlock()
	if err := m.repository.DeleteWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("delete window workspace: %w", err)
	}
	m.mu.Lock()
	delete(m.clients, workspaceID)
	hub := m.hubs[workspaceID]
	delete(m.hubs, workspaceID)
	m.mu.Unlock()
	if hub != nil {
		hub.closeAll(ErrWorkspaceNotFound)
	}
	return nil
}

// ForgetWorkspace tears down transient state after the workspace owner commits deletion.
func (m *Manager) ForgetWorkspace(workspaceID WorkspaceID) {
	m.mu.Lock()
	delete(m.clients, workspaceID)
	delete(m.workspaceLocks, workspaceID)
	hub := m.hubs[workspaceID]
	delete(m.hubs, workspaceID)
	m.mu.Unlock()
	if hub != nil {
		hub.closeAll(ErrWorkspaceNotFound)
	}
}
