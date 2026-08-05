package session

import (
	"context"
	"fmt"
	"strings"
)

type conversationOperationLock struct {
	token chan struct{}
	refs  int
}

func (m *Manager) lockConversationOperation(ctx context.Context, sessionID string) (func(), error) {
	target := strings.TrimSpace(sessionID)
	m.conversationOperationMu.Lock()
	if m.conversationOperationLocks == nil {
		m.conversationOperationLocks = make(map[string]*conversationOperationLock)
	}
	lock := m.conversationOperationLocks[target]
	if lock == nil {
		lock = &conversationOperationLock{token: make(chan struct{}, 1)}
		lock.token <- struct{}{}
		m.conversationOperationLocks[target] = lock
	}
	lock.refs++
	m.conversationOperationMu.Unlock()

	select {
	case <-lock.token:
	case <-ctx.Done():
		m.releaseConversationOperationRef(target, lock)
		return nil, fmt.Errorf("session: wait for conversation operation %q: %w", target, ctx.Err())
	}
	return func() {
		lock.token <- struct{}{}
		m.releaseConversationOperationRef(target, lock)
	}, nil
}

func (m *Manager) releaseConversationOperationRef(target string, lock *conversationOperationLock) {
	m.conversationOperationMu.Lock()
	lock.refs--
	if lock.refs == 0 && m.conversationOperationLocks[target] == lock {
		delete(m.conversationOperationLocks, target)
	}
	m.conversationOperationMu.Unlock()
}
