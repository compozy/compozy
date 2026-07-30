package extensionpkg

import (
	"context"

	"fmt"

	"slices"
	"strings"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	compozyconfig "github.com/compozy/compozy/internal/config"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	hookspkg "github.com/compozy/compozy/internal/hooks"
)

// Get returns the current snapshot for one installed extension.
func (m *Manager) Get(name string) (*Extension, error) {
	return m.GetForInstance(GlobalInstanceKey(name))
}

// GetForInstance resolves a workspace dev overlay before the global published instance.
func (m *Manager) GetForInstance(key InstanceKey) (*Extension, error) {
	if m == nil {
		return nil, ErrManagerRequired
	}

	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	ext := m.instanceLocked(key)
	if ext == nil && !key.IsGlobal() {
		ext = m.extensions[key.Name]
	}
	m.mu.RUnlock()
	if ext != nil {
		snapshot := m.cloneExtension(ext)
		if !ext.instanceKey().IsGlobal() {
			link, err := m.registry.GetDevLink(key.Name, key.WorkspaceID)
			if err == nil {
				snapshot.DevLink = link
				_, publishedErr := m.registry.Get(key.Name)
				snapshot.OverridesPublished = publishedErr == nil
			}
		}
		return snapshot, nil
	}

	if m.registry == nil {
		return nil, &ExtensionNotFoundError{Name: key.Name}
	}
	info, err := m.registry.Get(key.Name)
	if err != nil {
		return nil, err
	}
	return &Extension{
		Info:   *info,
		Status: ExtensionStatus{Name: info.Name, Version: info.Version, Source: info.Source, Enabled: info.Enabled},
	}, nil
}

// ListForWorkspace returns the effective extension registry for one trusted workspace.
func (m *Manager) ListForWorkspace(workspaceID string) []ExtensionInfo {
	if m == nil {
		return nil
	}
	workspaceID = strings.TrimSpace(workspaceID)
	m.mu.RLock()
	byName := make(map[string]ExtensionInfo, len(m.extensions)+len(m.devExtensions))
	for name, ext := range m.extensions {
		byName[name] = ext.info
	}
	if workspaceID != "" {
		for key, ext := range m.devExtensions {
			if key.WorkspaceID == workspaceID {
				byName[key.Name] = ext.info
			}
		}
	}
	m.mu.RUnlock()
	infos := make([]ExtensionInfo, 0, len(byName))
	for _, info := range byName {
		infos = append(infos, info)
	}
	slices.SortFunc(infos, func(left, right ExtensionInfo) int {
		return strings.Compare(left.Name, right.Name)
	})
	return infos
}

// StatusesForWorkspace returns effective status without leaking another workspace's dev instances.
func (m *Manager) StatusesForWorkspace(workspaceID string) []ExtensionStatus {
	infos := m.ListForWorkspace(workspaceID)
	statuses := make([]ExtensionStatus, 0, len(infos))
	for _, info := range infos {
		snapshot, err := m.GetForInstance(InstanceKey{Name: info.Name, WorkspaceID: workspaceID})
		if err == nil {
			statuses = append(statuses, snapshot.Status)
		}
	}
	return statuses
}

// List returns every currently known registry row in name order.
func (m *Manager) List() []ExtensionInfo {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]ExtensionInfo, 0, len(m.extensions))
	for _, ext := range m.extensions {
		infos = append(infos, ext.info)
	}
	slices.SortFunc(infos, func(left, right ExtensionInfo) int {
		return strings.Compare(left.Name, right.Name)
	})
	return infos
}

// Statuses returns the current runtime health snapshot for every known extension.
func (m *Manager) Statuses() []ExtensionStatus {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]ExtensionStatus, 0, len(m.extensions))
	for _, ext := range m.extensions {
		statuses = append(statuses, m.statusLocked(ext))
	}
	slices.SortFunc(statuses, func(left, right ExtensionStatus) int {
		return strings.Compare(left.Name, right.Name)
	})
	return statuses
}

// BridgeTargetSnapshots calls the negotiated bridge target snapshot service on
// the named bridge-capable extension runtime.
func (m *Manager) BridgeTargetSnapshots(
	ctx context.Context,
	extensionName string,
	req bridgepkg.BridgeTargetSnapshotRequest,
) ([]bridgepkg.BridgeTargetSnapshot, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	process, name, err := m.extensionServiceProcess(
		ctx,
		extensionName,
		extensionprotocol.ExtensionServiceMethodBridgeTargets,
	)
	if err != nil {
		return nil, err
	}

	var response bridgepkg.BridgeTargetSnapshotResponse
	if err := process.Call(
		ctx,
		string(extensionprotocol.ExtensionServiceMethodBridgeTargets),
		req,
		&response,
	); err != nil {
		return nil, fmt.Errorf("extension: list bridge target snapshots via %q: %w", name, err)
	}
	for index, target := range response.Targets {
		if err := target.Validate(); err != nil {
			return nil, fmt.Errorf("extension: bridge target snapshot %d from %q: %w", index, name, err)
		}
	}
	return cloneBridgeTargetSnapshots(response.Targets), nil
}

func cloneBridgeTargetSnapshots(values []bridgepkg.BridgeTargetSnapshot) []bridgepkg.BridgeTargetSnapshot {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]bridgepkg.BridgeTargetSnapshot, len(values))
	for index, value := range values {
		cloned[index] = value
		if len(value.Capabilities) > 0 {
			cloned[index].Capabilities = append([]string(nil), value.Capabilities...)
		}
	}
	return cloned
}

// HookDeclarations returns the manifest-declared hook resources from loaded extensions.
func (m *Manager) HookDeclarations(ctx context.Context) ([]hookspkg.HookDecl, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrManagerRequired
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	decls := make([]hookspkg.HookDecl, 0)
	names := make([]string, 0, len(m.extensions))
	for name := range m.extensions {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, name := range names {
		ext := m.extensions[name]
		if !ext.registered {
			continue
		}
		for _, decl := range ext.hooks {
			decls = append(decls, cloneHookDecl(decl))
		}
	}
	return decls, nil
}

// AgentDefinitions returns the currently registered extension agent definitions.
func (m *Manager) AgentDefinitions() []compozyconfig.AgentDef {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var agents []compozyconfig.AgentDef
	names := make([]string, 0, len(m.extensions))
	for name := range m.extensions {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		ext := m.extensions[name]
		if !ext.registered {
			continue
		}
		for _, agent := range ext.agents {
			agents = append(agents, compozyconfig.CloneAgentDef(agent))
		}
	}
	return agents
}
