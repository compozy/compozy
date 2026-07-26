package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

// InsertWorkspace creates a new persisted workspace registration row.
func (g *WorkspaceRepo) InsertWorkspace(ctx context.Context, ws aghworkspace.Workspace) error {
	if err := g.checkReady(ctx, "insert workspace"); err != nil {
		return err
	}

	normalized, addDirsJSON, err := g.normalizeWorkspaceForInsert(ws)
	if err != nil {
		return err
	}

	if err := g.queries.InsertWorkspace(ctx, sqlcgen.InsertWorkspaceParams{
		ID: normalized.ID, RootDir: normalized.RootDir, AddDirs: addDirsJSON, Name: normalized.Name,
		DefaultAgent: nullableWorkspaceString(normalized.DefaultAgent), SandboxRef: normalized.SandboxRef,
		CreatedAt: store.FormatTimestamp(normalized.CreatedAt), UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt),
	}); err != nil {
		return fmt.Errorf(
			"store: insert workspace %q: %w",
			normalized.ID,
			mapWorkspaceWriteConstraintError(ctx, g.db, normalized, err),
		)
	}

	return nil
}

// UpdateWorkspace updates an existing persisted workspace registration row.
func (g *WorkspaceRepo) UpdateWorkspace(ctx context.Context, ws aghworkspace.Workspace) error {
	if err := g.checkReady(ctx, "update workspace"); err != nil {
		return err
	}

	normalized, addDirsJSON, err := g.normalizeWorkspaceForUpdate(ws)
	if err != nil {
		return err
	}

	affected, err := g.queries.UpdateWorkspace(ctx, sqlcgen.UpdateWorkspaceParams{
		RootDir: normalized.RootDir, AddDirs: addDirsJSON, Name: normalized.Name,
		DefaultAgent: nullableWorkspaceString(normalized.DefaultAgent), SandboxRef: normalized.SandboxRef,
		UpdatedAt: store.FormatTimestamp(normalized.UpdatedAt), ID: normalized.ID,
	})
	if err != nil {
		return fmt.Errorf(
			"store: update workspace %q: %w",
			normalized.ID,
			mapWorkspaceWriteConstraintError(ctx, g.db, normalized, err),
		)
	}

	if affected == 0 {
		return fmt.Errorf("store: workspace %q: %w", normalized.ID, aghworkspace.ErrWorkspaceNotFound)
	}

	return nil
}

// DeleteWorkspace removes a persisted workspace registration row.
// It refuses to delete if any active sessions reference the workspace.
// Stopped or orphaned sessions are cleaned up automatically before deletion.
func (g *WorkspaceRepo) DeleteWorkspace(ctx context.Context, id string) error {
	if err := g.checkReady(ctx, "delete workspace"); err != nil {
		return err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return errors.New("store: workspace id is required")
	}

	return store.ExecuteWrite(ctx, g.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		activeSessions, err := queries.ListActiveSessionIDsByWorkspace(ctx, trimmedID)
		if err != nil {
			return err
		}
		if len(activeSessions) > 0 {
			return fmt.Errorf(
				"store: delete workspace %q: %w: %s",
				trimmedID,
				aghworkspace.ErrWorkspaceHasActiveSessions,
				strings.Join(activeSessions, ", "),
			)
		}

		mcpAuthRefs, err := queries.ListMCPAuthTokenRefsByWorkspace(ctx, trimmedID)
		if err != nil {
			return fmt.Errorf("store: list MCP auth token refs for workspace %q: %w", trimmedID, err)
		}
		for _, refs := range mcpAuthRefs {
			for _, ref := range []string{refs.AccessTokenRef, refs.RefreshTokenRef} {
				if strings.TrimSpace(ref) == "" {
					continue
				}
				if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
					return fmt.Errorf("store: delete MCP auth secret for workspace %q: %w", trimmedID, err)
				}
			}
		}
		if _, err := queries.DeleteMCPAuthTokensByWorkspace(ctx, trimmedID); err != nil {
			return fmt.Errorf("store: delete MCP auth tokens for workspace %q: %w", trimmedID, err)
		}

		if err := queries.DeleteSessionsByWorkspace(ctx, trimmedID); err != nil {
			return fmt.Errorf("store: delete stopped sessions for workspace %q: %w", trimmedID, err)
		}

		affected, err := queries.DeleteWorkspace(ctx, trimmedID)
		if err != nil {
			return fmt.Errorf("store: delete workspace %q: %w", trimmedID, mapWorkspaceDeleteConstraintError(err))
		}

		if affected == 0 {
			return fmt.Errorf("store: workspace %q: %w", trimmedID, aghworkspace.ErrWorkspaceNotFound)
		}

		return nil
	})
}

// GetWorkspace loads a workspace registration by primary key.
func (g *WorkspaceRepo) GetWorkspace(ctx context.Context, id string) (aghworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "get workspace"); err != nil {
		return aghworkspace.Workspace{}, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return aghworkspace.Workspace{}, errors.New("store: workspace id is required")
	}

	row, err := g.queries.GetWorkspace(ctx, trimmedID)
	return workspaceFromGenerated(row, err)
}

// GetWorkspaceByPath loads a workspace registration by canonical root directory.
func (g *WorkspaceRepo) GetWorkspaceByPath(ctx context.Context, rootDir string) (aghworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "get workspace by path"); err != nil {
		return aghworkspace.Workspace{}, err
	}

	trimmedRoot := strings.TrimSpace(rootDir)
	if trimmedRoot == "" {
		return aghworkspace.Workspace{}, errors.New("store: workspace root directory is required")
	}

	row, err := g.queries.GetWorkspaceByPath(ctx, trimmedRoot)
	return workspaceFromGenerated(row, err)
}

// GetWorkspaceByName loads a workspace registration by unique workspace name.
func (g *WorkspaceRepo) GetWorkspaceByName(ctx context.Context, name string) (aghworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "get workspace by name"); err != nil {
		return aghworkspace.Workspace{}, err
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return aghworkspace.Workspace{}, errors.New("store: workspace name is required")
	}

	row, err := g.queries.GetWorkspaceByName(ctx, trimmedName)
	return workspaceFromGenerated(row, err)
}

// ListWorkspaces returns all registered workspaces in stable name order.
func (g *WorkspaceRepo) ListWorkspaces(ctx context.Context) ([]aghworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "list workspaces"); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: query workspaces: %w", err)
	}
	workspaces := make([]aghworkspace.Workspace, 0, len(rows))
	for _, row := range rows {
		ws, scanErr := workspaceFromGenerated(row, nil)
		if scanErr != nil {
			return nil, scanErr
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (g *WorkspaceRepo) normalizeWorkspaceForInsert(ws aghworkspace.Workspace) (aghworkspace.Workspace, string, error) {
	normalized, addDirsJSON, err := normalizeWorkspaceRecord(ws)
	if err != nil {
		return aghworkspace.Workspace{}, "", err
	}

	if strings.TrimSpace(normalized.ID) == "" {
		normalized.ID = aghworkspace.NewWorkspaceID()
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	return normalized, addDirsJSON, nil
}

func (g *WorkspaceRepo) normalizeWorkspaceForUpdate(ws aghworkspace.Workspace) (aghworkspace.Workspace, string, error) {
	normalized, addDirsJSON, err := normalizeWorkspaceRecord(ws)
	if err != nil {
		return aghworkspace.Workspace{}, "", err
	}

	if strings.TrimSpace(normalized.ID) == "" {
		return aghworkspace.Workspace{}, "", errors.New("store: workspace id is required")
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}

	return normalized, addDirsJSON, nil
}

func workspaceFromGenerated(row sqlcgen.Workspace, queryErr error) (aghworkspace.Workspace, error) {
	if queryErr != nil {
		if errors.Is(queryErr, sql.ErrNoRows) {
			return aghworkspace.Workspace{}, aghworkspace.ErrWorkspaceNotFound
		}
		return aghworkspace.Workspace{}, fmt.Errorf("store: scan workspace: %w", queryErr)
	}
	addDirs, err := decodeWorkspaceDirs(row.AddDirs)
	if err != nil {
		return aghworkspace.Workspace{}, err
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return aghworkspace.Workspace{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return aghworkspace.Workspace{}, err
	}
	return aghworkspace.Workspace{
		ID: row.ID, RootDir: row.RootDir, AdditionalDirs: addDirs, Name: row.Name,
		DefaultAgent: strings.TrimSpace(row.DefaultAgent.String), SandboxRef: strings.TrimSpace(row.SandboxRef),
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func nullableWorkspaceString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func normalizeWorkspaceRecord(ws aghworkspace.Workspace) (aghworkspace.Workspace, string, error) {
	normalized := ws
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.RootDir = strings.TrimSpace(normalized.RootDir)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.DefaultAgent = strings.TrimSpace(normalized.DefaultAgent)
	normalized.SandboxRef = strings.TrimSpace(normalized.SandboxRef)
	normalized.AdditionalDirs = compactStrings(normalized.AdditionalDirs)

	switch {
	case normalized.RootDir == "":
		return aghworkspace.Workspace{}, "", errors.New("store: workspace root directory is required")
	case normalized.Name == "":
		return aghworkspace.Workspace{}, "", errors.New("store: workspace name is required")
	}

	addDirsJSON, err := encodeWorkspaceDirs(normalized.AdditionalDirs)
	if err != nil {
		return aghworkspace.Workspace{}, "", err
	}

	return normalized, addDirsJSON, nil
}

func encodeWorkspaceDirs(dirs []string) (string, error) {
	if len(dirs) == 0 {
		return "[]", nil
	}

	payload, err := json.Marshal(compactStrings(dirs))
	if err != nil {
		return "", fmt.Errorf("store: encode workspace add_dirs: %w", err)
	}
	return string(payload), nil
}

func decodeWorkspaceDirs(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var dirs []string
	if err := json.Unmarshal([]byte(trimmed), &dirs); err != nil {
		return nil, fmt.Errorf("store: decode workspace add_dirs: %w", err)
	}

	return compactStrings(dirs), nil
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func mapWorkspaceWriteConstraintError(
	ctx context.Context,
	exec globalSQLExecutor,
	workspace aghworkspace.Workspace,
	err error,
) error {
	if err == nil {
		return nil
	}
	if !isSQLiteUniqueConstraint(err) {
		return err
	}

	queries := sqlcgen.New(exec)
	byPath, pathErr := queries.GetWorkspaceByPath(ctx, workspace.RootDir)
	switch {
	case pathErr == nil && byPath.ID != workspace.ID:
		return aghworkspace.ErrWorkspacePathTaken
	case pathErr != nil && !errors.Is(pathErr, sql.ErrNoRows):
		return errors.Join(err, fmt.Errorf("store: classify workspace path constraint: %w", pathErr))
	}

	byName, nameErr := queries.GetWorkspaceByName(ctx, workspace.Name)
	switch {
	case nameErr == nil && byName.ID != workspace.ID:
		return aghworkspace.ErrWorkspaceNameTaken
	case nameErr != nil && !errors.Is(nameErr, sql.ErrNoRows):
		return errors.Join(err, fmt.Errorf("store: classify workspace name constraint: %w", nameErr))
	default:
		return err
	}
}

func mapWorkspaceDeleteConstraintError(err error) error {
	if err == nil {
		return nil
	}
	if isSQLiteForeignKeyConstraint(err) {
		return aghworkspace.ErrWorkspaceHasSessions
	}
	return err
}
