package extensionpkg

import (
	"context"
	"errors"
	"sync"
)

type devOperation struct {
	manager             *Manager
	ctx                 context.Context
	cancel              context.CancelFunc
	stopLifecycleCancel func() bool
	once                sync.Once
}

func (m *Manager) beginDevOperation(ctx context.Context, key InstanceKey) (*devOperation, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrManagerRequired
	}
	key = key.Normalize()
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if key.IsGlobal() {
		return nil, errors.New("extension: development operations require a workspace instance")
	}
	if m.registry == nil {
		return nil, ErrRegistryRequired
	}

	operationCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	if !m.started || m.stopping || m.lifecycleCtx == nil {
		m.mu.Unlock()
		cancel()
		return nil, errors.New("extension: manager is not accepting development operations")
	}
	if m.devOperations == 0 {
		m.devOperationsDone = make(chan struct{})
	}
	m.devOperations++
	lifecycleCtx := m.lifecycleCtx
	m.mu.Unlock()

	return &devOperation{
		manager:             m,
		ctx:                 operationCtx,
		cancel:              cancel,
		stopLifecycleCancel: context.AfterFunc(lifecycleCtx, cancel),
	}, nil
}

func (o *devOperation) close() {
	if o == nil || o.manager == nil {
		return
	}
	o.once.Do(func() {
		if o.stopLifecycleCancel != nil {
			o.stopLifecycleCancel()
		}
		if o.cancel != nil {
			o.cancel()
		}
		o.manager.completeDevOperation()
	})
}

func (m *Manager) completeDevOperation() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.devOperations == 0 {
		return
	}
	m.devOperations--
	if m.devOperations != 0 || m.devOperationsDone == nil {
		return
	}
	close(m.devOperationsDone)
	m.devOperationsDone = nil
}

func (m *Manager) waitForDevOperations(ctx context.Context, done <-chan struct{}) error {
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) waitForDevOperationsDuringShutdown(ctx context.Context, done <-chan struct{}) error {
	if m == nil {
		return ErrManagerRequired
	}
	if ctx == nil {
		return ErrContextRequired
	}
	drainCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.defaultShutdownTimeout)
	defer cancel()
	return m.waitForDevOperations(drainCtx, done)
}

func (o *devOperation) ensureActive() error {
	if o == nil || o.manager == nil {
		return ErrManagerRequired
	}
	if err := o.ctx.Err(); err != nil {
		return err
	}
	o.manager.mu.RLock()
	accepting := o.manager.started && !o.manager.stopping
	o.manager.mu.RUnlock()
	if !accepting {
		select {
		case <-o.manager.lifecycleDone():
			return context.Canceled
		default:
		}
		return errors.New("extension: manager is not accepting development operations")
	}
	return nil
}
