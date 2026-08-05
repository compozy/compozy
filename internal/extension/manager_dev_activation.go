package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

func (m *Manager) resolveDevGeneration(
	ctx context.Context,
	key InstanceKey,
	originPath string,
	generationHash string,
) (*verifiedDevGeneration, error) {
	if m.workspaceResolver == nil {
		return nil, errors.New("extension: workspace resolver is required for development extensions")
	}
	workspace, err := m.workspaceResolver.Resolve(ctx, key.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("extension: resolve development workspace %q: %w", key.WorkspaceID, err)
	}
	resolvedID := strings.TrimSpace(workspace.ID)
	if resolvedID == "" {
		return nil, errors.New("extension: resolved workspace registration id is required")
	}
	if resolvedID != key.WorkspaceID {
		return nil, fmt.Errorf("%w: resolved workspace %q", ErrExtensionWorkspaceDenied, resolvedID)
	}
	canonicalOrigin, err := canonicalizeDevOrigin(workspace.RootDir, originPath)
	if err != nil {
		return nil, err
	}
	verified, err := verifyDevGeneration(canonicalOrigin, generationHash)
	if err != nil {
		return nil, err
	}
	if verified.Manifest.Name != key.Name {
		return nil, fmt.Errorf(
			"%w: generation manifest name %q does not match %q",
			ErrExtensionGenerationInvalid,
			verified.Manifest.Name,
			key.Name,
		)
	}
	return verified, nil
}

func managedDevExtension(
	key InstanceKey,
	verified *verifiedDevGeneration,
	ring *ExtensionLogRing,
) *managedExtension {
	manifest := verified.Manifest
	return &managedExtension{
		key: key,
		info: ExtensionInfo{
			Name:                     manifest.Name,
			Version:                  manifest.Version,
			Source:                   SourceWorkspace,
			Enabled:                  true,
			ManifestPath:             verified.ManifestPath,
			Capabilities:             manifest.Capabilities,
			Permissions:              manifest.Permissions,
			Checksum:                 verified.GenerationHash,
			NetworkRequirementDigest: verified.NetworkRequirementDigest,
		},
		rootDir:          verified.GenerationDir,
		manifest:         manifest,
		phase:            ExtensionPhaseDiscover,
		generationHash:   verified.GenerationHash,
		logRing:          ring,
		deferSupervision: true,
	}
}

func (m *Manager) startVerifiedDevCandidate(
	ctx context.Context,
	key InstanceKey,
	verified *verifiedDevGeneration,
) (*managedExtension, error) {
	candidate := managedDevExtension(key, verified, m.logRingFor(key))
	if err := m.validateExtension(candidate); err != nil {
		return nil, err
	}
	prepared, err := m.prepareExtensionStartup(ctx, candidate)
	if err != nil {
		m.discardDevCandidate(ctx, candidate)
		return nil, err
	}
	candidate.startup = prepared
	return candidate, nil
}

func (m *Manager) activateAndPublishDevCandidate(
	ctx context.Context,
	key InstanceKey,
	candidate *managedExtension,
) (*managedExtension, error) {
	if candidate == nil || candidate.startup == nil {
		return nil, errors.New("extension: prepared development candidate is required")
	}
	key = key.Normalize()
	var previous *managedExtension
	err := m.commitPreparedExtensionWithPublish(ctx, candidate, candidate.startup, func() {
		previous = m.instanceLocked(key)
		m.devExtensions[key] = candidate
		candidate.deferSupervision = false
		candidate.supervisionStopped = false
	})
	if err != nil {
		return nil, err
	}
	return previous, nil
}

func (m *Manager) swapDevInstance(key InstanceKey, candidate *managedExtension) {
	m.mu.Lock()
	m.devExtensions[key.Normalize()] = candidate
	m.mu.Unlock()
}

func (m *Manager) discardDevCandidate(ctx context.Context, candidate *managedExtension) {
	if candidate == nil {
		return
	}
	if candidate.startup != nil && candidate.startup.transaction != nil {
		if err := candidate.startup.transaction.rollback(ctx, nil); err != nil {
			m.logger.Warn(
				"extension: discard dev candidate",
				"extension", candidate.instanceKey().runtimeID(),
				"error", err,
			)
		}
		candidate.startup = nil
		return
	}
	if candidate.process != nil {
		if err := shutdownProcessWithTimeout(ctx, candidate.process, m.defaultShutdownTimeout); err != nil {
			m.logger.Warn(
				"extension: discard dev candidate",
				"extension", candidate.instanceKey().runtimeID(),
				"error", err,
			)
		}
	}
	if processTerminal(candidate.process) {
		runExtensionRedactionCleanups(candidate.redactionCleanups)
	}
}

func (m *Manager) stopReplacedDevProcess(ctx context.Context, previous *managedExtension) {
	if previous == nil {
		return
	}
	key := previous.instanceKey()
	m.mu.Lock()
	previous.supervisionStopped = true
	proc := previous.process
	capabilityGrantID := previous.capabilityGrantID
	previous.process = nil
	previous.capabilityGrantID = ""
	previous.active = false
	previous.awaitingStability = false
	cleanups := previous.redactionCleanups
	previous.redactionCleanups = nil
	m.mu.Unlock()
	if capabilityGrantID != "" {
		m.capChecker.Unregister(capabilityGrantID)
	}
	if proc != nil {
		if err := shutdownProcessWithTimeout(ctx, proc, m.defaultShutdownTimeout); err != nil {
			m.logger.Warn("extension: stop replaced dev generation", "extension", key.runtimeID(), "error", err)
		}
	}
	if processTerminal(proc) {
		runExtensionRedactionCleanups(cleanups)
	} else if proc != nil {
		m.retainPendingCleanup(pendingExtensionCleanup{
			key:               key,
			process:           proc,
			redactionCleanups: cleanups,
		})
	}
}

func (m *Manager) restartLastGood(
	key InstanceKey,
	current *managedExtension,
	activationErr error,
) (*Extension, error) {
	if current == nil {
		return nil, activationErr
	}
	m.mu.Lock()
	if m.instanceLocked(key) == current {
		current.failureCode = extensionFailureActivationFailed
		current.lastError = fmt.Sprintf(
			"activation_failed; running %s: %s",
			current.lastGoodGeneration,
			activationErr,
		)
	}
	m.mu.Unlock()
	return m.cloneExtension(current), activationErr
}
