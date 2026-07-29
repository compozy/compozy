package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	eventspkg "github.com/compozy/compozy/internal/events"
)

func (m *Manager) setFailure(ext *managedExtension, phase ExtensionPhase, err error) {
	if ext == nil || err == nil {
		return
	}

	m.mu.Lock()
	ext.phase = phase
	ext.lastError = err.Error()
	ext.active = false
	name := ext.info.Name
	m.mu.Unlock()

	m.logger.Error("extension.lifecycle.failed", managerExtensionKey, name, "phase", phase, "error", err)
}

func (m *Manager) lookupManaged(name string) (*managedExtension, bool) {
	return m.lookupInstance(GlobalInstanceKey(name))
}

func (m *Manager) currentProcess(name string, generation int64) (processHandle, time.Duration, bool) {
	return m.currentInstanceProcess(GlobalInstanceKey(name), generation)
}

func (m *Manager) currentInstanceProcess(key InstanceKey, generation int64) (processHandle, time.Duration, bool) {
	_, proc, interval, ok := m.currentSupervisedInstance(key, generation)
	return proc, interval, ok
}

func (m *Manager) currentSupervisedInstance(
	key InstanceKey,
	generation int64,
) (*managedExtension, processHandle, time.Duration, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext := m.instanceLocked(key)
	if ext == nil || ext.process == nil || ext.generation != generation || ext.supervisionStopped {
		return nil, nil, 0, false
	}
	return ext, ext.process, ext.healthInterval, true
}

func (m *Manager) shouldStopSupervision(name string, generation int64, proc processHandle) bool {
	return m.shouldStopInstanceSupervision(GlobalInstanceKey(name), generation, proc)
}

func (m *Manager) shouldStopInstanceSupervision(key InstanceKey, generation int64, proc processHandle) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopping {
		return true
	}
	ext := m.instanceLocked(key)
	return ext == nil || ext.process == nil || ext.process != proc || ext.generation != generation ||
		ext.supervisionStopped
}

func (m *Manager) recordOwnedInstanceFailure(
	key InstanceKey,
	owner *managedExtension,
	expectedProcess processHandle,
	reason error,
) (time.Duration, bool, bool) {
	m.mu.Lock()
	ext := m.instanceLocked(key)
	if ext == nil || m.stopping || ext.supervisionStopped || owner != nil && ext != owner ||
		expectedProcess != nil && ext.process != expectedProcess {
		m.mu.Unlock()
		return 0, false, false
	}

	ext.process = nil
	ext.active = false
	ext.awaitingStability = false
	ext.phase = ExtensionPhaseRecover
	ext.lastExitedAt = m.now()
	ext.lastError = reason.Error()
	ext.consecutiveFailures++
	cleanups := ext.redactionCleanups
	ext.redactionCleanups = nil
	instanceIDs := managedBridgeInstanceIDs(ext)
	failures := ext.consecutiveFailures
	name := key.runtimeID()
	if ext.consecutiveFailures >= m.restartFailureThreshold {
		m.mu.Unlock()
		runExtensionRedactionCleanups(cleanups)
		m.reportBridgeRuntimeIssues(instanceIDs, bridgepkg.BridgeStatusError, reason)
		m.logger.Error(
			"extension.lifecycle.failed",
			managerExtensionKey,
			name,
			"phase",
			ExtensionPhaseRecover,
			"error",
			reason,
			"consecutive_failures",
			failures,
		)
		return 0, true, true
	}

	ext.restartBackoff = restartBackoff(ext.consecutiveFailures, m.restartBackoffMax)
	backoff := ext.restartBackoff
	m.mu.Unlock()
	runExtensionRedactionCleanups(cleanups)
	m.reportBridgeRuntimeIssues(instanceIDs, bridgepkg.BridgeStatusDegraded, reason)
	if eventErr := recordExtensionLifecycleEvent(m.lifecycleContext(), m.lifecycleEventSink, LifecycleEvent{
		Type: eventspkg.ExtensionCrashLoopBackoff, ExtensionName: key.Name,
		WorkspaceID: key.WorkspaceID,
	}); eventErr != nil {
		m.logger.Error("extension: record crash-loop backoff event", managerExtensionKey, name, "error", eventErr)
	}

	m.logger.Warn(
		"extension.lifecycle.failed",
		managerExtensionKey, name,
		"phase", ExtensionPhaseRecover,
		"error", reason,
		"consecutive_failures", failures,
		"restart_backoff_ms", backoff.Milliseconds(),
	)
	return backoff, false, true
}

func (m *Manager) disableExtension(name string, reason error) {
	m.disableInstance(GlobalInstanceKey(name), reason)
}

func (m *Manager) disableInstance(key InstanceKey, reason error) {
	ext, ok := m.lookupInstance(key)
	if !ok {
		return
	}
	instanceIDs := managedBridgeInstanceIDs(ext)
	if err := m.unregisterResources(m.lifecycleContext(), ext); err != nil {
		reason = errors.Join(reason, err)
	}

	if key.IsGlobal() {
		if err := m.registry.Disable(key.Name); err != nil {
			reason = errors.Join(reason, err)
		}
	}

	m.mu.Lock()
	ext.info.Enabled = false
	ext.phase = ExtensionPhaseRecover
	ext.lastError = reason.Error()
	ext.active = false
	ext.process = nil
	ext.awaitingStability = false
	cleanups := ext.redactionCleanups
	ext.redactionCleanups = nil
	m.mu.Unlock()
	runExtensionRedactionCleanups(cleanups)

	m.reportBridgeRuntimeIssues(instanceIDs, bridgepkg.BridgeStatusError, reason)
}

func (m *Manager) unregisterResources(ctx context.Context, ext *managedExtension) error {
	if ext == nil {
		return nil
	}
	key := ext.instanceKey()
	m.capChecker.Unregister(key.runtimeID())

	if err := m.resetExtensionSource(ctx, key); err != nil {
		return err
	}

	m.mu.Lock()
	ext.registered = false
	ext.sessionNonce = ""
	m.mu.Unlock()
	return nil
}

func (m *Manager) resetExtensionSource(ctx context.Context, key InstanceKey) error {
	if m == nil || m.sourceSessions == nil {
		return nil
	}
	if ctx == nil {
		return ErrContextRequired
	}

	source := extensionResourceSource(key)
	if err := m.sourceSessions.ResetSource(ctx, extensionManagerResourceActor(), source); err != nil {
		return fmt.Errorf("extension: reset source session for %q: %w", source.ID, err)
	}
	return nil
}

func (m *Manager) markStable(name string, generation int64) {
	m.markInstanceStable(GlobalInstanceKey(name), generation)
}

func (m *Manager) markInstanceStable(key InstanceKey, generation int64) {
	m.mu.Lock()
	ext := m.instanceLocked(key)
	if ext == nil || ext.generation != generation || !ext.awaitingStability {
		m.mu.Unlock()
		return
	}
	instanceIDs := managedBridgeInstanceIDs(ext)
	ext.awaitingStability = false
	ext.consecutiveFailures = 0
	ext.restartBackoff = 0
	m.mu.Unlock()
	m.clearBridgeRuntimeIssues(instanceIDs)
}

func (m *Manager) statusLocked(ext *managedExtension) ExtensionStatus {
	var missingEnv []string
	missingEnvChecked := false
	if ext.manifest != nil {
		missingEnv = ext.manifest.MissingEnv(m.getenv)
		missingEnvChecked = len(ext.manifest.RequiresEnv) > 0
	}
	status := ExtensionStatus{
		Name:                ext.info.Name,
		WorkspaceID:         ext.instanceKey().WorkspaceID,
		Version:             ext.info.Version,
		Source:              ext.info.Source,
		Enabled:             ext.info.Enabled,
		MissingEnv:          missingEnv,
		MissingEnvChecked:   missingEnvChecked,
		Registered:          ext.registered,
		Active:              ext.active,
		Phase:               ext.phase,
		ConsecutiveFailures: ext.consecutiveFailures,
		RestartBackoff:      ext.restartBackoff,
		LastError:           ext.lastError,
		FailureCode:         ext.failureCode,
		GenerationHash:      ext.generationHash,
		LastGoodGeneration:  ext.lastGoodGeneration,
		LastStartedAt:       ext.lastStartedAt,
		LastExitedAt:        ext.lastExitedAt,
	}
	if ext.process != nil {
		status.PID = ext.process.PID()
		health := ext.process.HealthState()
		status.Healthy = health.Healthy
		status.HealthMessage = health.Message
		status.HealthLastCheckedAt = health.LastCheckedAt
	} else {
		status.Healthy = ext.active
	}
	return status
}
