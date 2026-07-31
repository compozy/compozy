package session

import "context"

func (m *Manager) ensureProcessWatchContextLocked() {
	if m == nil || m.processWatchCtx != nil {
		return
	}
	base := m.lifecycleCtx
	if base == nil {
		base = context.Background()
	}
	m.processWatchCtx, m.processWatchCancel = context.WithCancel(base)
}

func (m *Manager) beginProcessWatch() (context.Context, bool) {
	if m == nil {
		return nil, false
	}
	m.processWatchMu.Lock()
	defer m.processWatchMu.Unlock()
	if m.processWatchClosing {
		return nil, false
	}
	m.ensureProcessWatchContextLocked()
	m.processWatchWG.Add(1)
	return m.processWatchCtx, true
}

func (m *Manager) shutdownProcessWatchers() {
	if m == nil {
		return
	}
	m.processWatchMu.Lock()
	m.processWatchClosing = true
	cancel := m.processWatchCancel
	m.processWatchMu.Unlock()
	if cancel != nil {
		cancel()
	}
	m.processWatchWG.Wait()
}
