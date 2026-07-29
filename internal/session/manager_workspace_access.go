package session

import "github.com/compozy/compozy/internal/workspaceaccess"

// SetWorkspaceAccessPolicy installs the late-bound cross-workspace policy.
func (m *Manager) SetWorkspaceAccessPolicy(policy workspaceaccess.Policy) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.workspaceAccess = policy
}

func (m *Manager) workspaceAccessPolicy() workspaceaccess.Policy {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workspaceAccess
}
