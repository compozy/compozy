package globaldb

import (
	"context"
	"database/sql"
	"errors"
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
) ([]store.SessionInfo, error) {
	if err := g.checkReady(ctx, "list session descendants"); err != nil {
		return nil, err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, errors.New("store: session descendants session id is required")
	}
	var descendants []store.SessionInfo
	err := g.withImmediateTransaction(ctx, "list session descendants", func(exec globalSQLExecutor) error {
		var e error
		descendants, e = sessionTreeDescendants(ctx, exec, workspaceID, sessionID)
		return e
	})
	return descendants, err
}

// ArchiveSessionTree archives one session and every hierarchy descendant in one
// transaction, so a live member leaves the whole tree unarchived.
func (g *SessionRepo) ArchiveSessionTree(
	ctx context.Context,
	workspaceID string,
	sessionID string,
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
		if root.Lineage != nil && root.Lineage.SpawnRole == store.SubagentSpawnRole {
			return store.ErrSubagentArchiveFollowsParent
		}
		descendants, e := sessionTreeDescendants(ctx, exec, workspaceID, sessionID)
		if e != nil {
			return e
		}
		ids := make([]string, 0, len(descendants)+1)
		for _, member := range append([]store.SessionInfo{root}, descendants...) {
			if member.State != globalDBSessionStateStopped {
				return &store.SessionArchiveLiveMemberError{SessionID: member.ID}
			}
			ids = append(ids, member.ID)
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
		result.Descendants, e = sessionTreeDescendants(ctx, exec, workspaceID, sessionID)
		return e
	})
	if err != nil {
		return store.SessionTreeArchive{}, err
	}
	return result, nil
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
