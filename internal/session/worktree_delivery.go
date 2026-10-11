package session

import (
	"context"
	"fmt"
	"sync"
)

// AcquireWorktreeDeliveryFence excludes new and resumed runtimes while the exact
// bound session hands its checkout to daemon-owned delivery. Other sessions are
// never stopped by this operation.
func (m *Manager) AcquireWorktreeDeliveryFence(
	ctx context.Context,
	workspaceID, worktreeID, callerID string,
) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	key := workspaceID + "\x00" + worktreeID
	if m.worktreeDeliveryFences[key] {
		return nil, fmt.Errorf("%w: worktree delivery is in progress", ErrValidation)
	}
	info, err := m.Status(ctx, callerID)
	if err != nil {
		return nil, err
	}
	if info.WorkspaceID != workspaceID || info.WorktreeID != worktreeID {
		return nil, fmt.Errorf("%w: delivery caller is not bound to this worktree", ErrValidation)
	}
	m.mu.RLock()
	for id, bound := range m.sessions {
		other := bound.Info()
		if id != callerID && other.WorkspaceID == workspaceID && other.WorktreeID == worktreeID &&
			(other.State == StateStarting || other.State == StateActive || other.State == StateStopping) {
			m.mu.RUnlock()
			return nil, fmt.Errorf("%w: another session is bound to this worktree", ErrValidation)
		}
	}
	m.mu.RUnlock()
	if m.worktreeDeliveryFences == nil {
		m.worktreeDeliveryFences = make(map[string]bool)
	}
	m.worktreeDeliveryFences[key] = true
	return sync.OnceFunc(
		func() {
			m.lifecycleMu.Lock()
			delete(m.worktreeDeliveryFences, key)
			m.lifecycleMu.Unlock()
			// Stopping the delivery caller precedes its commit and PR. Settle the
			// subagent only once the fence releases, so its final snapshot includes those effects.
			if service := m.subagentService(); service != nil {
				settled, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
				defer cancel()
				m.logSubagentError(service.OnChildSettled(settled, callerID))
			}
		},
	), nil
}
