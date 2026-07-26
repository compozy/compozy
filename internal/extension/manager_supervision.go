package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"path/filepath"

	"strings"

	"time"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/subprocess"
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
		err := errors.New("subprocess command is required when runtime capabilities or actions are declared")
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}

	grant, err := m.capChecker.RegisterForSession(
		ext.info.Name,
		ext.info.Source,
		ext.manifest,
		resources.ResourceScopeKindGlobal,
	)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseValidate, err)
		return phaseError(ext.info.Name, ExtensionPhaseValidate, err)
	}
	ext.grantedActions = grant.Actions
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
	ext.agents = agents
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

	m.wg.Add(1)
	go m.superviseExtension(ext.info.Name, generation)
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

func (m *Manager) superviseExtension(name string, generation int64) {
	defer m.wg.Done()

	for {
		proc, interval, ok := m.currentProcess(name, generation)
		if !ok {
			return
		}

		shouldRecover, reason := m.monitorProcess(name, generation, proc, interval)
		if !shouldRecover {
			return
		}

		nextGeneration, recovered := m.recoverExtension(name, reason)
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
	ticker := time.NewTicker(m.healthPollInterval(healthInterval))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if m.shouldStopSupervision(name, generation, proc) {
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
					m.shutdownDeadlineForProcess(name, generation),
				); shutdownErr != nil {
					reason = errors.Join(reason, shutdownErr)
				}
				return true, reason
			}
			if !health.LastCheckedAt.IsZero() {
				m.markStable(name, generation)
			}
		case <-proc.Done():
			if m.shouldStopSupervision(name, generation, proc) {
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

func (m *Manager) recoverExtension(name string, reason error) (int64, bool) {
	for {
		backoff, disable, ok := m.recordFailure(name, reason)
		if !ok {
			return 0, false
		}
		if disable {
			m.disableExtension(name, reason)
			return 0, false
		}
		if !m.waitBackoff(backoff) {
			return 0, false
		}

		ext, ok := m.lookupManaged(name)
		if !ok {
			return 0, false
		}
		launched, err := m.launchRuntime(m.lifecycleContext(), ext)
		if err != nil {
			reason = err
			continue
		}

		m.mu.Lock()
		if m.stopping || ext.generation == 0 && !ext.info.Enabled {
			m.mu.Unlock()
			shutdownErr := shutdownProcessWithTimeout(
				m.lifecycleContext(),
				launched.process,
				m.defaultShutdownTimeout,
			)
			runExtensionRedactionCleanups(launched.redactionCleanups)
			if shutdownErr != nil {
				m.logger.Warn(
					"extension.lifecycle.shutdown_failed",
					managerExtensionKey, name,
					"recovered", false,
					"error", shutdownErr,
				)
			}
			return 0, false
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
		nextGeneration := ext.generation
		name := ext.info.Name
		source := ext.info.Source.String()
		m.mu.Unlock()

		m.logger.Info("extension.lifecycle.loaded", managerExtensionKey, name, "source", source, "recovered", true)

		return nextGeneration, true
	}
}
