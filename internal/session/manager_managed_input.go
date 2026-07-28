package session

import (
	"context"
	"errors"
	"strings"
)

type managedInputLease struct {
	owner  ManagedInputOwner
	cancel context.CancelFunc
	done   chan struct{}
}

// SetManagedInputLifecycle installs the late-bound daemon lifecycle adapter.
func (m *Manager) SetManagedInputLifecycle(lifecycle ManagedInputLifecycle) {
	if m == nil {
		return
	}
	m.managedInputMu.Lock()
	m.managedInputLifecycle = lifecycle
	m.managedInputMu.Unlock()
}

// NotifyManagedInputReady runs the unified selector for an idle session.
func (m *Manager) NotifyManagedInputReady(sessionID string) {
	if m == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	m.startNextQueuedInputPrompt(sessionID)
}

// CancelManagedInputLease cancels only the exact owner lease, never a newer prompt.
func (m *Manager) CancelManagedInputLease(owner ManagedInputOwner) bool {
	if m == nil || strings.TrimSpace(owner.QueueEntryID) == "" {
		return false
	}
	m.managedInputMu.Lock()
	lease, ok := m.managedInputLeases[owner.QueueEntryID]
	if !ok || !managedInputOwnersEqual(lease.owner, owner) {
		ok = false
	}
	m.managedInputMu.Unlock()
	if ok && lease.cancel != nil {
		lease.cancel()
	}
	return ok
}

// WaitManagedInputLease waits until the exact prompt pump has persisted its terminal and released.
func (m *Manager) WaitManagedInputLease(ctx context.Context, owner ManagedInputOwner) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: managed input wait context is required")
	}
	m.managedInputMu.Lock()
	lease, ok := m.managedInputLeases[owner.QueueEntryID]
	if ok && !managedInputOwnersEqual(lease.owner, owner) {
		m.managedInputMu.Unlock()
		return errors.New("session: managed input lease owner changed")
	}
	m.managedInputMu.Unlock()
	if !ok {
		return nil
	}
	select {
	case <-lease.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RevokeManagedInput reconciles durable revocation and cancels only the exact in-memory lease.
func (m *Manager) RevokeManagedInput(owner ManagedInputOwner, reason string) {
	if m == nil {
		return
	}
	if lifecycle := m.currentManagedInputLifecycle(); lifecycle != nil {
		if err := lifecycle.Revoke(m.fallbackLifecycleContext(), owner, reason); err != nil {
			m.logManagedInputError("session: reconcile managed input revocation failed", err, owner)
		}
	}
	m.CancelManagedInputLease(owner)
}

func managedInputOwnersEqual(left ManagedInputOwner, right ManagedInputOwner) bool {
	return left == right
}

func (m *Manager) currentManagedInputLifecycle() ManagedInputLifecycle {
	if m == nil {
		return nil
	}
	m.managedInputMu.Lock()
	defer m.managedInputMu.Unlock()
	return m.managedInputLifecycle
}

func (m *Manager) registerManagedInputLease(owner ManagedInputOwner, cancel context.CancelFunc) error {
	if m == nil || strings.TrimSpace(owner.QueueEntryID) == "" || cancel == nil {
		return errors.New("session: managed input lease identity is incomplete")
	}
	m.managedInputMu.Lock()
	defer m.managedInputMu.Unlock()
	if _, exists := m.managedInputLeases[owner.QueueEntryID]; exists {
		return errors.New("session: managed input lease already exists")
	}
	m.managedInputLeases[owner.QueueEntryID] = managedInputLease{
		owner: owner, cancel: cancel, done: make(chan struct{}),
	}
	return nil
}

func (m *Manager) releaseManagedInputLease(owner ManagedInputOwner) {
	if m == nil {
		return
	}
	m.managedInputMu.Lock()
	lease, ok := m.managedInputLeases[owner.QueueEntryID]
	if ok && managedInputOwnersEqual(lease.owner, owner) {
		delete(m.managedInputLeases, owner.QueueEntryID)
	} else {
		ok = false
	}
	m.managedInputMu.Unlock()
	if ok && lease.cancel != nil {
		lease.cancel()
	}
	if ok {
		close(lease.done)
	}
}
