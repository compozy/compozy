package session

import (
	"context"
	"errors"
)

// WaitForPromptDrains blocks until active prompt tasks finish.
func (m *Manager) WaitForPromptDrains(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("session: wait for prompt drains context is required")
	}

	for {
		m.mu.RLock()
		pending := make([]<-chan struct{}, 0, len(m.promptDrains))
		for done := range m.promptDrains {
			pending = append(pending, done)
		}
		m.mu.RUnlock()

		if len(pending) == 0 {
			return nil
		}

		for _, done := range pending {
			select {
			case <-done:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (m *Manager) trackPromptDrain() func() {
	if m == nil {
		return func() {}
	}

	done := make(chan struct{})
	m.mu.Lock()
	m.promptDrains[done] = struct{}{}
	m.mu.Unlock()

	return func() {
		m.mu.Lock()
		if _, ok := m.promptDrains[done]; ok {
			delete(m.promptDrains, done)
			close(done)
		}
		m.mu.Unlock()
	}
}

func (m *Manager) startTrackedPromptTask(task func()) {
	if task == nil {
		return
	}
	finishDrain := m.trackPromptDrain()
	go func() {
		defer finishDrain()
		task()
	}()
}
