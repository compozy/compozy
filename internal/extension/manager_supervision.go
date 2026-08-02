package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"path/filepath"

	"strings"
	"time"
)

func (m *Manager) startOne(ctx context.Context, ext *managedExtension) error {
	if err := m.discoverExtension(ext); err != nil {
		return err
	}
	if err := m.parseExtension(ext); err != nil {
		return err
	}
	if err := m.validateExtension(ext); err != nil {
		return err
	}
	prepared, err := m.prepareExtensionStartup(ctx, ext)
	if err != nil {
		return err
	}
	return m.commitPreparedExtension(ctx, ext, prepared)
}

func (m *Manager) discoverExtension(ext *managedExtension) error {
	manifestPath := strings.TrimSpace(ext.info.ManifestPath)
	if manifestPath == "" {
		err := errors.New("manifest path is required")
		m.setFailure(ext, ExtensionPhaseDiscover, err)
		return phaseError(ext.info.Name, ExtensionPhaseDiscover, err)
	}

	rootDir := filepath.Dir(manifestPath)
	if rootDir == "." || rootDir == "" {
		err := fmt.Errorf("invalid manifest path %q", manifestPath)
		m.setFailure(ext, ExtensionPhaseDiscover, err)
		return phaseError(ext.info.Name, ExtensionPhaseDiscover, err)
	}

	ext.rootDir = rootDir
	ext.phase = ExtensionPhaseDiscover
	return nil
}

func (m *Manager) parseExtension(ext *managedExtension) error {
	manifest, err := loadManifestAtPath(ext.info.ManifestPath)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseParse, err)
		return phaseError(ext.info.Name, ExtensionPhaseParse, err)
	}

	ext.manifest = manifest
	ext.phase = ExtensionPhaseParse
	return nil
}

func (m *Manager) validateExtension(ext *managedExtension) error {
	if ext.manifest == nil {
		err := errors.New("manifest is required")
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}
	if ext.info.Name != ext.manifest.Name {
		err := fmt.Errorf("registry name %q does not match manifest name %q", ext.info.Name, ext.manifest.Name)
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}
	if ext.info.Version != "" && ext.info.Version != ext.manifest.Version {
		err := fmt.Errorf(
			"registry version %q does not match manifest version %q",
			ext.info.Version,
			ext.manifest.Version,
		)
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}
	if requiresSubprocess(ext.manifest) && strings.TrimSpace(ext.manifest.Subprocess.Command) == "" {
		err := errors.New("subprocess command is required when runtime capabilities or permissions are declared")
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}

	grant, err := m.capChecker.Resolve(
		ext.info.Source,
		ext.manifest,
		ext.maxResourceScope(),
	)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}
	ext.pendingGrant = grant
	ext.phase = ExtensionPhaseValidate
	return nil
}

func (m *Manager) superviseInstance(key InstanceKey, generation int64) {
	defer m.wg.Done()

	for {
		owner, proc, interval, ok := m.currentSupervisedInstance(key, generation)
		if !ok {
			return
		}

		shouldRecover, reason := m.monitorInstanceProcess(key, generation, proc, interval)
		if !shouldRecover {
			return
		}

		nextGeneration, recovered := m.recoverOwnedInstance(key, owner, proc, reason)
		if !recovered {
			return
		}
		generation = nextGeneration
	}
}

func (m *Manager) monitorProcess(
	name string,
	generation int64,
	proc processHandle,
	healthInterval time.Duration,
) (bool, error) {
	return m.monitorInstanceProcess(GlobalInstanceKey(name), generation, proc, healthInterval)
}

func (m *Manager) monitorInstanceProcess(
	key InstanceKey,
	generation int64,
	proc processHandle,
	healthInterval time.Duration,
) (bool, error) {
	ticker := time.NewTicker(m.healthPollInterval(healthInterval))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.shouldStopInstanceSupervision(key, generation, proc) {
				return false, nil
			}

			health := proc.HealthState()
			if !health.Healthy {
				reason := fmt.Errorf("health check failed: %s", strings.TrimSpace(health.Message))
				if strings.TrimSpace(health.LastError) != "" {
					reason = fmt.Errorf("%w: %s", reason, health.LastError)
				}

				if shutdownErr := shutdownProcessWithTimeout(
					m.lifecycleContext(),
					proc,
					m.shutdownDeadlineForInstance(key, generation),
				); shutdownErr != nil {
					reason = errors.Join(reason, shutdownErr)
				}
				return true, reason
			}
			if !health.LastCheckedAt.IsZero() {
				m.markInstanceStable(key, generation)
			}
		case <-proc.Done():
			if m.shouldStopInstanceSupervision(key, generation, proc) {
				return false, nil
			}

			err := proc.Wait()
			if err == nil {
				err = errors.New("process exited unexpectedly")
			}
			return true, err
		case <-m.lifecycleDone():
			return false, nil
		}
	}
}

func (m *Manager) recoverOwnedInstance(
	key InstanceKey,
	owner *managedExtension,
	expectedProcess processHandle,
	reason error,
) (int64, bool) {
	for {
		backoff, disableIdentity, ok := m.recordOwnedInstanceFailure(key, owner, expectedProcess, reason)
		expectedProcess = nil
		if !ok {
			return 0, false
		}
		if disableIdentity != nil {
			m.disableOwnedInstance(*disableIdentity, reason)
			return 0, false
		}
		if !m.waitBackoff(backoff) {
			return 0, false
		}

		m.mu.RLock()
		ext := m.instanceLocked(key)
		ownsInstance := ext != nil && (owner == nil || ext == owner) && !ext.supervisionStopped
		m.mu.RUnlock()
		if !ownsInstance {
			return 0, false
		}
		transaction := newExtensionStartupTransaction(m, ext)
		grant := ext.pendingGrant
		launched, resourceSession, err := m.launchStartupRuntime(m.lifecycleContext(), ext, grant, transaction)
		if err != nil {
			reason = transaction.rollback(m.lifecycleContext(), err)
			continue
		}
		if err := m.activatePreparedSourceSession(m.lifecycleContext(), resourceSession, transaction); err != nil {
			reason = transaction.rollback(m.lifecycleContext(), err)
			continue
		}

		nextGeneration, name, source, accepted := m.acceptRecoveredRuntime(key, owner, ext, launched)
		if !accepted {
			if discardErr := transaction.rollback(
				m.lifecycleContext(),
				errors.New("extension: recovered runtime rejected"),
			); discardErr != nil {
				m.logger.Warn(
					"extension.lifecycle.shutdown_failed",
					managerExtensionKey, key.runtimeID(),
					"recovered", false,
					"error", discardErr,
				)
			}
			return 0, false
		}
		transaction.commit()

		m.logger.Info(
			"extension.lifecycle.loaded",
			managerExtensionKey,
			name,
			"workspace_id",
			key.WorkspaceID,
			"source",
			source,
			"recovered",
			true,
		)

		return nextGeneration, true
	}
}

func (m *Manager) acceptRecoveredRuntime(
	key InstanceKey,
	owner *managedExtension,
	ext *managedExtension,
	launched launchedRuntime,
) (int64, string, string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping || m.instanceLocked(key) != ext || (owner != nil && ext != owner) ||
		ext.supervisionStopped || (ext.generation == 0 && !ext.info.Enabled) {
		return 0, "", "", false
	}

	ext.process = launched.process
	ext.initialize = &launched.response
	ext.runtime = launched.runtime
	ext.healthInterval = launched.healthInterval
	ext.sessionNonce = launched.sessionNonce
	ext.capabilityGrantID = launched.capabilityGrantID
	ext.redactionCleanups = launched.redactionCleanups
	ext.awaitingStability = true
	ext.active = true
	ext.phase = ExtensionPhaseActivate
	ext.lastError = ""
	ext.lastStartedAt = m.now()
	ext.generation++
	return ext.generation, ext.info.Name, ext.info.Source.String(), true
}
