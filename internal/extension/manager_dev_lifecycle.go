package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	extensionFailureMissingOrigin    = "missing_origin"
	extensionFailureActivationFailed = "activation_failed"
)

// LinkDevelopmentFromOrigin derives the authored name from a verified generation.
func (m *Manager) LinkDevelopmentFromOrigin(
	ctx context.Context,
	workspaceID string,
	originPath string,
	generationHash string,
) (*Extension, error) {
	if m == nil {
		return nil, ErrManagerRequired
	}
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("extension: workspace id is required for a development link")
	}
	if m.workspaceResolver == nil {
		return nil, errors.New("extension: workspace resolver is required for development extensions")
	}
	workspace, err := m.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("extension: resolve development workspace %q: %w", workspaceID, err)
	}
	canonicalOrigin, err := canonicalizeDevOrigin(workspace.RootDir, originPath)
	if err != nil {
		return nil, err
	}
	verified, err := verifyDevGeneration(canonicalOrigin, generationHash)
	if err != nil {
		return nil, err
	}
	return m.LinkDevelopment(
		ctx,
		InstanceKey{Name: verified.Manifest.Name, WorkspaceID: workspaceID},
		canonicalOrigin,
		generationHash,
	)
}

// LinkDevelopment verifies, activates, and persists one workspace-local generation.
func (m *Manager) LinkDevelopment(
	ctx context.Context,
	key InstanceKey,
	originPath string,
	generationHash string,
) (*Extension, error) {
	if err := m.checkDevOperation(ctx, key); err != nil {
		return nil, err
	}
	key = key.Normalize()
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()

	verified, err := m.resolveDevGeneration(ctx, key, originPath, generationHash)
	if err != nil {
		return nil, err
	}
	current, _ := m.lookupInstance(key)
	candidate, err := m.startVerifiedDevCandidate(ctx, key, verified)
	if err != nil {
		return nil, err
	}
	link, err := m.registry.LinkDev(DevLinkRequest{
		Name:           key.Name,
		WorkspaceID:    key.WorkspaceID,
		OriginPath:     verified.OriginPath,
		GenerationHash: verified.GenerationHash,
	})
	if err != nil {
		m.discardDevCandidate(ctx, candidate)
		m.restoreInstanceAuthority(current)
		return nil, err
	}
	candidate.lastGoodGeneration = verified.GenerationHash
	m.swapDevInstance(key, candidate)
	m.startInstanceSupervisor(candidate)
	if current != nil {
		m.stopReplacedDevProcess(ctx, current)
	}
	snapshot := m.cloneExtension(candidate)
	snapshot.DevLink = link
	_, publishedErr := m.registry.Get(key.Name)
	snapshot.OverridesPublished = publishedErr == nil
	return snapshot, nil
}

// ReloadExtension atomically replaces one active dev generation.
func (m *Manager) ReloadExtension(
	ctx context.Context,
	key InstanceKey,
	generationHash string,
) (*Extension, error) {
	if err := m.checkDevOperation(ctx, key); err != nil {
		return nil, err
	}
	key = key.Normalize()
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()

	link, err := m.registry.GetDevLink(key.Name, key.WorkspaceID)
	if err != nil {
		return nil, err
	}
	current, ok := m.lookupInstance(key)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrExtensionNotDevLinked, key.runtimeID())
	}
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, generationHash)
	if err != nil {
		return nil, err
	}
	candidate, activationErr := m.startVerifiedDevCandidate(ctx, key, verified)
	if activationErr != nil {
		return m.restartLastGood(ctx, key, link, current, activationErr)
	}
	updatedLink, err := m.registry.LinkDev(DevLinkRequest{
		Name:           key.Name,
		WorkspaceID:    key.WorkspaceID,
		OriginPath:     verified.OriginPath,
		GenerationHash: verified.GenerationHash,
	})
	if err != nil {
		m.discardDevCandidate(ctx, candidate)
		m.restoreInstanceAuthority(current)
		return nil, err
	}
	candidate.lastGoodGeneration = verified.GenerationHash
	m.swapDevInstance(key, candidate)
	m.startInstanceSupervisor(candidate)
	m.stopReplacedDevProcess(ctx, current)
	snapshot := m.cloneExtension(candidate)
	snapshot.DevLink = updatedLink
	_, publishedErr := m.registry.Get(key.Name)
	snapshot.OverridesPublished = publishedErr == nil
	return snapshot, nil
}

// UnlinkDevelopment removes and stops only one workspace-local instance.
func (m *Manager) UnlinkDevelopment(ctx context.Context, key InstanceKey) error {
	if err := m.checkDevOperation(ctx, key); err != nil {
		return err
	}
	key = key.Normalize()
	coordinator := m.coordinatorFor(key)
	coordinator.Lock()
	defer coordinator.Unlock()
	if err := m.registry.UnlinkDev(key.Name, key.WorkspaceID); err != nil {
		return err
	}
	current, _ := m.lookupInstance(key)
	m.mu.Lock()
	m.deleteInstanceLocked(key)
	delete(m.devLogs, key)
	m.mu.Unlock()
	if current != nil {
		return m.stopManagedExtension(ctx, current)
	}
	return nil
}

// Logs returns retained redacted stderr entries newer than afterSequence.
func (m *Manager) Logs(key InstanceKey, afterSequence int64) ([]ExtensionLogEntry, error) {
	if m == nil {
		return nil, ErrManagerRequired
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if !key.IsGlobal() {
		if _, err := m.registry.GetDevLink(key.Name, key.WorkspaceID); err != nil {
			return nil, err
		}
	} else if _, err := m.registry.Get(key.Name); err != nil {
		return nil, err
	}
	m.mu.RLock()
	ring := m.devLogs[key]
	m.mu.RUnlock()
	if ring == nil {
		return []ExtensionLogEntry{}, nil
	}
	return ring.snapshot(afterSequence), nil
}

func (m *Manager) startDevLinkOnBoot(ctx context.Context, link DevLink) {
	key := InstanceKey{Name: link.ExtensionName, WorkspaceID: link.WorkspaceID}.Normalize()
	verified, err := m.resolveDevGeneration(ctx, key, link.OriginPath, link.BundleGeneration)
	if err != nil {
		failureCode := extensionFailureActivationFailed
		if errors.Is(err, ErrExtensionDevOriginMissing) || errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) ||
			errors.Is(err, os.ErrNotExist) ||
			strings.Contains(err.Error(), "development origin") {
			failureCode = extensionFailureMissingOrigin
		}
		ext := &managedExtension{
			key:                key,
			info:               ExtensionInfo{Name: key.Name, Source: SourceWorkspace, Enabled: true},
			phase:              ExtensionPhase("errored"),
			lastError:          err.Error(),
			failureCode:        failureCode,
			generationHash:     link.BundleGeneration,
			lastGoodGeneration: link.BundleGeneration,
			logRing:            m.logRingFor(key),
		}
		m.mu.Lock()
		m.devExtensions[key] = ext
		m.mu.Unlock()
		return
	}
	ext, err := m.startVerifiedDevCandidate(ctx, key, verified)
	if err != nil {
		ext = managedDevExtension(key, verified, m.logRingFor(key))
		ext.phase = ExtensionPhase("errored")
		ext.failureCode = extensionFailureActivationFailed
		ext.lastError = err.Error()
	}
	ext.lastGoodGeneration = link.BundleGeneration
	m.swapDevInstance(key, ext)
	if err == nil {
		m.startInstanceSupervisor(ext)
	}
}

func (m *Manager) checkDevOperation(ctx context.Context, key InstanceKey) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil {
		return ErrManagerRequired
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return err
	}
	if key.IsGlobal() {
		return errors.New("extension: development operations require a workspace instance")
	}
	if m.registry == nil {
		return ErrRegistryRequired
	}
	m.mu.RLock()
	started := m.started
	stopping := m.stopping
	m.mu.RUnlock()
	if !started || stopping {
		return errors.New("extension: manager is not accepting development operations")
	}
	return nil
}
