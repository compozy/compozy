package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) changeStoppedWorktreeBinding(ctx context.Context, id, ref string, expected *string) (*Info, error) {
	meta, err := m.reconcileInactiveSession(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateStoppedWorktreeBinding(&meta); err != nil {
		return nil, err
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, id); err != nil {
		return nil, err
	}
	if expected != nil && *expected != meta.WorktreeIDValue() {
		return nil, store.ErrSessionWorktreeConflict
	}
	if err := m.requireWorktreeBindingIdle(ctx, id); err != nil {
		return nil, err
	}
	cwd, worktreeID, release, err := m.prepareWorktreeBindingDestination(ctx, &meta, ref)
	if err != nil {
		return nil, err
	}
	defer release()
	previous := meta.WorktreeIDValue()
	if previous == worktreeID && cwd == meta.CWDValue() {
		return m.sessionInfoFromMeta(ctx, &meta), nil
	}
	// Conversation exclusion blocks resume/delete. The lifecycle and archive locks
	// cover the final metadata read/write without holding them during Git resolution.
	info, err := m.persistStoppedWorktreeBinding(ctx, id, previous, cwd, worktreeID)
	if err != nil {
		return nil, err
	}
	m.notifyWorktreeBindingChanged(ctx, info, previous)
	return info, nil
}

func validateStoppedWorktreeBinding(meta *store.SessionMeta) error {
	if meta.State != string(StateStopped) {
		return ErrPromptInProgress
	}
	if meta.StopVerificationFailed || (meta.Liveness != nil && meta.Liveness.SubprocessPID != 0) {
		return ErrStopVerificationFailed
	}
	if (meta.CreationProfile != nil && meta.CreationProfile.RuntimeMode == RuntimeModeVerdictOnly) ||
		normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser ||
		meta.WorkspaceID == "" ||
		(meta.Lineage != nil && (meta.Lineage.SpawnRole != "" || meta.Lineage.Kind == store.LineageKindSpawn)) {
		return fmt.Errorf("%w: only ordinary project user sessions can change worktrees", ErrValidation)
	}
	return nil
}

func (m *Manager) persistStoppedWorktreeBinding(
	ctx context.Context,
	id, previous, cwd, worktreeID string,
) (*Info, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()
	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	if _, active := m.Get(
		id,
	); active || m.isPending(id) || m.resumeLifecycle.runs[id] != nil ||
		m.hasPendingStopSettlement(id) {
		return nil, ErrPromptInProgress
	}
	meta, err := m.readMetaWithContext(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := validateStoppedWorktreeBinding(&meta); err != nil {
		return nil, err
	}
	if err := m.requireSessionUnarchived(ctx, meta.WorkspaceID, id); err != nil {
		return nil, err
	}
	if meta.WorktreeIDValue() != previous {
		return nil, store.ErrSessionWorktreeConflict
	}
	if err := m.writeWorktreeBinding(ctx, &meta, cwd, worktreeID, nil); err != nil {
		return nil, err
	}
	return m.sessionInfoFromMeta(ctx, &meta), nil
}
