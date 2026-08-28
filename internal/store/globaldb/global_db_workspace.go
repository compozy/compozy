package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

// InsertWorkspace creates a new persisted workspace registration row.
func (g *WorkspaceRepo) InsertWorkspace(ctx context.Context, ws compozyworkspace.Workspace) error {
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
func (g *WorkspaceRepo) UpdateWorkspace(ctx context.Context, ws compozyworkspace.Workspace) error {
	if err := g.checkReady(ctx, "update workspace"); err != nil {
		return err
	}

	normalized, addDirsJSON, err := g.normalizeWorkspaceForUpdate(ws)
	if err != nil {
		return err
	}

	affected, err := g.queries.UpdateWorkspace(ctx, sqlcgen.UpdateWorkspaceParams{
		RootDir: normalized.RootDir, AddDirs: addDirsJSON, Name: normalized.Name,
		DefaultAgent: nullableWorkspaceString(
			normalized.DefaultAgent,
		), SandboxRef: normalized.SandboxRef,
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
		return fmt.Errorf(
			"store: workspace %q: %w",
			normalized.ID,
			compozyworkspace.ErrWorkspaceNotFound,
		)
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
		return deleteWorkspaceTx(ctx, sqlcgen.New(tx), trimmedID)
	})
}

func ensureWorkspaceDeletionAllowed(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID string,
) error {
	activeSessions, err := queries.ListActiveSessionIDsByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	if len(activeSessions) > 0 {
		return fmt.Errorf(
			"store: delete workspace %q: %w: %s",
			workspaceID,
			compozyworkspace.ErrWorkspaceHasActiveSessions,
			strings.Join(activeSessions, ", "),
		)
	}

	return nil
}

func deleteWorkspaceExtensionEnv(
	ctx context.Context,
	queries *sqlcgen.Queries,
	workspaceID string,
) error {
	extensionSecretRefs, err := queries.ListExtensionEnvSecretRefsByWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf(
			"store: list extension secret refs for workspace %q: %w",
			workspaceID,
			err,
		)
	}
	for _, ref := range extensionSecretRefs {
		if _, err := queries.DeleteVaultSecret(ctx, ref); err != nil {
			return fmt.Errorf(
				"store: delete extension secret for workspace %q: %w",
				workspaceID,
				err,
			)
		}
	}
	if _, err := queries.DeleteExtensionEnvBindingsByWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf(
			"store: delete extension env bindings for workspace %q: %w",
			workspaceID,
			err,
		)
	}

	return nil
}

// GetWorkspace loads a workspace registration by primary key.
func (g *WorkspaceRepo) GetWorkspace(
	ctx context.Context,
	id string,
) (compozyworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "get workspace"); err != nil {
		return compozyworkspace.Workspace{}, err
	}

	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return compozyworkspace.Workspace{}, errors.New("store: workspace id is required")
	}

	row, err := g.queries.GetWorkspace(ctx, trimmedID)
	return workspaceFromGenerated(row, err)
}

// GetWorkspaceByPath loads a workspace registration by canonical root directory.
func (g *WorkspaceRepo) GetWorkspaceByPath(
	ctx context.Context,
	rootDir string,
) (compozyworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "get workspace by path"); err != nil {
		return compozyworkspace.Workspace{}, err
	}

	trimmedRoot := strings.TrimSpace(rootDir)
	if trimmedRoot == "" {
		return compozyworkspace.Workspace{}, errors.New(
			"store: workspace root directory is required",
		)
	}

	row, err := g.queries.GetWorkspaceByPath(ctx, trimmedRoot)
	return workspaceFromGenerated(row, err)
}

// GetWorkspaceByName loads a workspace registration by unique workspace name.
func (g *WorkspaceRepo) GetWorkspaceByName(
	ctx context.Context,
	name string,
) (compozyworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "get workspace by name"); err != nil {
		return compozyworkspace.Workspace{}, err
	}

	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return compozyworkspace.Workspace{}, errors.New("store: workspace name is required")
	}

	row, err := g.queries.GetWorkspaceByName(ctx, trimmedName)
	return workspaceFromGenerated(row, err)
}

// ListWorkspaces returns all registered workspaces in stable name order.
func (g *WorkspaceRepo) ListWorkspaces(ctx context.Context) ([]compozyworkspace.Workspace, error) {
	if err := g.checkReady(ctx, "list workspaces"); err != nil {
		return nil, err
	}

	rows, err := g.queries.ListWorkspaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("store: query workspaces: %w", err)
	}
	workspaces := make([]compozyworkspace.Workspace, 0, len(rows))
	for _, row := range rows {
		ws, scanErr := workspaceFromGenerated(row, nil)
		if scanErr != nil {
			return nil, scanErr
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, nil
}

func (g *WorkspaceRepo) normalizeWorkspaceForInsert(
	ws compozyworkspace.Workspace,
) (compozyworkspace.Workspace, string, error) {
	normalized, addDirsJSON, err := normalizeWorkspaceRecord(ws)
	if err != nil {
		return compozyworkspace.Workspace{}, "", err
	}

	if strings.TrimSpace(normalized.ID) == "" {
		generatedID, generateErr := g.generateID()
		if generateErr != nil {
			return compozyworkspace.Workspace{}, "", fmt.Errorf(
				"store: generate workspace id: %w",
				generateErr,
			)
		}
		normalized.ID = strings.TrimSpace(generatedID)
		if !compozyworkspace.IsWorkspaceID(normalized.ID) {
			return compozyworkspace.Workspace{}, "", fmt.Errorf(
				"store: generated invalid workspace id %q",
				normalized.ID,
			)
		}
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}

	return normalized, addDirsJSON, nil
}

func (g *WorkspaceRepo) normalizeWorkspaceForUpdate(
	ws compozyworkspace.Workspace,
) (compozyworkspace.Workspace, string, error) {
	normalized, addDirsJSON, err := normalizeWorkspaceRecord(ws)
	if err != nil {
		return compozyworkspace.Workspace{}, "", err
	}

	if strings.TrimSpace(normalized.ID) == "" {
		return compozyworkspace.Workspace{}, "", errors.New("store: workspace id is required")
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}

	return normalized, addDirsJSON, nil
}

func workspaceFromGenerated(
	row sqlcgen.Workspace,
	queryErr error,
) (compozyworkspace.Workspace, error) {
	if queryErr != nil {
		if errors.Is(queryErr, sql.ErrNoRows) {
			return compozyworkspace.Workspace{}, compozyworkspace.ErrWorkspaceNotFound
		}
		return compozyworkspace.Workspace{}, fmt.Errorf("store: scan workspace: %w", queryErr)
	}
	addDirs, err := decodeWorkspaceDirs(row.AddDirs)
	if err != nil {
		return compozyworkspace.Workspace{}, err
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return compozyworkspace.Workspace{}, err
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return compozyworkspace.Workspace{}, err
	}
	return compozyworkspace.Workspace{
		ID: row.ID, RootDir: row.RootDir, AdditionalDirs: addDirs, Name: row.Name,
		DefaultAgent: strings.TrimSpace(
			row.DefaultAgent.String,
		), SandboxRef: strings.TrimSpace(row.SandboxRef),
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func nullableWorkspaceString(value string) sql.NullString {
	return store.SQLNullString(value)
}

func normalizeWorkspaceRecord(
	ws compozyworkspace.Workspace,
) (compozyworkspace.Workspace, string, error) {
	normalized := ws
	normalized.ID = strings.TrimSpace(normalized.ID)
	normalized.RootDir = strings.TrimSpace(normalized.RootDir)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.DefaultAgent = strings.TrimSpace(normalized.DefaultAgent)
	normalized.SandboxRef = strings.TrimSpace(normalized.SandboxRef)
	normalized.AdditionalDirs = compactStrings(normalized.AdditionalDirs)

	switch {
	case normalized.RootDir == "":
		return compozyworkspace.Workspace{}, "", errors.New(
			"store: workspace root directory is required",
		)
	case normalized.Name == "":
		return compozyworkspace.Workspace{}, "", errors.New("store: workspace name is required")
	}

	addDirsJSON, err := encodeWorkspaceDirs(normalized.AdditionalDirs)
	if err != nil {
		return compozyworkspace.Workspace{}, "", err
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
	workspace compozyworkspace.Workspace,
	err error,
) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "workspace deletion pending") {
		return compozyworkspace.ErrWorkspaceDeletionPending
	}
	if !isSQLiteUniqueConstraint(err) {
		return err
	}

	queries := sqlcgen.New(exec)
	byPath, pathErr := queries.GetWorkspaceByPath(ctx, workspace.RootDir)
	switch {
	case pathErr == nil && byPath.ID != workspace.ID:
		return compozyworkspace.ErrWorkspacePathTaken
	case pathErr != nil && !errors.Is(pathErr, sql.ErrNoRows):
		return errors.Join(
			err,
			fmt.Errorf("store: classify workspace path constraint: %w", pathErr),
		)
	}

	byName, nameErr := queries.GetWorkspaceByName(ctx, workspace.Name)
	switch {
	case nameErr == nil && byName.ID != workspace.ID:
		return compozyworkspace.ErrWorkspaceNameTaken
	case nameErr != nil && !errors.Is(nameErr, sql.ErrNoRows):
		return errors.Join(
			err,
			fmt.Errorf("store: classify workspace name constraint: %w", nameErr),
		)
	default:
		return err
	}
}

func mapWorkspaceDeleteConstraintError(err error) error {
	if err == nil {
		return nil
	}
	err = mapTerminalRunCommandGuardError(err)
	if errors.Is(err, taskpkg.ErrTerminalRunCommandInProgress) {
		return err
	}
	if isSQLiteForeignKeyConstraint(err) {
		return compozyworkspace.ErrWorkspaceHasSessions
	}
	return err
}
