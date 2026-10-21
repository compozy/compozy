package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const userScopeMigrationVersion int64 = 152

func prepareUserScopeMigrationContext(ctx context.Context, db *sql.DB, homeDir, operatorHome string) (resultErr error) {
	status, err := store.Status(ctx, db, MigrationStream())
	if err != nil {
		return fmt.Errorf("inspect User migration status: %w", err)
	}
	if status.Version >= userScopeMigrationVersion {
		return nil
	}
	var exists int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='workspaces'").
		Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return nil
	}
	roots := map[string]bool{}
	for _, root := range []string{homeDir, operatorHome} {
		if strings.TrimSpace(root) == "" {
			continue
		}
		canonical, err := canonicalMigrationPath(root)
		if err != nil {
			return fmt.Errorf("resolve User migration root: %w", err)
		}
		roots[canonical] = true
	}
	rows, err := db.QueryContext(ctx, "SELECT id, name, root_dir FROM workspaces ORDER BY created_at, id")
	if err != nil {
		return err
	}
	type registration struct{ id, name, root string }
	var registrations []registration
	for rows.Next() {
		var row registration
		if err := rows.Scan(&row.id, &row.name, &row.root); err != nil {
			return errors.Join(err, rows.Close())
		}
		registrations = append(registrations, row)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	if _, err := tx.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS user_scope_migration_context (workspace_id TEXT PRIMARY KEY, source_root TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS user_scope_name_context (workspace_id TEXT PRIMARY KEY, old_name TEXT NOT NULL, new_name TEXT NOT NULL);
 DELETE FROM user_scope_migration_context; DELETE FROM user_scope_name_context;`,
	); err != nil {
		return err
	}
	taken := map[string]bool{}
	for _, r := range registrations {
		taken[r.name] = true
	}
	for _, r := range registrations {
		root, err := canonicalMigrationPath(r.root)
		if err != nil {
			return fmt.Errorf("resolve migration workspace %q: %w", r.id, err)
		}
		if roots[root] {
			if _, err := tx.ExecContext(
				ctx,
				"INSERT INTO user_scope_migration_context VALUES (?,?)",
				r.id,
				root,
			); err != nil {
				return err
			}
		}
		if r.name != "user" && r.name != "global" && !strings.HasPrefix(r.name, "@") {
			continue
		}
		base := strings.TrimLeft(r.name, "@")
		if base == "" || base == "user" || base == "global" {
			base += "-project"
		}
		name := base
		for suffix := 2; taken[name]; suffix++ {
			name = fmt.Sprintf("%s-%d", base, suffix)
		}
		taken[name] = true
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO user_scope_name_context VALUES (?,?,?)",
			r.id,
			r.name,
			name,
		); err != nil {
			return err
		}
	}
	if err := prepareUserScopeStoreProfiles(ctx, tx, homeDir); err != nil {
		return err
	}
	if err := prepareUserScopeAutomationNames(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}
