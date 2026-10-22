package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func prepareUserScopeAutomationNames(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(
		ctx,
		`CREATE TABLE IF NOT EXISTS user_scope_automation_names (table_name TEXT NOT NULL, id TEXT NOT NULL, new_name TEXT NOT NULL); DELETE FROM user_scope_automation_names;`,
	); err != nil {
		return err
	}
	// Tables are a fixed migration inventory; names never come from caller input.
	for _, table := range []string{"automation_jobs", "automation_triggers"} {
		var exists int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).
			Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		rows, err := tx.QueryContext(
			ctx,
			"SELECT id,name,scope,coalesce(workspace_id,'') FROM "+table+" ORDER BY created_at,id",
		)
		if err != nil {
			return err
		}
		type record struct{ id, name, scope, workspace string }
		var records []record
		for rows.Next() {
			var r record
			if err := rows.Scan(&r.id, &r.name, &r.scope, &r.workspace); err != nil {
				return errors.Join(err, rows.Close())
			}
			records = append(records, r)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
		taken := map[string]bool{}
		for _, r := range records {
			if r.scope == legacyGlobalName {
				taken[r.name] = true
			}
		}
		for _, r := range records {
			var candidate int
			if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM user_scope_migration_context WHERE workspace_id=?", r.workspace).
				Scan(&candidate); err != nil {
				return err
			}
			if candidate == 0 {
				continue
			}
			name := r.name
			if taken[name] {
				base := name + " (migrated)"
				name = base
				for suffix := 2; taken[name]; suffix++ {
					name = fmt.Sprintf("%s-%d", base, suffix)
				}
			}
			taken[name] = true
			if _, err := tx.ExecContext(
				ctx,
				"INSERT INTO user_scope_automation_names VALUES (?,?,?)",
				table,
				r.id,
				name,
			); err != nil {
				return err
			}
		}
	}
	return nil
}
