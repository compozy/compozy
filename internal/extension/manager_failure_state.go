package extensionpkg

import (
	"context"
	"errors"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
)

const maxExtensionFailureLogBytes = 1024

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

	m.logger.Error(
		"extension.lifecycle.failed",
		managerExtensionKey,
		name,
		"phase",
		phase,
		"error",
		diagnostics.RedactAndBound(err.Error(), maxExtensionFailureLogBytes),
	)
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
) (time.Duration, *managedInstanceIdentity, bool) {
	m.mu.Lock()
	ext := m.instanceLocked(key)
	if ext == nil || m.stopping || ext.supervisionStopped || owner != nil && ext != owner ||
		expectedProcess != nil && ext.process != expectedProcess {
		m.mu.Unlock()
		return 0, nil, false
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
	capabilityGrantID := ext.capabilityGrantID
	ext.capabilityGrantID = ""
	instanceIDs := managedBridgeInstanceIDs(ext)
	failures := ext.consecutiveFailures
	name := key.runtimeID()
	if ext.consecutiveFailures >= m.restartFailureThreshold {
		identity := managedInstanceIdentity{
			key:          key.Normalize(),
			owner:        ext,
			generation:   ext.generation,
			sessionNonce: ext.sessionNonce,
		}
		m.mu.Unlock()
		if capabilityGrantID != "" {
			m.capChecker.Unregister(capabilityGrantID)
		}
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
		return 0, &identity, true
	}

	ext.restartBackoff = restartBackoff(ext.consecutiveFailures, m.restartBackoffMax)
	backoff := ext.restartBackoff
	m.mu.Unlock()
	if capabilityGrantID != "" {
		m.capChecker.Unregister(capabilityGrantID)
	}
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
	return backoff, nil, true
}

func (m *Manager) disableExtension(name string, reason error) {
	m.disableInstance(GlobalInstanceKey(name), reason)
}

func (m *Manager) disableInstance(key InstanceKey, reason error) {
	m.mu.RLock()
	ext := m.instanceLocked(key)
	if ext == nil {
		m.mu.RUnlock()
		return
	}
	identity := managedInstanceIdentity{
		key:          key.Normalize(),
		owner:        ext,
		generation:   ext.generation,
		sessionNonce: ext.sessionNonce,
	}
	m.mu.RUnlock()
	m.disableOwnedInstance(identity, reason)
}

type managedInstanceIdentity struct {
	key          InstanceKey
	owner        *managedExtension
	generation   int64
	sessionNonce string
}

func (m *Manager) disableOwnedInstance(identity managedInstanceIdentity, reason error) {
	m.mu.RLock()
	if !m.matchesInstanceIdentityLocked(identity) {
		m.mu.RUnlock()
		return
	}
	ext := identity.owner
	capabilityGrantID := ext.capabilityGrantID
	instanceIDs := managedBridgeInstanceIDs(ext)
	m.mu.RUnlock()

	if capabilityGrantID != "" {
		m.capChecker.Unregister(capabilityGrantID)
	}
	if identity.sessionNonce != "" {
		if err := m.resetExtensionResourceSourceOrRetain(
			m.lifecycleContext(),
			identity.key,
			extensionResourceSource(identity.key),
			identity.sessionNonce,
		); err != nil {
			reason = errors.Join(reason, err)
		}
	}
	if identity.key.IsGlobal() && m.registry != nil {
		if err := m.registry.Disable(identity.key.Name); err != nil {
			reason = errors.Join(reason, err)
		}
	}

	m.mu.Lock()
	if !m.matchesInstanceIdentityLocked(identity) {
		m.mu.Unlock()
		return
	}
	ext.info.Enabled = false
	ext.phase = ExtensionPhaseRecover
	ext.lastError = reason.Error()
	ext.active = false
	ext.process = nil
	ext.awaitingStability = false
	ext.registered = false
	ext.sessionNonce = ""
	ext.capabilityGrantID = ""
	cleanups := ext.redactionCleanups
	ext.redactionCleanups = nil
	m.mu.Unlock()
	runExtensionRedactionCleanups(cleanups)

	m.reportBridgeRuntimeIssues(instanceIDs, bridgepkg.BridgeStatusError, reason)
}

func (m *Manager) matchesInstanceIdentityLocked(identity managedInstanceIdentity) bool {
	ext := m.instanceLocked(identity.key)
	return ext == identity.owner && ext != nil && ext.generation == identity.generation &&
		ext.sessionNonce == identity.sessionNonce
}

func (m *Manager) unregisterResources(ctx context.Context, ext *managedExtension) error {
	if ext == nil {
		return nil
	}
	m.mu.Lock()
	key := ext.instanceKey()
	capabilityGrantID := ext.capabilityGrantID
	ext.capabilityGrantID = ""
	sessionNonce := ext.sessionNonce
	ext.sessionNonce = ""
	ext.registered = false
	m.mu.Unlock()
	if capabilityGrantID != "" {
		m.capChecker.Unregister(capabilityGrantID)
	}

	return m.resetExtensionResourceSourceOrRetain(
		ctx,
		key,
		extensionResourceSource(key),
		sessionNonce,
	)
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
