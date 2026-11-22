package extensionpkg

import (
	"context"
	"errors"
	"fmt"
)

func (m *Manager) startInstalledPackage(ctx context.Context, info *ExtensionInfo) error {
	key := GlobalInstanceKey(info.Name)
	ext := &managedExtension{
		key:     key,
		info:    *info,
		phase:   ExtensionPhaseDiscover,
		logRing: m.logRingFor(key),
	}
	m.mu.Lock()
	m.extensions[info.Name] = ext
	m.mu.Unlock()

	installations, err := m.registry.activeInstallations(ctx, info.Name)
	if err != nil {
		return fmt.Errorf("extension: list installations for %q: %w", info.Name, err)
	}
	var errs []error
	for _, installation := range installations {
		if installation.Scope.WorkspaceID != "" {
			_, linkErr := m.registry.GetDevLink(info.Name, installation.Scope.WorkspaceID)
			if linkErr == nil {
				continue
			}
			if !errors.Is(linkErr, ErrExtensionNotDevLinked) {
				errs = append(errs, linkErr)
				continue
			}
		}
		if installation.Scope.ProfileID != "" || installation.Scope.WorkspaceID != "" {
			instanceKey := InstanceKey{
				Name: info.Name, ProfileID: installation.Scope.ProfileID, WorkspaceID: installation.Scope.WorkspaceID,
			}
			errs = append(errs, m.startScopedInstallation(ctx, instanceKey))
			continue
		}
		if ext.info.Enabled {
			errs = append(errs, m.startOne(ctx, ext))
		}
	}
	return errors.Join(errs...)
}

func (m *Manager) startScopedInstallation(ctx context.Context, key InstanceKey) error {
	instance, err := m.newInstalledInstance(ctx, key)
	if err != nil {
		return err
	}
	m.mu.RLock()
	previous := m.scopedExtensions[key]
	reusable := previous != nil && previous.active && previous.info.Enabled == instance.info.Enabled &&
		previous.info.ManifestPath == instance.info.ManifestPath && previous.info.Checksum == instance.info.Checksum
	m.mu.RUnlock()
	if reusable {
		return nil
	}
	if previous != nil {
		if err := m.stopManagedExtension(ctx, previous); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.scopedExtensions[key] = instance
	m.mu.Unlock()
	if !instance.info.Enabled {
		return nil
	}
	return m.startOne(ctx, instance)
}

func (m *Manager) restoreWorkspaceInstallations(ctx context.Context, key InstanceKey) error {
	m.mu.RLock()
	stopping := m.stopping
	m.mu.RUnlock()
	if stopping {
		return nil
	}
	installations, err := m.registry.activeInstallations(ctx, key.Name)
	if err != nil {
		return err
	}
	var errs []error
	for _, installation := range installations {
		if installation.Scope.WorkspaceID != key.WorkspaceID {
			continue
		}
		errs = append(errs, m.startScopedInstallation(ctx, InstanceKey{
			Name: key.Name, WorkspaceID: key.WorkspaceID, ProfileID: installation.Scope.ProfileID,
		}))
	}
	return errors.Join(errs...)
}

func (m *Manager) newInstalledInstance(ctx context.Context, key InstanceKey) (*managedExtension, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := m.registry.Get(key.Name)
	if err != nil {
		return nil, err
	}
	if key.IsProfileScoped() {
		info.Enabled, err = m.registry.IsEnabledForProfile(key.Name, key.ProfileID)
		if err != nil {
			return nil, err
		}
	}
	return &managedExtension{key: key, info: *info, phase: ExtensionPhaseDiscover, logRing: m.logRingFor(key)}, nil
}

// RestartInstance restarts only the runtime that serves one instance after its launch inputs change.
// A lazily started profile runtime is invalidated and relaunches on its next call; an installed base
// or scoped runtime is stopped and started again from the registry. Every other runtime keeps running.
func (m *Manager) RestartInstance(ctx context.Context, key InstanceKey) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if m == nil {
		return ErrManagerRequired
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	source, development, err := m.readInstanceSource(ctx, key)
	if err != nil {
		return err
	}
	runtimeKey := runtimeKeyForInstallation(key, source)
	if development || (runtimeKey.IsProfileScoped() && !source.IsProfileScoped()) {
		// A dev overlay base restarts through `extension dev reload`; this only drops profile runtimes.
		return m.InvalidateProfileRuntime(ctx, key)
	}
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.mu.Lock()
	started := m.started && !m.stopping
	current := m.instanceLocked(runtimeKey)
	if started && current != nil {
		current.supervisionStopped = true
	}
	m.mu.Unlock()
	if !started {
		return nil
	}
	if err := m.stopManagedExtension(ctx, current); err != nil {
		return err
	}
	if runtimeKey.IsGlobal() && !runtimeKey.IsProfileScoped() {
		info, err := m.registry.Get(key.Name)
		if err != nil {
			return err
		}
		base := &managedExtension{
			key: runtimeKey, info: *info, phase: ExtensionPhaseDiscover, logRing: m.logRingFor(runtimeKey),
		}
		m.mu.Lock()
		m.extensions[key.Name] = base
		m.mu.Unlock()
		if !base.info.Enabled {
			return nil
		}
		return m.startOne(ctx, base)
	}
	m.mu.Lock()
	if m.scopedExtensions[runtimeKey] == current {
		delete(m.scopedExtensions, runtimeKey)
	}
	m.mu.Unlock()
	return m.startScopedInstallation(ctx, runtimeKey)
}
