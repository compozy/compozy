package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.SessionLayoutStore = (*SessionRepo)(nil)

// ListSessionFolders returns one profile's folders in a workspace in manual order.
func (g *SessionRepo) ListSessionFolders(
	ctx context.Context,
	scope store.SessionLayoutScope,
) ([]store.SessionFolder, error) {
	if err := g.checkReady(ctx, "list session folders"); err != nil {
		return nil, err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return nil, err
	}
	return listSessionFolders(ctx, g.queries, scope)
}

// CreateSessionFolder inserts a folder at the top of the scope's folder order.
func (g *SessionRepo) CreateSessionFolder(
	ctx context.Context,
	scope store.SessionLayoutScope,
	id string,
	name string,
) (store.SessionFolder, error) {
	if err := g.checkReady(ctx, "create session folder"); err != nil {
		return store.SessionFolder{}, err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return store.SessionFolder{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return store.SessionFolder{}, fmt.Errorf("%w: folder id is required", store.ErrSessionLayoutInvalid)
	}
	if name, err = store.NormalizeSessionFolderName(name); err != nil {
		return store.SessionFolder{}, err
	}
	var created store.SessionFolder
	err = g.withImmediateTransaction(ctx, "create session folder", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		count, e := q.CountSessionFolders(ctx, sqlcgen.CountSessionFoldersParams{
			ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
		})
		if e != nil {
			return e
		}
		if count >= store.MaxSessionFolders {
			return store.ErrSessionFolderLimit
		}
		if e := q.ShiftSessionFolders(ctx, sqlcgen.ShiftSessionFoldersParams{
			ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
		}); e != nil {
			return e
		}
		if e := q.InsertSessionFolder(ctx, sqlcgen.InsertSessionFolderParams{
			ID: id, ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
			Name: name, Now: store.FormatTimestamp(g.now()),
		}); e != nil {
			return e
		}
		created, e = findSessionFolder(ctx, q, scope, id)
		return e
	})
	if err != nil {
		return store.SessionFolder{}, err
	}
	return created, nil
}

// RenameSessionFolder changes one folder's display name.
func (g *SessionRepo) RenameSessionFolder(
	ctx context.Context,
	scope store.SessionLayoutScope,
	id string,
	name string,
) (store.SessionFolder, error) {
	if err := g.checkReady(ctx, "rename session folder"); err != nil {
		return store.SessionFolder{}, err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return store.SessionFolder{}, err
	}
	if name, err = store.NormalizeSessionFolderName(name); err != nil {
		return store.SessionFolder{}, err
	}
	id = strings.TrimSpace(id)
	var renamed store.SessionFolder
	err = g.withImmediateTransaction(ctx, "rename session folder", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		rows, e := q.RenameSessionFolder(ctx, sqlcgen.RenameSessionFolderParams{
			Name: name, Now: store.FormatTimestamp(g.now()),
			ID: id, ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
		})
		if e != nil {
			return e
		}
		if rows == 0 {
			return fmt.Errorf("%w: %s", store.ErrSessionFolderNotFound, id)
		}
		renamed, e = findSessionFolder(ctx, q, scope, id)
		return e
	})
	if err != nil {
		return store.SessionFolder{}, err
	}
	return renamed, nil
}

// DeleteSessionFolder removes a folder. Its placements cascade away, which
// returns member sessions to the workspace root without touching sessions.
func (g *SessionRepo) DeleteSessionFolder(ctx context.Context, scope store.SessionLayoutScope, id string) error {
	if err := g.checkReady(ctx, "delete session folder"); err != nil {
		return err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	err = g.withImmediateTransaction(ctx, "delete session folder", func(exec globalSQLExecutor) error {
		rows, e := sqlcgen.New(exec).DeleteSessionFolder(ctx, sqlcgen.DeleteSessionFolderParams{
			ID: id, ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
		})
		if e != nil {
			return e
		}
		if rows == 0 {
			return fmt.Errorf("%w: %s", store.ErrSessionFolderNotFound, id)
		}
		return nil
	})
	return err
}

// ReorderSessionFolders applies a complete folder order for the scope.
func (g *SessionRepo) ReorderSessionFolders(
	ctx context.Context,
	scope store.SessionLayoutScope,
	ids []string,
) ([]store.SessionFolder, error) {
	if err := g.checkReady(ctx, "reorder session folders"); err != nil {
		return nil, err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return nil, err
	}
	ids, err = store.NormalizeSessionLayoutIDs(ids, store.MaxSessionFolders)
	if err != nil {
		return nil, err
	}
	var folders []store.SessionFolder
	err = g.withImmediateTransaction(ctx, "reorder session folders", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		current, e := listSessionFolders(ctx, q, scope)
		if e != nil {
			return e
		}
		if !sameSessionFolderSet(current, ids) {
			return fmt.Errorf("%w: folder order must list every folder exactly once", store.ErrSessionLayoutConflict)
		}
		now := store.FormatTimestamp(g.now())
		for position, id := range ids {
			if e := q.SetSessionFolderPosition(ctx, sqlcgen.SetSessionFolderPositionParams{
				Position: int64(position), Now: now,
				ID: id, ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
			}); e != nil {
				return e
			}
		}
		folders, e = listSessionFolders(ctx, q, scope)
		return e
	})
	if err != nil {
		return nil, err
	}
	return folders, nil
}

// MoveSession places a session at the end of a folder, or returns it to the
// unplaced workspace root when folderID is empty.
func (g *SessionRepo) MoveSession(
	ctx context.Context,
	scope store.SessionLayoutScope,
	sessionID string,
	folderID string,
) error {
	if err := g.checkReady(ctx, "move session"); err != nil {
		return err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return err
	}
	sessionID = strings.TrimSpace(sessionID)
	folderID = strings.TrimSpace(folderID)
	err = g.withImmediateTransaction(ctx, "move session", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if e := requireSessionLayoutMember(ctx, q, scope, sessionID); e != nil {
			return e
		}
		if folderID == "" {
			return q.DeleteSessionPlacement(ctx, sqlcgen.DeleteSessionPlacementParams{
				SessionID: sessionID, WorkspaceID: scope.WorkspaceID,
			})
		}
		if _, e := findSessionFolder(ctx, q, scope, folderID); e != nil {
			return e
		}
		members, e := listSessionContainer(ctx, q, scope, folderID)
		if e != nil {
			return e
		}
		if slices.Contains(members, sessionID) {
			return nil
		}
		return upsertSessionPlacement(ctx, q, scope, folderID, sessionID, len(members), store.FormatTimestamp(g.now()))
	})
	return err
}

// ReorderSessions pins the listed sessions, in order, at the head of one
// container (a folder, or the workspace root when folderID is empty). Members
// already in the container but absent from the list keep their relative order
// after the listed sessions, so a partial page never drops placements.
func (g *SessionRepo) ReorderSessions(
	ctx context.Context,
	scope store.SessionLayoutScope,
	folderID string,
	sessionIDs []string,
) error {
	if err := g.checkReady(ctx, "reorder sessions"); err != nil {
		return err
	}
	scope, err := scope.Normalize()
	if err != nil {
		return err
	}
	folderID = strings.TrimSpace(folderID)
	sessionIDs, err = store.NormalizeSessionLayoutIDs(sessionIDs, store.MaxSessionLayoutOrder)
	if err != nil {
		return err
	}
	if len(sessionIDs) == 0 {
		return fmt.Errorf("%w: session order is empty", store.ErrSessionLayoutInvalid)
	}
	err = g.withImmediateTransaction(ctx, "reorder sessions", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if folderID != "" {
			if _, e := findSessionFolder(ctx, q, scope, folderID); e != nil {
				return e
			}
		}
		listed := make(map[string]struct{}, len(sessionIDs))
		for _, sessionID := range sessionIDs {
			if e := requireSessionLayoutMember(ctx, q, scope, sessionID); e != nil {
				return e
			}
			listed[sessionID] = struct{}{}
		}
		members, e := listSessionContainer(ctx, q, scope, folderID)
		if e != nil {
			return e
		}
		order := append([]string(nil), sessionIDs...)
		for _, member := range members {
			if _, ok := listed[member]; !ok {
				order = append(order, member)
			}
		}
		now := store.FormatTimestamp(g.now())
		for position, sessionID := range order {
			if e := upsertSessionPlacement(ctx, q, scope, folderID, sessionID, position, now); e != nil {
				return e
			}
		}
		return nil
	})
	return err
}

// SessionPlacements returns placements for a bounded set of session IDs.
func (g *SessionRepo) SessionPlacements(
	ctx context.Context,
	sessionIDs []string,
) (map[string]store.SessionPlacement, error) {
	if err := g.checkReady(ctx, "read session placements"); err != nil {
		return nil, err
	}
	if len(sessionIDs) == 0 {
		return map[string]store.SessionPlacement{}, nil
	}
	if len(sessionIDs) > store.MaxSessionPlacementLookup {
		return nil, fmt.Errorf(
			"%w: at most %d placements can be read at once",
			store.ErrSessionLayoutInvalid,
			store.MaxSessionPlacementLookup,
		)
	}
	encoded, err := json.Marshal(sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("store: encode session placement ids: %w", err)
	}
	rows, err := g.queries.ListSessionPlacementsByIDs(ctx, string(encoded))
	if err != nil {
		return nil, fmt.Errorf("store: read session placements: %w", err)
	}
	placements := make(map[string]store.SessionPlacement, len(rows))
	for _, row := range rows {
		placements[row.SessionID] = store.SessionPlacement{
			SessionID:      row.SessionID,
			FolderID:       row.FolderID.String,
			Position:       int(row.Position),
			FolderPosition: int(row.FolderPosition.Int64),
		}
	}
	return placements, nil
}

func listSessionFolders(
	ctx context.Context,
	q *sqlcgen.Queries,
	scope store.SessionLayoutScope,
) ([]store.SessionFolder, error) {
	rows, err := q.ListSessionFolders(ctx, sqlcgen.ListSessionFoldersParams{
		ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("store: list session folders: %w", err)
	}
	folders := make([]store.SessionFolder, 0, len(rows))
	for _, row := range rows {
		folder, err := sessionFolderFromRow(row)
		if err != nil {
			return nil, err
		}
		folders = append(folders, folder)
	}
	return folders, nil
}

func findSessionFolder(
	ctx context.Context,
	q *sqlcgen.Queries,
	scope store.SessionLayoutScope,
	id string,
) (store.SessionFolder, error) {
	folders, err := listSessionFolders(ctx, q, scope)
	if err != nil {
		return store.SessionFolder{}, err
	}
	for _, folder := range folders {
		if folder.ID == id {
			return folder, nil
		}
	}
	return store.SessionFolder{}, fmt.Errorf("%w: %s", store.ErrSessionFolderNotFound, id)
}

func sessionFolderFromRow(row sqlcgen.ListSessionFoldersRow) (store.SessionFolder, error) {
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return store.SessionFolder{}, fmt.Errorf("store: parse session folder %q created_at: %w", row.ID, err)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return store.SessionFolder{}, fmt.Errorf("store: parse session folder %q updated_at: %w", row.ID, err)
	}
	return store.SessionFolder{
		ID:           row.ID,
		ProfileID:    row.ProfileID,
		WorkspaceID:  row.WorkspaceID,
		Name:         row.Name,
		Position:     int(row.Position),
		SessionCount: int(row.SessionCount),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}, nil
}

func sameSessionFolderSet(current []store.SessionFolder, ids []string) bool {
	if len(current) != len(ids) {
		return false
	}
	known := make(map[string]struct{}, len(current))
	for _, folder := range current {
		known[folder.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := known[id]; !ok {
			return false
		}
	}
	return true
}

// requireSessionLayoutMember enforces workspace and profile isolation: a
// session outside the scope reads as missing.
func requireSessionLayoutMember(
	ctx context.Context,
	q *sqlcgen.Queries,
	scope store.SessionLayoutScope,
	sessionID string,
) error {
	if sessionID == "" {
		return fmt.Errorf("%w: session id is required", store.ErrSessionLayoutInvalid)
	}
	owner, err := q.GetSessionLayoutOwner(ctx, sqlcgen.GetSessionLayoutOwnerParams{
		WorkspaceID: scope.WorkspaceID, ID: sessionID,
	})
	if errors.Is(err, sql.ErrNoRows) || (err == nil && owner.ProfileID != scope.ProfileID) {
		return fmt.Errorf("%w: %s", store.ErrSessionNotFound, sessionID)
	}
	if err != nil {
		return fmt.Errorf("store: read session layout owner %q: %w", sessionID, err)
	}
	if owner.SpawnRole.Valid && owner.SpawnRole.String == store.SubagentSpawnRole {
		return fmt.Errorf("%w: %s", store.ErrSessionLayoutSubagent, sessionID)
	}
	return nil
}

func listSessionContainer(
	ctx context.Context,
	q *sqlcgen.Queries,
	scope store.SessionLayoutScope,
	folderID string,
) ([]string, error) {
	members, err := q.ListSessionContainerMembers(ctx, sqlcgen.ListSessionContainerMembersParams{
		ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
		FolderID: sql.NullString{String: folderID, Valid: folderID != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("store: list session container: %w", err)
	}
	return members, nil
}

func upsertSessionPlacement(
	ctx context.Context,
	q *sqlcgen.Queries,
	scope store.SessionLayoutScope,
	folderID string,
	sessionID string,
	position int,
	now string,
) error {
	return q.UpsertSessionPlacement(ctx, sqlcgen.UpsertSessionPlacementParams{
		SessionID: sessionID, ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID,
		FolderID: sql.NullString{String: folderID, Valid: folderID != ""},
		Position: int64(position), Now: now,
	})
}
