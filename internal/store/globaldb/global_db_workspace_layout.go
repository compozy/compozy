package globaldb

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.WorkspaceLayoutStore = (*SessionRepo)(nil)

// WorkspaceLayout returns one profile's workspace folders and placements.
func (g *SessionRepo) WorkspaceLayout(ctx context.Context, profileID string) (store.WorkspaceLayout, error) {
	if err := g.checkReady(ctx, "read workspace layout"); err != nil {
		return store.WorkspaceLayout{}, err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return store.WorkspaceLayout{}, err
	}
	return readWorkspaceLayout(ctx, g.queries, profileID)
}

// CreateWorkspaceFolder inserts a root folder at the top of the folder order.
func (g *SessionRepo) CreateWorkspaceFolder(
	ctx context.Context,
	profileID string,
	id string,
	name string,
) (store.WorkspaceFolder, error) {
	if err := g.checkReady(ctx, "create workspace folder"); err != nil {
		return store.WorkspaceFolder{}, err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return store.WorkspaceFolder{}, fmt.Errorf("%w: folder id is required", store.ErrWorkspaceLayoutInvalid)
	}
	if name, err = normalizeWorkspaceFolderName(name); err != nil {
		return store.WorkspaceFolder{}, err
	}
	var created store.WorkspaceFolder
	err = g.withImmediateTransaction(ctx, "create workspace folder", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		count, e := q.CountWorkspaceFolders(ctx, profileID)
		if e != nil {
			return e
		}
		if count >= store.MaxWorkspaceFolders {
			return store.ErrWorkspaceFolderLimit
		}
		if e := q.ShiftWorkspaceFolders(ctx, profileID); e != nil {
			return e
		}
		if e := q.InsertWorkspaceFolder(ctx, sqlcgen.InsertWorkspaceFolderParams{
			ID: id, ProfileID: profileID, Name: name, Now: store.FormatTimestamp(g.now()),
		}); e != nil {
			return e
		}
		created, e = findWorkspaceFolder(ctx, q, profileID, id)
		return e
	})
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	return created, nil
}

// RenameWorkspaceFolder changes one folder's display name.
func (g *SessionRepo) RenameWorkspaceFolder(
	ctx context.Context,
	profileID string,
	id string,
	name string,
) (store.WorkspaceFolder, error) {
	if err := g.checkReady(ctx, "rename workspace folder"); err != nil {
		return store.WorkspaceFolder{}, err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	if name, err = normalizeWorkspaceFolderName(name); err != nil {
		return store.WorkspaceFolder{}, err
	}
	id = strings.TrimSpace(id)
	var renamed store.WorkspaceFolder
	err = g.withImmediateTransaction(ctx, "rename workspace folder", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		rows, e := q.RenameWorkspaceFolder(ctx, sqlcgen.RenameWorkspaceFolderParams{
			Name: name, Now: store.FormatTimestamp(g.now()), ID: id, ProfileID: profileID,
		})
		if e != nil {
			return e
		}
		if rows == 0 {
			return fmt.Errorf("%w: %s", store.ErrWorkspaceFolderNotFound, id)
		}
		renamed, e = findWorkspaceFolder(ctx, q, profileID, id)
		return e
	})
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	return renamed, nil
}

// DeleteWorkspaceFolder removes a folder. Its placements cascade away, which
// returns member workspaces to the unplaced root without touching them.
func (g *SessionRepo) DeleteWorkspaceFolder(ctx context.Context, profileID string, id string) error {
	if err := g.checkReady(ctx, "delete workspace folder"); err != nil {
		return err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	return g.withImmediateTransaction(ctx, "delete workspace folder", func(exec globalSQLExecutor) error {
		rows, e := sqlcgen.New(exec).DeleteWorkspaceFolder(ctx, sqlcgen.DeleteWorkspaceFolderParams{
			ID: id, ProfileID: profileID,
		})
		if e != nil {
			return e
		}
		if rows == 0 {
			return fmt.Errorf("%w: %s", store.ErrWorkspaceFolderNotFound, id)
		}
		return nil
	})
}

// ReorderWorkspaceFolders applies a complete folder order for the profile.
func (g *SessionRepo) ReorderWorkspaceFolders(ctx context.Context, profileID string, ids []string) error {
	if err := g.checkReady(ctx, "reorder workspace folders"); err != nil {
		return err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return err
	}
	ids, err = store.NormalizeSessionLayoutIDs(ids, store.MaxWorkspaceFolders)
	if err != nil {
		return fmt.Errorf("%w: %w", store.ErrWorkspaceLayoutInvalid, err)
	}
	return g.withImmediateTransaction(ctx, "reorder workspace folders", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		current, e := listWorkspaceFolders(ctx, q, profileID)
		if e != nil {
			return e
		}
		if !sameWorkspaceFolderSet(current, ids) {
			return fmt.Errorf("%w: folder order must list every folder exactly once", store.ErrWorkspaceLayoutConflict)
		}
		now := store.FormatTimestamp(g.now())
		for position, id := range ids {
			if e := q.SetWorkspaceFolderPosition(ctx, sqlcgen.SetWorkspaceFolderPositionParams{
				Position: int64(position), Now: now, ID: id, ProfileID: profileID,
			}); e != nil {
				return e
			}
		}
		return nil
	})
}

// MoveWorkspace changes a workspace's container: it joins the end of a folder,
// or the end of the placed root when folderID is empty. A workspace already in
// that container keeps its position; ReorderWorkspaces owns order within it.
func (g *SessionRepo) MoveWorkspace(ctx context.Context, profileID string, workspaceID string, folderID string) error {
	if err := g.checkReady(ctx, "move workspace"); err != nil {
		return err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return err
	}
	workspaceID = strings.TrimSpace(workspaceID)
	folderID = strings.TrimSpace(folderID)
	return g.withImmediateTransaction(ctx, "move workspace", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if e := requireWorkspaceLayoutTarget(ctx, q, workspaceID); e != nil {
			return e
		}
		if folderID != "" {
			if _, e := findWorkspaceFolder(ctx, q, profileID, folderID); e != nil {
				return e
			}
		}
		members, e := listWorkspaceContainer(ctx, q, profileID, folderID)
		if e != nil {
			return e
		}
		if slices.Contains(members, workspaceID) {
			return nil
		}
		// Moves and deletes leave gaps, so append after the highest position
		// rather than at len(members), which could tie with the last member.
		next, e := q.NextWorkspaceContainerPosition(ctx, sqlcgen.NextWorkspaceContainerPositionParams{
			ProfileID: profileID, FolderID: nullableFolderID(folderID),
		})
		if e != nil {
			return e
		}
		return upsertWorkspacePlacement(
			ctx,
			q,
			profileID,
			folderID,
			workspaceID,
			int(next),
			store.FormatTimestamp(g.now()),
		)
	})
}

// ReorderWorkspaces pins the listed workspaces, in order, at the head of one
// container (a folder, or the root when folderID is empty). Members already in
// the container but absent from the list keep their relative order after the
// listed workspaces.
func (g *SessionRepo) ReorderWorkspaces(
	ctx context.Context,
	profileID string,
	folderID string,
	workspaceIDs []string,
) error {
	if err := g.checkReady(ctx, "reorder workspaces"); err != nil {
		return err
	}
	profileID, err := store.NormalizeWorkspaceLayoutProfile(profileID)
	if err != nil {
		return err
	}
	folderID = strings.TrimSpace(folderID)
	workspaceIDs, err = normalizeWorkspaceLayoutIDs(workspaceIDs)
	if err != nil {
		return err
	}
	return g.withImmediateTransaction(ctx, "reorder workspaces", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if folderID != "" {
			if _, e := findWorkspaceFolder(ctx, q, profileID, folderID); e != nil {
				return e
			}
		}
		listed := make(map[string]struct{}, len(workspaceIDs))
		for _, workspaceID := range workspaceIDs {
			if e := requireWorkspaceLayoutTarget(ctx, q, workspaceID); e != nil {
				return e
			}
			listed[workspaceID] = struct{}{}
		}
		members, e := listWorkspaceContainer(ctx, q, profileID, folderID)
		if e != nil {
			return e
		}
		order := append([]string(nil), workspaceIDs...)
		for _, member := range members {
			if _, ok := listed[member]; !ok {
				order = append(order, member)
			}
		}
		now := store.FormatTimestamp(g.now())
		for position, workspaceID := range order {
			if e := upsertWorkspacePlacement(ctx, q, profileID, folderID, workspaceID, position, now); e != nil {
				return e
			}
		}
		return nil
	})
}

func readWorkspaceLayout(ctx context.Context, q *sqlcgen.Queries, profileID string) (store.WorkspaceLayout, error) {
	folders, err := listWorkspaceFolders(ctx, q, profileID)
	if err != nil {
		return store.WorkspaceLayout{}, err
	}
	rows, err := q.ListWorkspacePlacements(ctx, profileID)
	if err != nil {
		return store.WorkspaceLayout{}, fmt.Errorf("store: list workspace placements: %w", err)
	}
	placements := make([]store.WorkspacePlacement, 0, len(rows))
	for _, row := range rows {
		placements = append(placements, store.WorkspacePlacement{
			WorkspaceID: row.WorkspaceID,
			FolderID:    row.FolderID.String,
			Position:    int(row.Position),
		})
	}
	return store.WorkspaceLayout{Folders: folders, Placements: placements}, nil
}

func listWorkspaceFolders(ctx context.Context, q *sqlcgen.Queries, profileID string) ([]store.WorkspaceFolder, error) {
	rows, err := q.ListWorkspaceFolders(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("store: list workspace folders: %w", err)
	}
	folders := make([]store.WorkspaceFolder, 0, len(rows))
	for _, row := range rows {
		createdAt, err := store.ParseTimestamp(row.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse workspace folder %q created_at: %w", row.ID, err)
		}
		updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("store: parse workspace folder %q updated_at: %w", row.ID, err)
		}
		folders = append(folders, store.WorkspaceFolder{
			ID:             row.ID,
			ProfileID:      row.ProfileID,
			Name:           row.Name,
			Position:       int(row.Position),
			WorkspaceCount: int(row.WorkspaceCount),
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		})
	}
	return folders, nil
}

func findWorkspaceFolder(
	ctx context.Context,
	q *sqlcgen.Queries,
	profileID string,
	id string,
) (store.WorkspaceFolder, error) {
	folders, err := listWorkspaceFolders(ctx, q, profileID)
	if err != nil {
		return store.WorkspaceFolder{}, err
	}
	for _, folder := range folders {
		if folder.ID == id {
			return folder, nil
		}
	}
	return store.WorkspaceFolder{}, fmt.Errorf("%w: %s", store.ErrWorkspaceFolderNotFound, id)
}

func sameWorkspaceFolderSet(current []store.WorkspaceFolder, ids []string) bool {
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

// requireWorkspaceLayoutTarget accepts a registered workspace or the User
// scope (empty id). Workspaces are global across profiles, so any registered
// workspace may be placed in the caller profile's layout.
func requireWorkspaceLayoutTarget(ctx context.Context, q *sqlcgen.Queries, workspaceID string) error {
	if workspaceID == "" {
		return nil
	}
	found, err := q.WorkspaceLayoutTargetExists(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("store: read workspace %q: %w", workspaceID, err)
	}
	if !found {
		return fmt.Errorf("%w: %s", store.ErrWorkspaceLayoutTargetNotFound, workspaceID)
	}
	return nil
}

func listWorkspaceContainer(
	ctx context.Context,
	q *sqlcgen.Queries,
	profileID string,
	folderID string,
) ([]string, error) {
	members, err := q.ListWorkspaceContainerMembers(ctx, sqlcgen.ListWorkspaceContainerMembersParams{
		ProfileID: profileID, FolderID: nullableFolderID(folderID),
	})
	if err != nil {
		return nil, fmt.Errorf("store: list workspace container: %w", err)
	}
	return members, nil
}

func upsertWorkspacePlacement(
	ctx context.Context,
	q *sqlcgen.Queries,
	profileID string,
	folderID string,
	workspaceID string,
	position int,
	now string,
) error {
	return q.UpsertWorkspacePlacement(ctx, sqlcgen.UpsertWorkspacePlacementParams{
		ProfileID: profileID, WorkspaceID: workspaceID,
		FolderID: nullableFolderID(folderID), Position: int64(position), Now: now,
	})
}

func nullableFolderID(folderID string) sql.NullString {
	return sql.NullString{String: folderID, Valid: folderID != ""}
}

func normalizeWorkspaceFolderName(name string) (string, error) {
	name, err := store.NormalizeSessionFolderName(name)
	if err != nil {
		return "", fmt.Errorf("%w: %w", store.ErrWorkspaceLayoutInvalid, err)
	}
	return name, nil
}

// normalizeWorkspaceLayoutIDs trims and bounds a workspace order. The User
// scope is the empty id, so blanks are kept, but each id appears once.
func normalizeWorkspaceLayoutIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: workspace order is empty", store.ErrWorkspaceLayoutInvalid)
	}
	if len(ids) > store.MaxWorkspaceLayoutOrder {
		return nil, fmt.Errorf(
			"%w: at most %d ids are allowed", store.ErrWorkspaceLayoutInvalid, store.MaxWorkspaceLayoutOrder,
		)
	}
	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("%w: duplicate workspace id %q", store.ErrWorkspaceLayoutInvalid, id)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized, nil
}
