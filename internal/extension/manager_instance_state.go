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
	if result := cmp.Compare(left.ProfileID, right.ProfileID); result != 0 {
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
	if ext == nil {
		return resources.ResourceScopeKindUser
	}
	key := ext.instanceKey()
	if key.IsProfileScoped() && !key.IsGlobal() {
		return resources.ResourceScopeKindWorkspaceProfile
	}
	if key.IsProfileScoped() {
		return resources.ResourceScopeKindProfile
	}
	if !key.IsGlobal() {
		return resources.ResourceScopeKindWorkspace
	}
	return resources.ResourceScopeKindUser
}

func extensionCapabilityGrantID(key InstanceKey, sessionNonce string) string {
	return key.Normalize().runtimeID() + "#session:" + sessionNonce
}

func (m *Manager) instanceLocked(key InstanceKey) *managedExtension {
	key = key.Normalize()
	if key.IsProfileScoped() {
		return m.profileExtensions[key]
	}
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
	if key.IsProfileScoped() {
		delete(m.profileExtensions, key)
		return
	}
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
	coordinators := m.devCoordinators
	if key.IsProfileScoped() {
		coordinators = m.profileCoordinators
	}
	coordinator := coordinators[key]
	if coordinator == nil {
		coordinator = &sync.Mutex{}
		coordinators[key] = coordinator
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
