package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var errConversationOperationReentrant = errors.New("session: conversation operation cannot acquire its own lock")

type conversationOperationOwners map[string]struct{}

type conversationOperationLock struct {
	token chan struct{}
	refs  int
}

func (m *Manager) lockConversationOperation(
	ctx context.Context,
	sessionID string,
) (context.Context, func(), error) {
	target := strings.TrimSpace(sessionID)
	if owners, ok := ctx.Value(conversationOperationOwnersContextKey{}).(conversationOperationOwners); ok {
		if _, owned := owners[target]; owned {
			return nil, nil, fmt.Errorf("%w: %s", errConversationOperationReentrant, target)
		}
	}
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
		return nil, nil, fmt.Errorf("session: wait for conversation operation %q: %w", target, ctx.Err())
	}
	owners := addConversationOperationOwner(ctx, target)
	operationCtx := context.WithValue(ctx, conversationOperationOwnersContextKey{}, owners)
	return operationCtx, func() {
		lock.token <- struct{}{}
		m.releaseConversationOperationRef(target, lock)
	}, nil
}

type conversationOperationOwnersContextKey struct{}

func ownsConversationOperation(ctx context.Context, target string) bool {
	if ctx == nil {
		return false
	}
	owners, ok := ctx.Value(conversationOperationOwnersContextKey{}).(conversationOperationOwners)
	if !ok {
		return false
	}
	_, owned := owners[strings.TrimSpace(target)]
	return owned
}

func addConversationOperationOwner(ctx context.Context, target string) conversationOperationOwners {
	owners := make(conversationOperationOwners)
	if current, ok := ctx.Value(conversationOperationOwnersContextKey{}).(conversationOperationOwners); ok {
		for sessionID := range current {
			owners[sessionID] = struct{}{}
		}
	}
	owners[target] = struct{}{}
	return owners
}

func (m *Manager) releaseConversationOperationRef(target string, lock *conversationOperationLock) {
	m.conversationOperationMu.Lock()
	lock.refs--
	if lock.refs == 0 && m.conversationOperationLocks[target] == lock {
		delete(m.conversationOperationLocks, target)
	}
	m.conversationOperationMu.Unlock()
}
