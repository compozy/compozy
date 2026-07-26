package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/compozy/agh/internal/subprocess"
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
	lifecycleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	m.lifecycleCtx = lifecycleCtx
	m.cancel = cancel
	m.started = true
	m.stopping = false
	m.extensions = make(map[string]*managedExtension)
	m.mu.Unlock()

	infos, err := m.registry.List()
	if err != nil {
		cancel()
		m.mu.Lock()
		m.started = false
		m.lifecycleCtx = nil
		m.cancel = nil
		m.extensions = make(map[string]*managedExtension)
		m.mu.Unlock()
		return fmt.Errorf("extension: list registry entries: %w", err)
	}

	var errs []error
	for _, info := range infos {
		ext := &managedExtension{
			info:  info,
			phase: ExtensionPhaseDiscover,
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

	return errors.Join(errs...)
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
		return nil
	}
	m.stopping = true
	cancel := m.cancel
	names := make([]string, 0, len(m.extensions))
	for name := range m.extensions {
		names = append(names, name)
	}
	slices.Sort(names)
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	errCh := make(chan error, len(names))
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
	stopWG.Wait()
	close(errCh)

	m.wg.Wait()

	m.mu.Lock()
	m.started = false
	m.stopping = false
	m.cancel = nil
	m.lifecycleCtx = nil
	m.mu.Unlock()

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (m *Manager) stopManagedExtension(ctx context.Context, item *managedExtension) error {
	proc := item.process
	var itemErr error
	if proc != nil {
		if err := proc.Shutdown(ctx); err != nil {
			select {
			case <-proc.Done():
				if waitErr := proc.Wait(); waitErr != nil {
					itemErr = errors.Join(
						itemErr,
						fmt.Errorf("extension %q stop: %w", item.info.Name, errors.Join(err, waitErr)),
					)
				} else if !errors.Is(err, context.DeadlineExceeded) &&
					!errors.Is(err, subprocess.ErrTransportClosedBeforeResponse) {
					itemErr = errors.Join(itemErr, fmt.Errorf("extension %q stop: %w", item.info.Name, err))
				}
			case <-ctx.Done():
				itemErr = errors.Join(
					itemErr,
					fmt.Errorf("extension %q stop: %w", item.info.Name, errors.Join(err, ctx.Err())),
				)
			}
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
	m.mu.Unlock()

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
