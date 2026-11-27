package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

// ListSessionDescendants returns the hierarchy descendants of one session,
// shallowest first. The walk stays inside the root's workspace and profile.
func (g *SessionRepo) ListSessionDescendants(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (descendants []store.SessionInfo, err error) {
	if err := g.checkReady(ctx, "list session descendants"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("store: session descendants session id is required")
	}
	// A read snapshot keeps the walk consistent without holding the write lock.
	tx, err := g.readDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("store: begin session descendants read: %w", err)
	}
	defer func() { err = errors.Join(err, rollbackTx(tx, "list session descendants")) }()
	return sessionTreeDescendants(ctx, tx, workspaceID, sessionID)
}

// ArchiveSessionTree archives one session and every hierarchy descendant in one
// transaction, so a live member leaves the whole tree unarchived.
func (g *SessionRepo) ArchiveSessionTree(
	ctx context.Context,
	workspaceID string,
	sessionID string,
) (store.SessionTreeArchive, error) {
	return g.archiveSessionTree(ctx, workspaceID, sessionID, nil)
}

// archiveSessionTree runs the tree archive transaction; a non-nil recheck runs
// first inside it and may refuse the archive or narrow the archived members.
func (g *SessionRepo) archiveSessionTree(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	recheck func(context.Context, globalSQLExecutor, *store.SessionInfo) (map[string]bool, error),
) (store.SessionTreeArchive, error) {
	if err := g.checkReady(ctx, "archive session tree"); err != nil {
		return store.SessionTreeArchive{}, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return store.SessionTreeArchive{}, errors.New("store: session archive session id is required")
	}
	var result store.SessionTreeArchive
	err := g.withImmediateTransaction(ctx, "archive session tree", func(exec globalSQLExecutor) error {
		root, e := sessionInWorkspace(ctx, exec, workspaceID, sessionID)
		if errors.Is(e, sql.ErrNoRows) {
			return store.ErrSessionNotFound
		}
		if e != nil {
			return e
		}
		var archivable map[string]bool
		if recheck != nil {
			if archivable, e = recheck(ctx, exec, &root); e != nil {
				return e
			}
		}
		if root.Lineage != nil && root.Lineage.SpawnRole == store.SubagentSpawnRole {
			return store.ErrSubagentArchiveFollowsParent
		}
		descendants, e := sessionTreeDescendants(ctx, exec, workspaceID, sessionID)
		if e != nil {
			return e
		}
		ids, e := archiveTreeMemberIDs(ctx, sqlcgen.New(exec), &root, descendants)
		if e != nil {
			return e
		}
		encoded, e := subagentIDsJSON(ids)
		if e != nil {
			return e
		}
		now := store.FormatTimestamp(g.now())
		if e = sqlcgen.New(exec).SetSubagentFamilyArchived(ctx, sqlcgen.SetSubagentFamilyArchivedParams{
			WorkspaceID: workspaceID, Ids: encoded, ArchivedAt: sql.NullString{String: now, Valid: true}, Now: now,
		}); e != nil {
			return mapSessionArchivedConstraint(sessionID, e)
		}
		if result.Root, e = sessionInWorkspace(ctx, exec, workspaceID, sessionID); e != nil {
			return e
		}
		if result.Descendants, e = sessionTreeDescendants(ctx, exec, workspaceID, sessionID); e != nil {
			return e
		}
		if archivable != nil {
			result.Descendants = slices.DeleteFunc(result.Descendants, func(member store.SessionInfo) bool {
				return !archivable[member.ID]
			})
		}
		return nil
	})
	if err != nil {
		return store.SessionTreeArchive{}, err
	}
	return result, nil
}

// archiveTreeMemberIDs refuses a tree holding an instance main session or a
// live member and returns its member IDs, root first.
func archiveTreeMemberIDs(
	ctx context.Context,
	q *sqlcgen.Queries,
	root *store.SessionInfo,
	descendants []store.SessionInfo,
) ([]string, error) {
	if err := guardInstanceMainArchive(ctx, q, root.ID); err != nil {
		return nil, err
	}
	for i := range descendants {
		if err := guardInstanceMainArchive(ctx, q, descendants[i].ID); err != nil {
			return nil, err
		}
	}
	if root.State != globalDBSessionStateStopped {
		return nil, &store.SessionArchiveLiveMemberError{SessionID: root.ID}
	}
	ids := make([]string, 1, len(descendants)+1)
	ids[0] = root.ID
	for i := range descendants {
		if descendants[i].State != globalDBSessionStateStopped {
			return nil, &store.SessionArchiveLiveMemberError{SessionID: descendants[i].ID}
		}
		ids = append(ids, descendants[i].ID)
	}
	return ids, nil
}

func sessionInWorkspace(
	ctx context.Context,
	exec globalSQLExecutor,
	workspaceID string,
	sessionID string,
) (store.SessionInfo, error) {
	return scanSessionInfo(
		exec.QueryRowContext(ctx, sessionInfoSelectQuery+" WHERE workspace_id = ? AND id = ?", workspaceID, sessionID),
	)
}

func sessionTreeDescendants(
	ctx context.Context,
	exec globalSQLExecutor,
	workspaceID string,
	sessionID string,
) ([]store.SessionInfo, error) {
	ids, err := sqlcgen.New(exec).ListSessionTreeDescendantIDs(ctx, sqlcgen.ListSessionTreeDescendantIDsParams{
		WorkspaceID: workspaceID, SessionID: sessionID,
	})
	if err != nil {
		return nil, err
	}
	descendants := make([]store.SessionInfo, 0, len(ids))
	for _, id := range ids {
		member, err := sessionInWorkspace(ctx, exec, workspaceID, id)
		if err != nil {
			return nil, err
		}
		descendants = append(descendants, member)
	}
	return descendants, nil
}

var _ store.SessionTreeStore = (*SessionRepo)(nil)
