package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
)

// ensureProfileRuntime returns a live runtime key whose process environment is
// isolated to the requested profile. The default profile keeps using the
// eagerly started base process so existing machine services remain singular.
func (m *Manager) ensureProfileRuntime(ctx context.Context, key InstanceKey) (InstanceKey, error) {
	if ctx == nil {
		return InstanceKey{}, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return InstanceKey{}, err
	}
	if m == nil {
		return InstanceKey{}, ErrManagerRequired
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return InstanceKey{}, err
	}
	if !key.IsProfileScoped() {
		return key, nil
	}
	if m.profileNames == nil {
		return InstanceKey{}, errors.New("extension: profile name resolver is required")
	}
	if _, err := m.profileNames.ProfileName(ctx, key.ProfileID); err != nil {
		return InstanceKey{}, fmt.Errorf("extension: resolve runtime profile %q: %w", key.ProfileID, err)
	}
	if m.registry != nil {
		enabled, err := m.registry.IsEnabledForProfile(key.Name, key.ProfileID)
		if err != nil {
			return InstanceKey{}, fmt.Errorf("extension: resolve profile runtime enablement: %w", err)
		}
		if !enabled {
			return InstanceKey{}, fmt.Errorf("extension: extension %q is disabled for the selected profile", key.Name)
		}
	}

	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()
	exists, active, runtimeErr := m.profileRuntimeState(key)
	if active {
		return key, nil
	}
	if exists {
		return InstanceKey{}, runtimeErr
	}
	profileRuntime, err := m.newProfileRuntime(key)
	if err != nil {
		return InstanceKey{}, err
	}
	transaction := newExtensionStartupTransaction(m, profileRuntime)
	launched, resourceSession, err := m.launchStartupRuntime(
		ctx,
		profileRuntime,
		profileRuntime.pendingGrant,
		transaction,
	)
	if err != nil {
		return InstanceKey{}, transaction.rollback(ctx, err)
	}
	prepared := &preparedExtensionStartup{
		transaction:     transaction,
		grant:           profileRuntime.pendingGrant,
		resourceSession: resourceSession,
		runtime:         &launched,
	}
	if err := m.commitPreparedExtensionWithPublish(ctx, profileRuntime, prepared, func() {
		m.profileExtensions[key] = profileRuntime
	}); err != nil {
		return InstanceKey{}, err
	}
	return key, nil
}

// InvalidateProfileRuntime stops one profile subprocess after its launch
// bindings change. The next scoped call starts it with the new resolved values.
func (m *Manager) InvalidateProfileRuntime(ctx context.Context, key InstanceKey) error {
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
	if !key.IsProfileScoped() {
		return nil
	}
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()

	m.mu.RLock()
	extension := m.profileExtensions[key]
	m.mu.RUnlock()
	if extension == nil {
		return nil
	}
	m.mu.Lock()
	if m.profileExtensions[key] == extension {
		extension.supervisionStopped = true
	}
	m.mu.Unlock()
	stopErr := m.stopManagedExtension(ctx, extension)
	m.mu.Lock()
	if m.profileExtensions[key] == extension {
		delete(m.profileExtensions, key)
	}
	m.mu.Unlock()
	if stopErr != nil {
		return fmt.Errorf("extension: stop stale profile runtime %q: %w", key.runtimeID(), stopErr)
	}
	return nil
}

func (m *Manager) profileRuntimeState(key InstanceKey) (bool, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	extension := m.profileExtensions[key.Normalize()]
	if extension == nil {
		return false, false, nil
	}
	if extension.active && extension.process != nil {
		return true, true, nil
	}
	message := strings.TrimSpace(extension.lastError)
	if message == "" {
		message = "profile runtime is recovering"
	}
	return true, false, fmt.Errorf("extension: extension %q is unavailable: %s", key.Name, message)
}

func (m *Manager) newProfileRuntime(key InstanceKey) (*managedExtension, error) {
	baseKey := key
	baseKey.ProfileID = ""
	m.mu.RLock()
	base := m.instanceLocked(baseKey)
	if base == nil && !baseKey.IsGlobal() {
		base = m.extensions[baseKey.Name]
	}
	if base == nil || base.manifest == nil {
		m.mu.RUnlock()
		return nil, fmt.Errorf("extension: extension %q has no loaded runtime definition", key.Name)
	}
	info := cloneExtensionInfo(base.info)
	manifest := profileRuntimeManifest(base.manifest)
	rootDir := base.rootDir
	grant := cloneEffectiveGrant(base.pendingGrant)
	generationHash := base.generationHash
	lastGoodGeneration := base.lastGoodGeneration
	m.mu.RUnlock()

	return &managedExtension{
		key:                key.Normalize(),
		info:               info,
		rootDir:            rootDir,
		manifest:           manifest,
		pendingGrant:       grant,
		phase:              ExtensionPhaseValidate,
		generationHash:     generationHash,
		lastGoodGeneration: lastGoodGeneration,
		logRing:            m.logRingFor(key),
	}, nil
}

func profileRuntimeManifest(source *Manifest) *Manifest {
	manifest := cloneManifest(source)
	if manifest == nil {
		return nil
	}
	manifest.Capabilities.Provides = slices.DeleteFunc(
		manifest.Capabilities.Provides,
		func(capability string) bool {
			switch strings.TrimSpace(capability) {
			case extensionprotocol.CapabilityToolProvider,
				extensionprotocol.CapabilityProvideViewProvider:
				return false
			default:
				return true
			}
		},
	)
	return manifest
}

func cloneEffectiveGrant(grant EffectiveGrant) EffectiveGrant {
	return EffectiveGrant{
		Permissions:    slices.Clone(grant.Permissions),
		Security:       slices.Clone(grant.Security),
		ResourceKinds:  slices.Clone(grant.ResourceKinds),
		ResourceScopes: slices.Clone(grant.ResourceScopes),
	}
}

func profileWorkspaceScopeID(workspaceID, profileName string) string {
	return strings.TrimSpace(workspaceID) + "@pf:" + strings.TrimSpace(profileName)
}
