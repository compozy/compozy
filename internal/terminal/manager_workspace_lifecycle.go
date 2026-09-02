package terminal

import (
	"context"
	"fmt"
	"strings"
)

type workspaceProducer struct {
	manager     *Service
	workspaceID string
	registered  bool
	released    bool
}

func (m *Service) beginWorkspaceProducer(workspaceID string) (*workspaceProducer, error) {
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
	return &workspaceProducer{manager: m, workspaceID: workspaceID}, nil
}

func (p *workspaceProducer) MarkRegistered() {
	if p == nil || p.manager == nil {
		return
	}
	p.manager.mu.Lock()
	defer p.manager.mu.Unlock()
	if p.registered || p.released {
		return
	}
	p.registered = true
	p.manager.registeredProducers[p.workspaceID]++
	p.manager.notifyProducerChangeLocked()
}

func (p *workspaceProducer) Release() {
	if p == nil || p.manager == nil {
		return
	}
	p.manager.mu.Lock()
	defer p.manager.mu.Unlock()
	if p.released {
		return
	}
	p.released = true
	p.manager.workspaceProducers[p.workspaceID]--
	if p.manager.workspaceProducers[p.workspaceID] == 0 {
		delete(p.manager.workspaceProducers, p.workspaceID)
	}
	if p.registered {
		p.manager.registeredProducers[p.workspaceID]--
		if p.manager.registeredProducers[p.workspaceID] == 0 {
			delete(p.manager.registeredProducers, p.workspaceID)
		}
	}
	p.manager.notifyProducerChangeLocked()
}

func (m *Service) trackUnpublishedExec(run *execRun) {
	if run == nil || run.item == nil || run.producer == nil {
		return
	}
	m.mu.Lock()
	m.unpublishedExecs[run.key] = run.item
	m.mu.Unlock()
	run.producer.MarkRegistered()
}

func (m *Service) untrackUnpublishedExec(run *execRun) {
	if run == nil || run.item == nil {
		return
	}
	m.mu.Lock()
	if m.unpublishedExecs[run.key] == run.item {
		delete(m.unpublishedExecs, run.key)
	}
	m.mu.Unlock()
}

func (m *Service) notifyProducerChangeLocked() {
	close(m.producerChanged)
	m.producerChanged = make(chan struct{})
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

func (m *Service) waitWorkspaceProducerStarts(ctx context.Context, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	return m.waitProducers(ctx, func() bool {
		return m.workspaceProducers[workspaceID] == m.registeredProducers[workspaceID]
	})
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
