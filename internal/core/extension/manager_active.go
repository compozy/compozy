package extensions

import (
	"strings"
	"sync"
)

var activeManagers = struct {
	mu       sync.RWMutex
	managers map[*Manager]struct{}
}{
	managers: make(map[*Manager]struct{}),
}

func registerActiveManager(manager *Manager) {
	if manager == nil {
		return
	}

	activeManagers.mu.Lock()
	activeManagers.managers[manager] = struct{}{}
	activeManagers.mu.Unlock()
}

func unregisterActiveManager(manager *Manager) {
	if manager == nil {
		return
	}

	activeManagers.mu.Lock()
	delete(activeManagers.managers, manager)
	activeManagers.mu.Unlock()
}

func lookupActiveExtensionSession(workspaceRoot string, extensionName string) *extensionSession {
	normalizedRoot := strings.TrimSpace(workspaceRoot)
	normalizedName := strings.TrimSpace(extensionName)
	if normalizedRoot == "" || normalizedName == "" {
		return nil
	}

	activeManagers.mu.RLock()
	defer activeManagers.mu.RUnlock()

	for manager := range activeManagers.managers {
		if manager == nil || strings.TrimSpace(manager.workspaceRoot) != normalizedRoot {
			continue
		}
		session, ok := manager.sessionForExtension(normalizedName)
		if !ok || session == nil || session.runtime == nil {
			continue
		}
		switch session.runtime.State() {
		case ExtensionStateReady, ExtensionStateDegraded:
			return session
		}
	}
	return nil
}
