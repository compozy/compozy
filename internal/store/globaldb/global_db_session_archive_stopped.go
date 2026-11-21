package globaldb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListArchiveStoppedMembers reads every unarchived member of every candidate
// thread in one profile scope, with the facts that block archiving it.
func (g *SessionRepo) ListArchiveStoppedMembers(
	ctx context.Context,
	query store.ArchiveStoppedQuery,
) ([]store.ArchiveStoppedMember, error) {
	if err := g.checkReady(ctx, "list archive stopped members"); err != nil {
		return nil, err
	}
	profileID := strings.TrimSpace(query.ProfileID)
	if profileID == "" {
		return nil, errors.New("store: archive stopped requires a profile id")
	}
	members, err := listArchiveStoppedMembers(ctx, g.readQueries, profileID, strings.TrimSpace(query.WorkspaceID), "")
	if err != nil {
		return nil, fmt.Errorf("store: list archive stopped members: %w", err)
	}
	return members, nil
}

// ArchiveStoppedSessionTree archives one thread in the tree archive
// transaction after re-reading its members there: a thread that is no longer a
// candidate, or that a rule now blocks, is refused whole.
func (g *SessionRepo) ArchiveStoppedSessionTree(
	ctx context.Context,
	workspaceID string,
	rootID string,
) (store.SessionTreeArchive, error) {
	return g.archiveSessionTree(ctx, workspaceID, rootID, func(
		ctx context.Context,
		exec globalSQLExecutor,
		root *store.SessionInfo,
	) (map[string]bool, error) {
		members, err := listArchiveStoppedMembers(ctx, sqlcgen.New(exec), root.ProfileID, root.WorkspaceID, root.ID)
		if err != nil {
			return nil, err
		}
		if len(members) == 0 {
			return nil, fmt.Errorf("%w: %s", store.ErrArchiveStoppedNotCandidate, root.ID)
		}
		if block := store.ArchiveStoppedThreadBlock(members); block != store.ArchiveStoppedBlockNone {
			return nil, &store.ArchiveStoppedIneligibleError{RootID: root.ID, Block: block}
		}
		archivable := make(map[string]bool, len(members))
		for i := range members {
			archivable[members[i].SessionID] = true
		}
		return archivable, nil
	})
}

func listArchiveStoppedMembers(
	ctx context.Context,
	q *sqlcgen.Queries,
	profileID string,
	workspaceID string,
	rootID string,
) ([]store.ArchiveStoppedMember, error) {
	rows, err := q.ListArchiveStoppedMembers(ctx, sqlcgen.ListArchiveStoppedMembersParams{
		ProfileID: profileID, WorkspaceID: workspaceID, RootID: rootID,
	})
	if err != nil {
		return nil, err
	}
	members := make([]store.ArchiveStoppedMember, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		members = append(members, store.ArchiveStoppedMember{
			RootID:       row.RootID,
			SessionID:    row.ID,
			WorkspaceID:  row.WorkspaceID,
			Name:         row.Name,
			State:        row.State,
			InstanceMain: row.InstanceMain,
			WaitingOnYou: row.WaitingOnYou,
			QueuedInput:  row.QueuedInput,
			Pinned:       row.Pinned,
			Later:        row.Later,
		})
	}
	return members, nil
}

var _ store.SessionArchiveStoppedStore = (*SessionRepo)(nil)
