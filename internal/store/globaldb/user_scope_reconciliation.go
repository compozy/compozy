package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
)

type UserScopeMigration struct {
	SourceWorkspaceID string
	SourceRoot        string
	Disposition       string
	ProfileIDs        []string
	JournalState      string
	LayoutState       string
}

func (g *GlobalDB) PendingUserScopeMigrations(ctx context.Context) (result []UserScopeMigration, resultErr error) {
	rows, err := g.db.QueryContext(
		ctx,
		`SELECT source_workspace_id,source_root,disposition,profile_ids,journal_state,layout_state FROM user_scope_migrations WHERE registration_removed_at IS NULL AND disposition='migrate' ORDER BY source_workspace_id`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	for rows.Next() {
		var row UserScopeMigration
		var profiles string
		if err := rows.Scan(
			&row.SourceWorkspaceID,
			&row.SourceRoot,
			&row.Disposition,
			&profiles,
			&row.JournalState,
			&row.LayoutState,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(profiles), &row.ProfileIDs); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (g *GlobalDB) MarkUserScopeStoreVerified(ctx context.Context, id, kind string) error {
	switch kind {
	case "journal":
		_, err := g.db.ExecContext(
			ctx,
			"UPDATE user_scope_migrations SET journal_state='verified' WHERE source_workspace_id=?",
			id,
		)
		return err
	case "layout":
		_, err := g.db.ExecContext(
			ctx,
			"UPDATE user_scope_migrations SET layout_state='verified' WHERE source_workspace_id=?",
			id,
		)
		return err
	default:
		return errors.New("unknown User migration store")
	}
}

// CompleteUserScopeMigration refuses the cascade path while any table still references the source.
func (g *GlobalDB) CompleteUserScopeMigration(ctx context.Context, id string) (resultErr error) {
	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
	}()
	var ready int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM user_scope_migrations WHERE source_workspace_id=? AND disposition='migrate' AND sql_done_at IS NOT NULL AND journal_state IN ('verified','skipped') AND layout_state IN ('verified','skipped')`, id).
		Scan(&ready); err != nil {
		return err
	}
	if ready != 1 {
		return errors.New("User migration stores are not verified")
	}
	references, err := workspaceReferenceColumns(ctx, tx)
	if err != nil {
		return err
	}
	for table, columns := range references {
		for _, column := range columns {
			var count int
			if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM "+quoteMigrationIdentifier(table)+" WHERE "+quoteMigrationIdentifier(column)+"=?", id).
				Scan(&count); err != nil {
				return err
			}
			if count != 0 {
				return fmt.Errorf(
					"User migration registration retained: table %s column %s has %d references",
					table,
					column,
					count,
				)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM workspaces WHERE id=?", id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(
		ctx,
		"UPDATE user_scope_migrations SET registration_removed_at=datetime('now') WHERE source_workspace_id=?",
		id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

func quoteMigrationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func workspaceReferenceColumns(ctx context.Context, tx *sql.Tx) (map[string][]string, error) {
	rows, err := tx.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		tables = append(tables, table)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, err
	}
	result := map[string][]string{}
	for _, table := range tables {
		// Compatibility aliases are metadata, not workspace-owned data.
		if table == "workspace_name_aliases" {
			continue
		}
		columns, err := tx.QueryContext(
			ctx,
			`SELECT name FROM pragma_table_info(?) WHERE name IN ('workspace_id','loop_workspace_id') UNION SELECT "from" FROM pragma_foreign_key_list(?) WHERE "table"='workspaces'`,
			table,
			table,
		)
		if err != nil {
			return nil, err
		}
		for columns.Next() {
			var column string
			if err := columns.Scan(&column); err != nil {
				return nil, errors.Join(err, columns.Close())
			}
			result[table] = append(result[table], column)
		}
		if err := errors.Join(columns.Err(), columns.Close()); err != nil {
			return nil, err
		}
	}
	return result, nil
}
