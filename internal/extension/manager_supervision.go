package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"path/filepath"

	"strings"

	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/subprocess"
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
	if err := m.registerExtension(ctx, ext); err != nil {
		return err
	}
	if err := m.initializeExtension(ctx, ext); err != nil {
		return err
	}
	m.activateExtension(ext)
	return nil
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

	grant, err := m.capChecker.RegisterForSession(
		ext.instanceKey().runtimeID(),
		ext.info.Source,
		ext.manifest,
		ext.maxResourceScope(),
	)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}
	ext.grantedPermissions = grant.Permissions
	ext.grantedSecurity = grant.Security
	ext.grantedResourceKinds = grant.ResourceKinds
	ext.grantedResourceScopes = grant.ResourceScopes
	ext.phase = ExtensionPhaseValidate
	return nil
}

func (m *Manager) registerExtension(ctx context.Context, ext *managedExtension) error {
	if err := ctx.Err(); err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return err
	}

	skills, err := m.loadSkillResources(ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	loops, err := m.loadLoopResources(ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	agents, err := m.loadAgentResources(ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	automationJobs, automationTriggers, err := m.loadAutomationResources(ext, agents)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	layouts, err := m.loadLayoutResources(ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	hooks, err := m.loadHookResources(ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	bundles, err := m.loadBundleResources(ctx, ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	m.mu.Lock()
	ext.skills = skills
	ext.staticAgents = agents
	ext.agents = make([]compozyconfig.AgentDef, 0, len(agents))
	for _, agent := range agents {
		ext.agents = append(ext.agents, compozyconfig.CloneAgentDef(agent.Agent))
	}
	ext.automationJobs = automationJobs
	ext.automationTriggers = automationTriggers
	ext.layouts = layouts
	ext.hooks = hooks
	ext.bundles = bundles
	ext.registered = true
	ext.loops = loops
	ext.phase = ExtensionPhaseRegister
	m.mu.Unlock()
	return nil
}

func (m *Manager) initializeExtension(ctx context.Context, ext *managedExtension) error {
	if !requiresSubprocess(ext.manifest) {
		m.mu.Lock()
		ext.phase = ExtensionPhaseInitialize
		ext.active = false
		ext.lastError = ""
		m.mu.Unlock()
		return nil
	}

	launched, err := m.launchRuntime(ctx, ext)
	if err != nil {
		if errors.Is(err, ErrBridgeRuntimeDeferred) {
			m.mu.Lock()
			ext.process = nil
			ext.initialize = nil
			ext.runtime = subprocess.InitializeRuntime{}
			ext.healthInterval = 0
			ext.awaitingStability = false
			ext.lastStartedAt = time.Time{}
			ext.phase = ExtensionPhaseInitialize
			ext.lastError = ""
			m.mu.Unlock()
			return nil
		}
		m.setFailure(ext, ExtensionPhaseInitialize, err)
		return phaseError(ext.info.Name, ExtensionPhaseInitialize, err)
	}

	m.mu.Lock()
	ext.process = launched.process
	ext.initialize = &launched.response
	ext.runtime = launched.runtime
	ext.healthInterval = launched.healthInterval
	ext.sessionNonce = launched.sessionNonce
	ext.redactionCleanups = launched.redactionCleanups
	ext.awaitingStability = true
	ext.lastStartedAt = m.now()
	ext.phase = ExtensionPhaseInitialize
	ext.lastError = ""
	ext.generation++
	generation := ext.generation
	m.mu.Unlock()

	if !ext.deferSupervision {
		m.wg.Add(1)
		go m.superviseInstance(ext.instanceKey(), generation)
	}
	return nil
}

func (m *Manager) activateExtension(ext *managedExtension) {
	if ext == nil {
		return
	}

	m.mu.Lock()
	ext.phase = ExtensionPhaseActivate
	ext.active = ext.process != nil || !requiresSubprocess(ext.manifest)
	ext.restartBackoff = 0
	ext.lastError = ""
	name := ext.info.Name
	source := ext.info.Source.String()
	active := ext.active
	skillCount := len(ext.skills)
	agentCount := len(ext.agents)
	hookCount := len(ext.hooks)
	m.mu.Unlock()

	m.logger.Info(
		"extension.lifecycle.loaded",
		managerExtensionKey, name,
		"source", source,
		"active", active,
		"skill_count", skillCount,
		"agent_count", agentCount,
		"hook_count", hookCount,
	)
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
		backoff, disable, ok := m.recordOwnedInstanceFailure(key, owner, expectedProcess, reason)
		expectedProcess = nil
		if !ok {
			return 0, false
		}
		if disable {
			m.disableInstance(key, reason)
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
		launched, err := m.launchRuntime(m.lifecycleContext(), ext)
		if err != nil {
			reason = err
			continue
		}

		nextGeneration, name, source, accepted := m.acceptRecoveredRuntime(key, owner, ext, launched)
		if !accepted {
			m.discardRecoveredRuntime(key, launched)
			return 0, false
		}

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
	ext.redactionCleanups = launched.redactionCleanups
	ext.awaitingStability = true
	ext.active = true
	ext.phase = ExtensionPhaseActivate
	ext.lastError = ""
	ext.lastStartedAt = m.now()
	ext.generation++
	return ext.generation, ext.info.Name, ext.info.Source.String(), true
}

func (m *Manager) discardRecoveredRuntime(key InstanceKey, launched launchedRuntime) {
	shutdownErr := shutdownProcessWithTimeout(
		m.lifecycleContext(),
		launched.process,
		m.defaultShutdownTimeout,
	)
	runExtensionRedactionCleanups(launched.redactionCleanups)
	if shutdownErr != nil {
		m.logger.Warn(
			"extension.lifecycle.shutdown_failed",
			managerExtensionKey, key.runtimeID(),
			"recovered", false,
			"error", shutdownErr,
		)
	}
}
