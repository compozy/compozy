package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/resources"
)

type startupDeclarations = loadedExtensionResources

type preparedExtensionStartup struct {
	transaction     *extensionStartupTransaction
	declarations    startupDeclarations
	grant           EffectiveGrant
	resourceSession *hostAPIResourceSession
	runtime         *launchedRuntime
}

type startupCleanup struct {
	name string
	run  func(context.Context) error
}

type extensionStartupTransaction struct {
	manager   *Manager
	extension *managedExtension
	cleanups  []startupCleanup
	committed bool
}

type startupRuntimeOwnership struct {
	manager           *Manager
	key               InstanceKey
	process           processHandle
	redactionCleanups []func()
	retained          bool
}

func newExtensionStartupTransaction(m *Manager, ext *managedExtension) *extensionStartupTransaction {
	return &extensionStartupTransaction{manager: m, extension: ext}
}

func (tx *extensionStartupTransaction) add(name string, run func(context.Context) error) {
	if tx == nil || tx.committed || run == nil {
		return
	}
	tx.cleanups = append(tx.cleanups, startupCleanup{name: name, run: run})
}

func (tx *extensionStartupTransaction) commit() {
	if tx == nil {
		return
	}
	tx.committed = true
	tx.cleanups = nil
}

func (tx *extensionStartupTransaction) rollback(ctx context.Context, primary error) error {
	if tx == nil || tx.committed {
		return primary
	}
	if tx.manager == nil {
		return errors.Join(primary, errors.New("extension: startup transaction manager is required"))
	}

	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tx.manager.defaultShutdownTimeout)
	defer cancel()

	result := primary
	for _, cleanup := range slices.Backward(tx.cleanups) {
		if cleanup.run == nil {
			continue
		}
		if err := cleanup.run(cleanupCtx); err != nil {
			result = errors.Join(result, fmt.Errorf("extension: rollback %s: %w", cleanup.name, err))
		}
	}
	tx.cleanups = nil
	return result
}

func (m *Manager) newStartupRuntimeOwnership(
	key InstanceKey,
	proc processHandle,
	redactionCleanups []func(),
) *startupRuntimeOwnership {
	return &startupRuntimeOwnership{
		manager:           m,
		key:               key.Normalize(),
		process:           proc,
		redactionCleanups: append([]func(){}, redactionCleanups...),
	}
}

func (o *startupRuntimeOwnership) stop(ctx context.Context) error {
	if o == nil || o.process == nil {
		return nil
	}

	err := shutdownProcessWithTimeout(ctx, o.process, o.manager.defaultShutdownTimeout)
	if processTerminal(o.process) {
		return err
	}
	o.manager.retainPendingCleanup(pendingExtensionCleanup{
		key:               o.key,
		process:           o.process,
		redactionCleanups: append([]func(){}, o.redactionCleanups...),
	})
	o.retained = true
	return err
}

func (o *startupRuntimeOwnership) releaseRedaction() error {
	if o == nil || o.retained {
		return nil
	}
	runExtensionRedactionCleanups(o.redactionCleanups)
	o.redactionCleanups = nil
	return nil
}

func (m *Manager) stageExtensionDeclarations(
	ctx context.Context,
	ext *managedExtension,
) (startupDeclarations, error) {
	return m.loadDeclarativeResources(ctx, ext)
}

func (m *Manager) prepareExtensionStartup(
	ctx context.Context,
	ext *managedExtension,
) (prepared *preparedExtensionStartup, err error) {
	transaction := newExtensionStartupTransaction(m, ext)
	defer func() {
		if err != nil {
			err = transaction.rollback(ctx, err)
		}
	}()

	declarations, err := m.stageExtensionDeclarations(ctx, ext)
	if err != nil {
		m.setFailure(ext, ExtensionPhaseRegister, err)
		return nil, phaseError(ext.info.Name, ExtensionPhaseRegister, err)
	}
	prepared = &preparedExtensionStartup{
		transaction:  transaction,
		declarations: declarations,
		grant:        ext.pendingGrant,
	}
	if !requiresSubprocess(ext.manifest) {
		return prepared, nil
	}

	runtime, resourceSession, err := m.launchStartupRuntime(ctx, ext, prepared.grant, transaction)
	if err != nil {
		if errors.Is(err, ErrBridgeRuntimeDeferred) {
			return prepared, nil
		}
		m.setFailure(ext, ExtensionPhaseInitialize, err)
		return nil, phaseError(ext.info.Name, ExtensionPhaseInitialize, err)
	}
	prepared.runtime = &runtime
	prepared.resourceSession = resourceSession
	return prepared, nil
}

func (m *Manager) commitPreparedExtension(
	ctx context.Context,
	ext *managedExtension,
	prepared *preparedExtensionStartup,
) error {
	return m.commitPreparedExtensionWithPublish(ctx, ext, prepared, nil)
}

func (m *Manager) commitPreparedExtensionWithPublish(
	ctx context.Context,
	ext *managedExtension,
	prepared *preparedExtensionStartup,
	publish func(),
) (err error) {
	if ext == nil || prepared == nil || prepared.transaction == nil {
		return errors.New("extension: prepared startup is required")
	}
	defer func() {
		if err != nil {
			err = prepared.transaction.rollback(ctx, err)
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := m.activatePreparedSourceSession(ctx, prepared.resourceSession, prepared.transaction); err != nil {
		return err
	}

	m.mu.Lock()
	if err := ctx.Err(); err != nil {
		m.mu.Unlock()
		return err
	}
	if !m.started || m.stopping {
		m.mu.Unlock()
		return errors.New("extension: manager is not accepting startup commits")
	}
	m.applyPreparedExtensionLocked(ext, prepared)
	if publish != nil {
		publish()
	}
	generation := ext.generation
	name := ext.info.Name
	source := ext.info.Source.String()
	active := ext.active
	skillCount := len(ext.skills)
	agentCount := len(ext.agents)
	hookCount := len(ext.hooks)
	supervise := prepared.runtime != nil && !ext.deferSupervision
	if supervise {
		m.wg.Add(1)
	}
	m.mu.Unlock()

	prepared.transaction.commit()
	m.logger.Info(
		"extension.lifecycle.loaded",
		managerExtensionKey, name,
		"source", source,
		"active", active,
		"skill_count", skillCount,
		"agent_count", agentCount,
		"hook_count", hookCount,
	)
	if supervise {
		go m.superviseInstance(ext.instanceKey(), generation)
	}
	return nil
}

func (m *Manager) applyPreparedExtensionLocked(
	ext *managedExtension,
	prepared *preparedExtensionStartup,
) {
	prepared.declarations.apply(ext)
	ext.grantedPermissions = slices.Clone(prepared.grant.Permissions)
	ext.grantedSecurity = slices.Clone(prepared.grant.Security)
	ext.grantedResourceKinds = slices.Clone(prepared.grant.ResourceKinds)
	ext.grantedResourceScopes = slices.Clone(prepared.grant.ResourceScopes)
	if prepared.runtime != nil {
		ext.process = prepared.runtime.process
		ext.initialize = &prepared.runtime.response
		ext.runtime = prepared.runtime.runtime
		ext.healthInterval = prepared.runtime.healthInterval
		ext.sessionNonce = prepared.runtime.sessionNonce
		ext.capabilityGrantID = prepared.runtime.capabilityGrantID
		ext.redactionCleanups = prepared.runtime.redactionCleanups
		ext.awaitingStability = true
		ext.lastStartedAt = m.now()
		ext.generation++
	}
	ext.phase = ExtensionPhaseActivate
	ext.registered = true
	ext.active = ext.process != nil || !requiresSubprocess(ext.manifest)
	ext.restartBackoff = 0
	ext.lastError = ""
	ext.startup = nil
}

func (m *Manager) activatePreparedSourceSession(
	ctx context.Context,
	resourceSession *hostAPIResourceSession,
	transaction *extensionStartupTransaction,
) error {
	if resourceSession == nil || m.sourceSessions == nil {
		return nil
	}
	if transaction == nil || transaction.extension == nil {
		return errors.New("extension: startup transaction is required for source session activation")
	}
	if err := m.sourceSessions.ActivateSourceSession(
		ctx,
		extensionManagerResourceActor(),
		resourceSession.Actor.Source,
		resourceSession.Actor.SessionNonce,
	); err != nil {
		return fmt.Errorf(
			"extension: activate source session for %q: %w",
			resourceSession.Actor.Source.ID,
			err,
		)
	}
	key := transaction.extension.instanceKey()
	transaction.add("source session", func(cleanupCtx context.Context) error {
		return m.resetExtensionResourceSourceOrRetain(
			cleanupCtx,
			key,
			resourceSession.Actor.Source,
			resourceSession.Actor.SessionNonce,
		)
	})
	return nil
}

func (m *Manager) resetExtensionResourceSourceIfActiveSession(
	ctx context.Context,
	source resources.ResourceSource,
	sessionNonce string,
) error {
	if m == nil || m.sourceSessions == nil {
		return nil
	}
	if ctx == nil {
		return ErrContextRequired
	}
	if _, err := m.sourceSessions.ResetSourceIfActiveSession(
		ctx,
		extensionManagerResourceActor(),
		source,
		sessionNonce,
	); err != nil {
		return fmt.Errorf("extension: reset active source session for %q: %w", source.ID, err)
	}
	return nil
}
