package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

func (m *Manager) reserveStart(ctx context.Context, id string, workspaceID string) error {
	release, err := m.reserveStartLifecycle(ctx, id, workspaceID)
	if release != nil {
		release()
	}
	return err
}

func (m *Manager) reserveStartLifecycle(
	ctx context.Context,
	id string,
	workspaceID string,
) (func(), error) {
	if ctx == nil {
		return nil, errors.New("session: reserve start context is required")
	}
	targetWorkspace := strings.TrimSpace(workspaceID)
	if targetWorkspace == "" {
		return nil, errors.New("session: reserve start workspace id is required")
	}

	m.lifecycleMu.Lock()
	release := sync.OnceFunc(m.lifecycleMu.Unlock)
	fail := func(err error) (func(), error) {
		release()
		return nil, err
	}

	resolver, err := m.requireWorkspaceResolver()
	if err != nil {
		return fail(err)
	}
	resolved, err := resolver.Resolve(ctx, targetWorkspace)
	if err != nil {
		return fail(fmt.Errorf("session: revalidate workspace %q before start: %w", targetWorkspace, err))
	}
	if strings.TrimSpace(resolved.ID) != targetWorkspace {
		return fail(fmt.Errorf(
			"session: revalidated workspace id %q does not match %q",
			strings.TrimSpace(resolved.ID),
			targetWorkspace,
		))
	}

	m.mu.Lock()
	if _, ok := m.sessions[id]; ok {
		m.mu.Unlock()
		return fail(fmt.Errorf("session: session %q is already active", id))
	}
	if _, ok := m.pending[id]; ok {
		m.mu.Unlock()
		return fail(fmt.Errorf("session: session %q is already pending", id))
	}

	m.pending[id] = sessionReservation{workspaceID: targetWorkspace}
	m.mu.Unlock()
	return release, nil
}
