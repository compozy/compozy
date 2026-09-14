package extensionpkg

import (
	"context"
	"errors"
	"slices"
)

type retiredWorkspaceProfile struct {
	previous   *managedExtension
	definition *managedExtension
	active     bool
}

// The workspace coordinator serializes retirement with profile startup and invalidation.
func (m *Manager) retireWorkspaceProfileRuntimes(
	ctx context.Context, workspace InstanceKey, transaction *extensionStartupTransaction,
) error {
	if transaction == nil {
		return errors.New("extension: startup transaction is required for profile retirement")
	}
	m.mu.Lock()
	var retired []retiredWorkspaceProfile
	for key, current := range m.scopedExtensions {
		if key.Name != workspace.Name || key.WorkspaceID != workspace.WorkspaceID || !key.IsProfileScoped() {
			continue
		}
		retired = append(retired, retiredWorkspaceProfile{
			previous: current, active: current.active,
			definition: &managedExtension{
				key: key, info: cloneExtensionInfo(&current.info), rootDir: current.rootDir,
				manifest: cloneManifest(current.manifest), phase: current.phase,
				lastError: current.lastError, failureCode: current.failureCode,
				generation: current.generation, generationHash: current.generationHash,
				lastGoodGeneration: current.lastGoodGeneration, logRing: current.logRing,
			},
		})
		current.supervisionStopped = true
		delete(m.scopedExtensions, key)
	}
	m.mu.Unlock()
	if len(retired) == 0 {
		return nil
	}
	slices.SortFunc(retired, func(left, right retiredWorkspaceProfile) int {
		return compareInstanceKeys(left.definition.key, right.definition.key)
	})
	transaction.add("workspace profile runtimes", func(cleanupCtx context.Context) error {
		return m.restoreWorkspaceProfileRuntimes(cleanupCtx, retired)
	})
	var errs []error
	for _, item := range retired {
		errs = append(errs, m.stopManagedExtension(ctx, item.previous))
	}
	return errors.Join(errs...)
}

func (m *Manager) restoreWorkspaceProfileRuntimes(ctx context.Context, retired []retiredWorkspaceProfile) error {
	m.mu.RLock()
	running := m.started && !m.stopping
	m.mu.RUnlock()
	if !running {
		return nil
	}
	if err := m.retryPendingCleanups(ctx); err != nil {
		return err
	}
	var errs []error
	for _, item := range retired {
		errs = append(errs, m.restoreWorkspaceProfileRuntime(ctx, item))
	}
	return errors.Join(errs...)
}

func (m *Manager) restoreWorkspaceProfileRuntime(ctx context.Context, item retiredWorkspaceProfile) error {
	definition := item.definition
	if !item.active {
		m.mu.Lock()
		m.scopedExtensions[definition.key] = definition
		m.mu.Unlock()
		return nil
	}
	if err := m.validateExtension(definition); err != nil {
		return err
	}
	prepared, err := m.prepareExtensionStartup(ctx, definition)
	if err != nil {
		return err
	}
	return m.commitPreparedExtensionWithPublish(ctx, definition, prepared, func() {
		m.scopedExtensions[definition.key] = definition
	})
}

func (m *Manager) restoreUnlinkedDevelopmentRuntime(
	ctx context.Context, key InstanceKey, link *DevLink, previous *managedExtension,
) error {
	m.mu.RLock()
	running := m.started && !m.stopping
	m.mu.RUnlock()
	if !running || previous == nil {
		return nil
	}
	if err := m.retryPendingCleanups(ctx); err != nil {
		return err
	}
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, link.BundleGeneration)
	if err != nil {
		return err
	}
	_, err = m.activateDevelopmentLinkLocked(ctx, key, link, verified)
	return err
}
