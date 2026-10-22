package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/store"
)

func prepareUserScopeStoreProfiles(ctx context.Context, tx *sql.Tx, homeDir string) error {
	if _, err := tx.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS user_scope_profile_context (
			workspace_id TEXT NOT NULL, profile_id TEXT NOT NULL, PRIMARY KEY(workspace_id,profile_id));
		DELETE FROM user_scope_profile_context;`,
	); err != nil {
		return err
	}
	directories, err := os.ReadDir(filepath.Join(homeDir, "workspaces"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, directory := range directories {
		if !directory.IsDir() || !strings.HasPrefix(directory.Name(), "user-") ||
			strings.HasSuffix(directory.Name(), ".migrated") {
			continue
		}
		if err := insertUserScopeJournalProfiles(ctx, tx, homeDir, directory.Name(), ""); err != nil {
			return err
		}
	}
	ids, err := migrationContextWorkspaceIDs(ctx, tx)
	if err != nil {
		return err
	}
	// User state from the earlier home migration also participates in template normalization.
	ids = append(ids, "")
	for _, id := range ids {
		if id != "" {
			if err := insertUserScopeJournalProfiles(ctx, tx, homeDir, id, id); err != nil {
				return err
			}
		}
		profiles, err := userScopeLayoutProfiles(ctx, homeDir, id)
		if err != nil {
			return err
		}
		if err := insertUserScopeProfiles(ctx, tx, id, profiles); err != nil {
			return err
		}
	}
	return nil
}

// insertUserScopeJournalProfiles records the profiles owning one legacy journal directory.
func insertUserScopeJournalProfiles(ctx context.Context, tx *sql.Tx, homeDir, directory, workspaceID string) error {
	profiles, err := legacyJournalProfiles(
		ctx,
		filepath.Join(homeDir, "workspaces", directory, store.GlobalDatabaseName),
	)
	if err != nil {
		return err
	}
	return insertUserScopeProfiles(ctx, tx, workspaceID, profiles)
}

func insertUserScopeProfiles(ctx context.Context, tx *sql.Tx, workspaceID string, profiles []string) error {
	for _, profile := range profiles {
		if _, err := tx.ExecContext(
			ctx,
			"INSERT OR IGNORE INTO user_scope_profile_context VALUES (?,?)",
			workspaceID,
			profile,
		); err != nil {
			return err
		}
	}
	return nil
}

func migrationContextWorkspaceIDs(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT workspace_id FROM user_scope_migration_context")
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		ids = append(ids, id)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	return ids, nil
}

// userScopeLayoutProfiles is every profile with a stored window-manager layout in the partition.
func userScopeLayoutProfiles(ctx context.Context, homeDir, workspaceID string) ([]string, error) {
	partition := workspaceID
	if partition == "" {
		partition = reservedUserName
	}
	entries, err := clientstate.ReadMigrationEntries(
		ctx,
		clientstate.DatabasePath(homeDir),
		clientstate.WorkspaceID(partition),
		"window_manager",
	)
	if err != nil {
		return nil, err
	}
	var profiles []string
	for _, entry := range entries {
		profile, ok := strings.CutPrefix(entry.Key, "snapshot:")
		if ok && profile != "" && !entry.Deleted {
			profiles = append(profiles, profile)
		}
	}
	return profiles, nil
}

func legacyJournalProfiles(ctx context.Context, path string) (profiles []string, resultErr error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	for _, table := range []string{"terminal_recordings", "terminal_commands", "terminal_artifacts"} {
		var exists int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pragma_table_info(?) WHERE name='profile_id'", table).
			Scan(&exists); err != nil {
			return nil, err
		}
		if exists == 0 {
			continue
		}
		// #nosec G202 -- table comes from the fixed journal inventory above.
		rows, err := db.QueryContext(ctx, "SELECT DISTINCT profile_id FROM "+table+" WHERE profile_id<>''")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var profile string
			if err := rows.Scan(&profile); err != nil {
				return nil, errors.Join(err, rows.Close())
			}
			profiles = append(profiles, profile)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
	}
	return profiles, nil
}
