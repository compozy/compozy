package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

func prepareOperatorHomeMigrationContext(
	ctx context.Context,
	db *sql.DB,
	operatorHomeDir string,
) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS phase0_operator_home_context (
			home_workspace_id TEXT PRIMARY KEY
		)`); err != nil {
		return fmt.Errorf("globaldb: create operator home migration context: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM phase0_operator_home_context`); err != nil {
		return fmt.Errorf("globaldb: clear operator home migration context: %w", err)
	}

	trimmedHome := strings.TrimSpace(operatorHomeDir)
	if trimmedHome == "" {
		return nil
	}
	canonicalHome, err := canonicalMigrationPath(trimmedHome)
	if err != nil {
		return fmt.Errorf("globaldb: resolve migration operator home %q: %w", trimmedHome, err)
	}

	var workspacesTable string
	err = db.QueryRowContext(ctx, `
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'workspaces'
	`).Scan(&workspacesTable)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("globaldb: inspect workspaces table for operator home: %w", err)
	}

	homeWorkspaceID, err := findOperatorHomeWorkspaceID(ctx, db, canonicalHome)
	if err != nil {
		return err
	}
	if homeWorkspaceID == "" {
		return nil
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO phase0_operator_home_context (home_workspace_id) VALUES (?)
	`, homeWorkspaceID); err != nil {
		return fmt.Errorf("globaldb: record operator home workspace migration context: %w", err)
	}
	return nil
}

func findOperatorHomeWorkspaceID(
	ctx context.Context,
	db *sql.DB,
	canonicalHome string,
) (homeWorkspaceID string, resultErr error) {
	rows, err := db.QueryContext(ctx, `SELECT id, root_dir FROM workspaces`)
	if err != nil {
		return "", fmt.Errorf("globaldb: list workspaces for operator home migration: %w", err)
	}
	defer func() {
		resultErr = errors.Join(resultErr, rows.Close())
	}()

	for rows.Next() {
		var workspaceID string
		var rootDir string
		if err := rows.Scan(&workspaceID, &rootDir); err != nil {
			return "", fmt.Errorf("globaldb: scan workspace for operator home migration: %w", err)
		}
		canonicalRoot, err := canonicalMigrationPath(rootDir)
		if err != nil {
			return "", fmt.Errorf("globaldb: resolve workspace root %q for operator home migration: %w", rootDir, err)
		}
		if canonicalRoot != canonicalHome {
			continue
		}
		if homeWorkspaceID != "" {
			return "", errors.New("globaldb: multiple workspaces resolve to the operator home")
		}
		homeWorkspaceID = workspaceID
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("globaldb: iterate workspaces for operator home migration: %w", err)
	}
	return homeWorkspaceID, nil
}

func canonicalMigrationPath(path string) (string, error) {
	absolutePath, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err == nil {
		return filepath.Clean(resolvedPath), nil
	}
	resolvedParent, parentErr := filepath.EvalSymlinks(filepath.Dir(absolutePath))
	if parentErr != nil {
		return "", fmt.Errorf("resolve parent symlinks: %w", parentErr)
	}
	return filepath.Join(resolvedParent, filepath.Base(absolutePath)), nil
}
