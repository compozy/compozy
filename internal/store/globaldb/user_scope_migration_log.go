package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
)

func logUserScopeMigration(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(
		ctx,
		"SELECT source_workspace_id,disposition FROM user_scope_migrations ORDER BY source_workspace_id",
	)
	if err != nil {
		return err
	}
	count := 0
	for rows.Next() {
		var id, disposition string
		if err := rows.Scan(&id, &disposition); err != nil {
			return errors.Join(err, rows.Close())
		}
		count++
		slog.InfoContext(ctx, "user_scope.migration.sql_completed", "workspace_id", id, "disposition", disposition)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	aliases, err := db.QueryContext(
		ctx,
		`SELECT a.old_name,w.name,w.id FROM workspace_name_aliases a JOIN workspaces w ON w.id=a.workspace_id ORDER BY a.old_name`,
	)
	if err != nil {
		return err
	}
	for aliases.Next() {
		var oldName, newName, id string
		if err := aliases.Scan(&oldName, &newName, &id); err != nil {
			return errors.Join(err, aliases.Close())
		}
		slog.InfoContext(
			ctx,
			"user_scope.migration.workspace_renamed",
			"workspace_id",
			id,
			"old_name",
			oldName,
			"name",
			newName,
		)
	}
	if err := errors.Join(aliases.Err(), aliases.Close()); err != nil {
		return err
	}
	if count == 0 {
		return nil
	}
	for _, table := range userScopeMigrationInventory {
		var userRows, archived int
		// The static inventory is also checked against the schema by the migration fixture suite.
		query := "SELECT count(*) FROM " + quoteMigrationIdentifier(table) +
			" WHERE workspace_id='' OR workspace_id IS NULL"
		if err := db.QueryRowContext(ctx, query).
			Scan(&userRows); err != nil {
			return err
		}
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM user_scope_migration_archive WHERE table_name=?", table).
			Scan(&archived); err != nil {
			return err
		}
		slog.InfoContext(
			ctx,
			"user_scope.migration.table",
			"table",
			table,
			"user_rows",
			userRows,
			"archived_rows",
			archived,
		)
	}
	return nil
}
