package terminal

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

func (m *Service) beginWorkspaceProducer(workspaceID string) (func(), error) {
	workspaceID = strings.TrimSpace(workspaceID)
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return nil, fmt.Errorf("%s: %w", errorMessageShuttingDown, ErrShuttingDown)
	}
	if _, sealed := m.sealedWorkspaces[workspaceID]; sealed {
		m.mu.Unlock()
		return nil, fmt.Errorf("terminal workspace is sealed: %w", ErrServiceUnavailable)
	}
	m.workspaceProducers[workspaceID]++
	m.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			m.mu.Lock()
			m.workspaceProducers[workspaceID]--
			if m.workspaceProducers[workspaceID] == 0 {
				delete(m.workspaceProducers, workspaceID)
			}
			close(m.producerChanged)
			m.producerChanged = make(chan struct{})
			m.mu.Unlock()
		})
	}, nil
}

func (m *Service) sealWorkspace(ctx context.Context, workspaceID string) error {
	if err := requestContextError(ctx, "seal workspace"); err != nil {
		return err
	}
	m.mu.Lock()
	m.sealedWorkspaces[strings.TrimSpace(workspaceID)] = struct{}{}
	m.mu.Unlock()
	return nil
}

func (m *Service) unsealWorkspace(workspaceID string) {
	m.mu.Lock()
	delete(m.sealedWorkspaces, strings.TrimSpace(workspaceID))
	m.mu.Unlock()
}

func (m *Service) waitWorkspaceProducers(ctx context.Context, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	return m.waitProducers(ctx, func() bool { return m.workspaceProducers[workspaceID] == 0 })
}

func (m *Service) waitAllProducers(ctx context.Context) error {
	return m.waitProducers(ctx, func() bool { return len(m.workspaceProducers) == 0 })
}

func (m *Service) waitProducers(ctx context.Context, ready func() bool) error {
	for {
		m.mu.Lock()
		if ready() {
			m.mu.Unlock()
			return nil
		}
		changed := m.producerChanged
		m.mu.Unlock()
		select {
		case <-ctx.Done():
			return fmt.Errorf("terminal: wait for producer quiescence: %w", context.Cause(ctx))
		case <-changed:
		}
	}
}
