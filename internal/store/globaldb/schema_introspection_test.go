package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type schemaQueryExecutor interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func tableExists(ctx context.Context, exec schemaQueryExecutor, table string) (bool, error) {
	var count int
	if err := exec.QueryRowContext(
		ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`,
		strings.TrimSpace(table),
	).Scan(&count); err != nil {
		return false, fmt.Errorf("query table %q existence: %w", table, err)
	}
	return count > 0, nil
}

func tableColumns(
	ctx context.Context,
	exec schemaQueryExecutor,
	table string,
) (columns map[string]struct{}, err error) {
	rows, err := exec.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, fmt.Errorf("inspect table %q: %w", table, err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			closeErr = fmt.Errorf("close table %q column rows: %w", table, closeErr)
			err = errors.Join(err, closeErr)
		}
	}()

	columns = make(map[string]struct{})
	for rows.Next() {
		var (
			position     int
			name         string
			dataType     string
			notNull      int
			defaultValue sql.NullString
			primaryKey   int
		)
		if scanErr := rows.Scan(&position, &name, &dataType, &notNull, &defaultValue, &primaryKey); scanErr != nil {
			return nil, fmt.Errorf("scan table %q column: %w", table, scanErr)
		}
		columns[name] = struct{}{}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate table %q columns: %w", table, rowsErr)
	}
	return columns, nil
}

func assertTableHasColumns(t *testing.T, db *sql.DB, table string, want []string) {
	t.Helper()
	columns, err := tableColumns(context.Background(), db, table)
	if err != nil {
		t.Fatalf("tableColumns(%q) error = %v", table, err)
	}
	for _, column := range want {
		if _, ok := columns[column]; !ok {
			t.Fatalf("table %q missing column %q", table, column)
		}
	}
}
