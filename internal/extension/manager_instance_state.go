package extensionpkg

import (
	"cmp"
	"sync"

	"github.com/compozy/compozy/internal/resources"
)

func compareInstanceKeys(left, right InstanceKey) int {
	if result := cmp.Compare(left.Name, right.Name); result != 0 {
		return result
	}
	return cmp.Compare(left.WorkspaceID, right.WorkspaceID)
}

func instanceKeyFromAny(value any) InstanceKey {
	switch typed := value.(type) {
	case InstanceKey:
		return typed.Normalize()
	case string:
		return GlobalInstanceKey(typed)
	default:
		return InstanceKey{}
	}
}

func (ext *managedExtension) instanceKey() InstanceKey {
	if ext == nil {
		return InstanceKey{}
	}
	key := ext.key.Normalize()
	if key.Name == "" {
		key.Name = ext.info.Name
	}
	return key
}

func (ext *managedExtension) maxResourceScope() resources.ResourceScopeKind {
	if ext != nil && !ext.instanceKey().IsGlobal() {
		return resources.ResourceScopeKindWorkspace
	}
	return resources.ResourceScopeKindGlobal
}

func extensionCapabilityGrantID(key InstanceKey, sessionNonce string) string {
	return key.Normalize().runtimeID() + "#session:" + sessionNonce
}

func (m *Manager) instanceLocked(key InstanceKey) *managedExtension {
	key = key.Normalize()
	if key.IsGlobal() {
		return m.extensions[key.Name]
	}
	return m.devExtensions[key]
}

func (m *Manager) lookupInstance(key InstanceKey) (*managedExtension, bool) {
	if m == nil {
		return nil, false
	}
	key = key.Normalize()
	m.mu.RLock()
	defer m.mu.RUnlock()
	ext := m.instanceLocked(key)
	return ext, ext != nil
}

func (m *Manager) deleteInstanceLocked(key InstanceKey) {
	key = key.Normalize()
	if key.IsGlobal() {
		delete(m.extensions, key.Name)
		return
	}
	delete(m.devExtensions, key)
}

func (m *Manager) coordinatorFor(key InstanceKey) *sync.Mutex {
	key = key.Normalize()
	m.mu.Lock()
	defer m.mu.Unlock()
	coordinator := m.devCoordinators[key]
	if coordinator == nil {
		coordinator = &sync.Mutex{}
		m.devCoordinators[key] = coordinator
	}
	return coordinator
}

func (m *Manager) logRingFor(key InstanceKey) *ExtensionLogRing {
	key = key.Normalize()
	m.mu.Lock()
	defer m.mu.Unlock()
	ring := m.devLogs[key]
	if ring == nil {
		ring = newExtensionLogRing(m.now)
		m.devLogs[key] = ring
	}
	return ring
}
