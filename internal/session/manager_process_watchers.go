package session

import (
	"context"
	"errors"
	"fmt"
)

type processWatchRun struct {
	done chan struct{}
}

func (m *Manager) ensureProcessWatchContextLocked() {
	if m == nil || m.processWatchLifecycle.ctx != nil {
		return
	}
	m.processWatchLifecycle.ctx, m.processWatchLifecycle.cancel = context.WithCancel(m.lifecycleCtx)
}

func (m *Manager) beginProcessWatch() (context.Context, *processWatchRun, bool) {
	if m == nil {
		return nil, nil, false
	}
	m.processWatchLifecycle.mu.Lock()
	defer m.processWatchLifecycle.mu.Unlock()
	if m.processWatchLifecycle.closing {
		return nil, nil, false
	}
	m.ensureProcessWatchContextLocked()
	if m.processWatchLifecycle.runs == nil {
		m.processWatchLifecycle.runs = make(map[*processWatchRun]struct{})
	}
	run := &processWatchRun{done: make(chan struct{})}
	m.processWatchLifecycle.runs[run] = struct{}{}
	return m.processWatchLifecycle.ctx, run, true
}

func (m *Manager) finishProcessWatch(run *processWatchRun) {
	if m == nil || run == nil {
		return
	}
	m.processWatchLifecycle.mu.Lock()
	if _, ok := m.processWatchLifecycle.runs[run]; ok {
		delete(m.processWatchLifecycle.runs, run)
		close(run.done)
	}
	m.processWatchLifecycle.mu.Unlock()
}

func (m *Manager) shutdownProcessWatchers(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: process watcher shutdown context is required")
	}
	m.processWatchLifecycle.mu.Lock()
	m.processWatchLifecycle.closing = true
	cancel := m.processWatchLifecycle.cancel
	done := make([]<-chan struct{}, 0, len(m.processWatchLifecycle.runs))
	for run := range m.processWatchLifecycle.runs {
		done = append(done, run.done)
	}
	m.processWatchLifecycle.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for _, runDone := range done {
		select {
		case <-runDone:
		case <-ctx.Done():
			return fmt.Errorf("session: wait for process watchers: %w", ctx.Err())
		}
	}
	return nil
}
