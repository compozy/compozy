package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Start loads every enabled extension through the six-phase pipeline.
func (m *Manager) Start(ctx context.Context) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if m == nil {
		return ErrManagerRequired
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.startLocked(ctx)
}

func (m *Manager) startLocked(ctx context.Context) error {
	if err := m.prepareStart(ctx); err != nil {
		return err
	}

	m.mu.Lock()
	lifecycleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	m.lifecycleCtx = lifecycleCtx
	m.cancel = cancel
	m.started = true
	m.stopping = false
	m.devOperations = 0
	m.devOperationsDone = nil
	m.extensions = make(map[string]*managedExtension)
	m.devExtensions = make(map[InstanceKey]*managedExtension)
	m.profileExtensions = make(map[InstanceKey]*managedExtension)
	m.devCoordinators = make(map[InstanceKey]*sync.Mutex)
	m.profileCoordinators = make(map[InstanceKey]*sync.Mutex)
	m.devLogs = make(map[InstanceKey]*ExtensionLogRing)
	m.mu.Unlock()

	infos, err := m.registry.List()
	if err != nil {
		cancel()
		m.mu.Lock()
		m.started = false
		m.lifecycleCtx = nil
		m.cancel = nil
		m.devOperations = 0
		m.devOperationsDone = nil
		m.extensions = make(map[string]*managedExtension)
		m.devExtensions = make(map[InstanceKey]*managedExtension)
		m.profileExtensions = make(map[InstanceKey]*managedExtension)
		m.devCoordinators = make(map[InstanceKey]*sync.Mutex)
		m.profileCoordinators = make(map[InstanceKey]*sync.Mutex)
		m.devLogs = make(map[InstanceKey]*ExtensionLogRing)
		m.mu.Unlock()
		return fmt.Errorf("extension: list registry entries: %w", err)
	}

	var errs []error
	for _, info := range infos {
		key := GlobalInstanceKey(info.Name)
		ext := &managedExtension{
			key:     key,
			info:    info,
			phase:   ExtensionPhaseDiscover,
			logRing: m.logRingFor(key),
		}
		m.mu.Lock()
		m.extensions[info.Name] = ext
		m.mu.Unlock()

		if !info.Enabled {
			ext.lastError = ""
			continue
		}

		if err := m.startOne(ctx, ext); err != nil {
			errs = append(errs, err)
		}
	}

	links, err := m.registry.ListDevLinks()
	if err != nil {
		errs = append(errs, fmt.Errorf("extension: list development links: %w", err))
	} else {
		for i := range links {
			m.startDevLinkOnBoot(ctx, links[i])
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) prepareStart(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if m.registry == nil {
		return ErrRegistryRequired
	}

	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return errors.New("extension: manager already started")
	}
	m.mu.Unlock()
	if err := m.retryPendingCleanups(ctx); err != nil {
		return fmt.Errorf("extension: drain pending cleanup before start: %w", err)
	}
	return nil
}

// Stop gracefully drains all active extension subprocesses.
func (m *Manager) Stop(ctx context.Context) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if m == nil {
		return ErrManagerRequired
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	return m.stopLocked(ctx)
}

func (m *Manager) stopLocked(ctx context.Context) error {
	m.mu.Lock()
	if !m.started {
		m.mu.Unlock()
		return m.retryPendingCleanups(ctx)
	}
	m.stopping = true
	cancel := m.cancel
	devOperationsDone := m.devOperationsDone
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	// A drain that never settles must not strand extension subprocesses, so teardown continues.
	var drainErr error
	if err := m.waitForDevOperationsDuringShutdown(ctx, devOperationsDone); err != nil {
		drainErr = fmt.Errorf("extension: wait for admitted development operations: %w", err)
	}
	pendingCleanupErr := m.retryPendingCleanups(ctx)

	names, instanceKeys := m.stopTargets()

	errCh := make(chan error, len(names)+len(instanceKeys))
	var stopWG sync.WaitGroup
	for _, name := range names {
		ext, ok := m.lookupManaged(name)
		if !ok {
			continue
		}

		stopWG.Add(1)
		go func(item *managedExtension) {
			defer stopWG.Done()

			if err := m.stopManagedExtension(ctx, item); err != nil {
				errCh <- err
			}
		}(ext)
	}
	for _, key := range instanceKeys {
		ext, ok := m.lookupInstance(key)
		if !ok {
			continue
		}
		stopWG.Add(1)
		go func(item *managedExtension) {
			defer stopWG.Done()
			if err := m.stopManagedExtension(ctx, item); err != nil {
				errCh <- err
			}
		}(ext)
	}
	stopWG.Wait()
	close(errCh)

	m.wg.Wait()

	m.mu.Lock()
	m.started = false
	m.stopping = false
	m.cancel = nil
	m.lifecycleCtx = nil
	m.mu.Unlock()

	errs := []error{drainErr}
	for err := range errCh {
		errs = append(errs, err)
	}
	if pendingCleanupErr != nil {
		errs = append(errs, pendingCleanupErr)
	}
	return errors.Join(errs...)
}

func (m *Manager) stopTargets() ([]string, []InstanceKey) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.extensions))
	for name := range m.extensions {
		names = append(names, name)
	}
	slices.Sort(names)
	instanceKeys := make([]InstanceKey, 0, len(m.devExtensions)+len(m.profileExtensions))
	for key := range m.devExtensions {
		instanceKeys = append(instanceKeys, key)
	}
	for key := range m.profileExtensions {
		instanceKeys = append(instanceKeys, key)
	}
	slices.SortFunc(instanceKeys, compareInstanceKeys)
	return names, instanceKeys
}

func (m *Manager) stopManagedExtension(ctx context.Context, item *managedExtension) error {
	if item == nil {
		return nil
	}
	m.mu.RLock()
	proc := item.process
	m.mu.RUnlock()
	var itemErr error
	if proc != nil {
		if err := shutdownProcessWithTimeout(ctx, proc, m.defaultShutdownTimeout); err != nil {
			itemErr = errors.Join(itemErr, fmt.Errorf("extension %q stop: %w", item.info.Name, err))
		}
	}

	if err := m.unregisterResources(ctx, item); err != nil {
		itemErr = errors.Join(itemErr, err)
	}

	m.mu.Lock()
	item.process = nil
	item.active = false
	item.awaitingStability = false
	item.phase = ExtensionPhaseStop
	cleanups := item.redactionCleanups
	item.redactionCleanups = nil
	m.mu.Unlock()
	if processTerminal(proc) {
		runExtensionRedactionCleanups(cleanups)
	} else if proc != nil {
		m.retainPendingCleanup(pendingExtensionCleanup{
			key:               item.instanceKey(),
			process:           proc,
			redactionCleanups: cleanups,
		})
	}

	m.logger.Info("extension.lifecycle.shutdown", managerExtensionKey, item.info.Name)
	return itemErr
}

// Reload restarts the manager from the current registry state as one lifecycle operation.
func (m *Manager) Reload(ctx context.Context) error {
	if ctx == nil {
		return ErrContextRequired
	}
	if m == nil {
		return ErrManagerRequired
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	stopErr := m.stopLocked(ctx)
	startErr := m.startLocked(ctx)
	return errors.Join(stopErr, startErr)
}
