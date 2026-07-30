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
	resolvedID := strings.TrimSpace(workspace.WorkspaceID)
	if resolvedID == "" {
		resolvedID = strings.TrimSpace(workspace.ID)
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
			Name:         manifest.Name,
			Version:      manifest.Version,
			Source:       SourceWorkspace,
			Enabled:      true,
			ManifestPath: verified.ManifestPath,
			Capabilities: manifest.Capabilities,
			Permissions:  manifest.Permissions,
			Checksum:     verified.GenerationHash,
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
	if err := m.startOne(ctx, candidate); err != nil {
		m.discardDevCandidate(ctx, candidate)
		return nil, err
	}
	return candidate, nil
}

func (m *Manager) swapDevInstance(key InstanceKey, candidate *managedExtension) {
	m.mu.Lock()
	m.devExtensions[key.Normalize()] = candidate
	m.mu.Unlock()
}

func (m *Manager) startInstanceSupervisor(ext *managedExtension) {
	if ext == nil {
		return
	}
	m.mu.Lock()
	if ext.process == nil {
		m.mu.Unlock()
		return
	}
	ext.deferSupervision = false
	ext.supervisionStopped = false
	generation := ext.generation
	m.mu.Unlock()
	m.wg.Add(1)
	go m.superviseInstance(ext.instanceKey(), generation)
}

func (m *Manager) discardDevCandidate(ctx context.Context, candidate *managedExtension) {
	if candidate == nil {
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
	runExtensionRedactionCleanups(candidate.redactionCleanups)
}

func (m *Manager) stopReplacedDevProcess(ctx context.Context, previous *managedExtension) {
	if previous == nil {
		return
	}
	key := previous.instanceKey()
	m.mu.Lock()
	previous.supervisionStopped = true
	proc := previous.process
	previous.process = nil
	previous.active = false
	previous.awaitingStability = false
	cleanups := previous.redactionCleanups
	previous.redactionCleanups = nil
	m.mu.Unlock()
	if proc != nil {
		if err := shutdownProcessWithTimeout(ctx, proc, m.defaultShutdownTimeout); err != nil {
			m.logger.Warn("extension: stop replaced dev generation", "extension", key.runtimeID(), "error", err)
		}
	}
	runExtensionRedactionCleanups(cleanups)
}

func (m *Manager) restoreInstanceAuthority(ext *managedExtension) {
	if ext == nil || ext.manifest == nil {
		return
	}
	if _, err := m.capChecker.RegisterForSession(
		ext.instanceKey().runtimeID(),
		ext.info.Source,
		ext.manifest,
		ext.maxResourceScope(),
	); err != nil {
		m.logger.Error(
			"extension: restore prior generation authority",
			"extension", ext.instanceKey().runtimeID(),
			"error", err,
		)
	}
}

func (m *Manager) restartLastGood(
	ctx context.Context,
	key InstanceKey,
	link *DevLink,
	current *managedExtension,
	activationErr error,
) (*Extension, error) {
	m.stopReplacedDevProcess(ctx, current)
	prior, verifyErr := m.resolveDevGeneration(ctx, key, link.OriginPath, link.BundleGeneration)
	if verifyErr != nil {
		return nil, errors.Join(activationErr, fmt.Errorf("extension: verify last-good generation: %w", verifyErr))
	}
	restarted, restartErr := m.startVerifiedDevCandidate(ctx, key, prior)
	if restartErr != nil {
		return nil, errors.Join(activationErr, fmt.Errorf("extension: restart last-good generation: %w", restartErr))
	}
	restarted.lastGoodGeneration = prior.GenerationHash
	restarted.failureCode = extensionFailureActivationFailed
	restarted.lastError = fmt.Sprintf(
		"activation_failed; running %s: %s",
		prior.GenerationHash,
		activationErr,
	)
	restarted.phase = ExtensionPhase("errored")
	m.swapDevInstance(key, restarted)
	m.startInstanceSupervisor(restarted)
	return m.cloneExtension(restarted), activationErr
}
