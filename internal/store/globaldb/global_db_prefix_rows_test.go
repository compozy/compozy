package globaldb

import (
	"database/sql"
	"slices"
	"strings"
	"testing"
)

// insertPrefixRow seeds one row into a database migrated only to a historical prefix.
// Repository writers target the current schema, so prefix fixtures insert only the
// columns that exist at that version; absent columns keep their migration defaults.
func insertPrefixRow(t *testing.T, db *sql.DB, table string, values map[string]any) {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		t.Fatalf("pragma_table_info(%s) error = %v", table, err)
	}
	var present []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table_info(%s) error = %v", table, err)
		}
		present = append(present, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close table_info(%s) error = %v", table, err)
	}
	columns := make([]string, 0, len(values))
	for column := range values {
		if slices.Contains(present, column) {
			columns = append(columns, column)
		}
	}
	slices.Sort(columns)
	args := make([]any, 0, len(columns))
	for _, column := range columns {
		args = append(args, values[column])
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(columns)), ", ")
	query := "INSERT INTO " + table + " (" + strings.Join(columns, ", ") + ") VALUES (" + placeholders + ")"
	if _, err := db.ExecContext(t.Context(), query, args...); err != nil {
		t.Fatalf("insert prefix %s row error = %v", table, err)
	}
}
