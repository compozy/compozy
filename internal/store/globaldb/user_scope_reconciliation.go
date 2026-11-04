package globaldb

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
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
	rows, err := g.readDB.QueryContext(
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
	var statement string
	switch kind {
	case "journal":
		statement = "UPDATE user_scope_migrations SET journal_state='verified' WHERE source_workspace_id=?"
	case "layout":
		statement = "UPDATE user_scope_migrations SET layout_state='verified' WHERE source_workspace_id=?"
	default:
		return errors.New("unknown User migration store")
	}
	return store.ExecuteWriteOperation(
		ctx,
		g.db,
		"verify User scope store",
		func(ctx context.Context, tx *store.WriteTx) error {
			_, err := tx.ExecContext(ctx, statement, id)
			return err
		},
	)
}

// CompleteUserScopeMigration refuses the cascade path while any table still references the source.
func (g *GlobalDB) CompleteUserScopeMigration(ctx context.Context, id string) error {
	// The schema is fixed after Open. Discover its foreign-key inventory without
	// reserving the writer, then recheck every reference under the delete fence.
	references, err := workspaceReferenceColumns(ctx, g.readDB)
	if err != nil {
		return err
	}
	return store.ExecuteWriteOperation(
		ctx,
		g.db,
		"complete User scope migration",
		func(ctx context.Context, tx *store.WriteTx) error {
			var ready int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM user_scope_migrations WHERE source_workspace_id=? AND disposition='migrate' AND sql_done_at IS NOT NULL AND journal_state IN ('verified','skipped') AND layout_state IN ('verified','skipped')`, id).
				Scan(&ready); err != nil {
				return err
			}
			if ready != 1 {
				return errors.New("migration stores for the User scope are not verified")
			}
			for table, columns := range references {
				for _, column := range columns {
					var present bool
					query := "SELECT EXISTS(SELECT 1 FROM " + quoteMigrationIdentifier(
						table,
					) + " WHERE " + quoteMigrationIdentifier(
						column,
					) + "=?)"
					if err := tx.QueryRowContext(ctx, query, id).Scan(&present); err != nil {
						return err
					}
					if present {
						return fmt.Errorf(
							"migration registration retained for the User scope: table %s column %s has references",
							table,
							column,
						)
					}
				}
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM workspaces WHERE id=?", id); err != nil {
				return err
			}
			_, err := tx.ExecContext(
				ctx,
				"UPDATE user_scope_migrations SET registration_removed_at=datetime('now') WHERE source_workspace_id=?",
				id,
			)
			return err
		},
	)
}

func quoteMigrationIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func workspaceReferenceColumns(ctx context.Context, tx globalSQLExecutor) (map[string][]string, error) {
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
