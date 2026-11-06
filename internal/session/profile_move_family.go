package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// lockProfileMoveArchiveFamily excludes resumes until every inherited binding is restored.
func (m *Manager) lockProfileMoveArchiveFamily(
	ctx context.Context,
	meta *store.SessionMeta,
) (context.Context, func(), []store.SessionInfo, error) {
	tree, err := m.sessionTreeStore()
	if err != nil {
		return nil, nil, nil, err
	}
	children, err := tree.ListSessionDescendants(ctx, meta.WorkspaceID, meta.ID)
	if err != nil {
		return nil, nil, nil, err
	}
	family := profileMoveArchiveFamily(meta.ID, children)
	members := make([]store.SessionInfo, 0, len(children))
	unlocks := make([]func(), 0, len(children))
	for i := range children {
		child := &children[i]
		if !family[child.ID] {
			continue
		}
		lockedCtx, unlock, err := m.lockConversationOperation(ctx, child.ID)
		if err != nil {
			releaseConversationOperations(unlocks)
			return nil, nil, nil, err
		}
		ctx = lockedCtx
		unlocks = append(unlocks, unlock)
		members = append(members, *child)
	}
	return ctx, func() { releaseConversationOperations(unlocks) }, members, nil
}

// rebindProfileMoveSubagents preserves isolated checkouts and relocates only inherited bindings.
func (m *Manager) rebindProfileMoveSubagents(
	ctx context.Context,
	children []store.SessionInfo,
	previousID, ref string,
) ([]*Info, error) {
	var rebound []*Info
	if previousID == "" || previousID == ref {
		return rebound, nil
	}
	for i := range children {
		child := &children[i]
		meta, err := m.readMetaWithContext(ctx, child.ID)
		if err != nil {
			return rebound, err
		}
		if meta.WorktreeIDValue() != previousID {
			continue
		}
		if _, live := m.Get(
			child.ID,
		); live || meta.State != string(StateStopped) || m.isPending(child.ID) ||
			m.sessionStartRun(child.ID) != nil {
			return rebound, ErrPromptInProgress
		}
		if meta.StopVerificationFailed || (meta.Liveness != nil && meta.Liveness.SubprocessPID != 0) {
			return rebound, ErrStopVerificationFailed
		}
		cwd, id, release, err := m.prepareWorktreeBindingDestination(ctx, &meta, ref)
		if err != nil {
			return rebound, err
		}
		err = m.writeWorktreeBinding(ctx, &meta, cwd, id, nil)
		release()
		if err != nil {
			return rebound, fmt.Errorf("restore subagent %s worktree binding: %w", child.ID, err)
		}
		rebound = append(rebound, m.sessionInfoFromMeta(ctx, &meta))
	}
	return rebound, nil
}

// unarchiveProfileMoveFamily releases inherited bindings to a checkout the move removed.
func (m *Manager) unarchiveProfileMoveFamily(ctx context.Context, workspaceID, sessionID string) (*Info, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	port, ok := m.worktreeResolver.(ProfileMoveWorktrees)
	if !ok || meta.WorkspaceID != workspaceID || meta.State != string(StateStopped) || meta.WorktreeIDValue() == "" ||
		(meta.Lineage != nil && meta.Lineage.SpawnRole == store.SubagentSpawnRole) {
		return m.setArchived(ctx, workspaceID, sessionID, false)
	}
	checkout, err := port.InspectProfileMoveWorktree(ctx, workspaceID, meta.WorktreeIDValue())
	if errors.Is(err, ErrProfileMoveWorktreeNotFound) {
		return m.setArchived(ctx, workspaceID, sessionID, false)
	}
	if err != nil {
		return nil, err
	}
	if !checkout.Gone {
		return m.setArchived(ctx, workspaceID, sessionID, false)
	}
	tree, err := m.sessionTreeStore()
	if err != nil {
		return nil, err
	}
	children, err := tree.ListSessionDescendants(ctx, workspaceID, sessionID)
	if err != nil {
		return nil, err
	}
	family := profileMoveArchiveFamily(sessionID, children)
	hasFamily := false
	for i := range children {
		child := &children[i]
		if family[child.ID] {
			hasFamily = true
			break
		}
	}
	if !hasFamily {
		return m.setArchived(ctx, workspaceID, sessionID, false)
	}
	if err := m.restoreProfileMoveSource(ctx, &meta, meta.WorktreeIDValue(), "", true); err != nil {
		return nil, err
	}
	return m.Status(ctx, sessionID)
}
