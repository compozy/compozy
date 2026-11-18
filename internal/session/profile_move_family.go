package session

import (
	"context"
	"fmt"
	"slices"

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
	ids := []string{meta.ID}
	for i := range children {
		if family[children[i].ID] {
			members = append(members, children[i])
			ids = append(ids, children[i].ID)
		}
	}
	slices.Sort(ids)
	unlocks := make([]func(), 0, len(ids))
	for _, id := range ids {
		lockedCtx, unlock, err := m.lockConversationOperation(ctx, id)
		if err != nil {
			releaseConversationOperations(unlocks)
			return nil, nil, nil, err
		}
		ctx = lockedCtx
		unlocks = append(unlocks, unlock)
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
		if meta.WorktreeTransition != nil && meta.WorktreeTransition.Pending {
			if err := m.persistSessionCatalogFromMeta(ctx, &meta); err != nil {
				return rebound, err
			}
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

// unarchiveProfileMoveFamily cleans up only a checkout released by a recorded completed move.
func (m *Manager) unarchiveProfileMoveFamily(ctx context.Context, workspaceID, sessionID string) (*Info, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	move := meta.CompletedProfileMove
	binding := meta.WorktreeIDValue()
	if transition := meta.WorktreeTransition; transition != nil && transition.Pending &&
		transition.Current.WorktreeID == "" {
		binding = transition.Previous.WorktreeID
	}
	if meta.WorkspaceID != workspaceID || move == nil || move.SessionID != meta.ID || move.SourceWorktreeID == "" ||
		binding != move.SourceWorktreeID {
		return m.setArchived(ctx, workspaceID, sessionID, false)
	}
	if err := m.restoreProfileMoveSource(ctx, &meta, move.SourceWorktreeID, "", true); err != nil {
		return nil, err
	}
	return m.Status(ctx, sessionID)
}
