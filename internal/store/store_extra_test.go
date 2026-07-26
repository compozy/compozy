package store

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

func TestStoreSQLHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	where, args := BuildClauses(
		StringClause("type", " agent_message "),
		StringClause("ignored", "   "),
		TimeClause("timestamp", ">=", now),
		TimeClause("timestamp", ">=", time.Time{}),
		Int64Clause("sequence", ">", 3),
		Int64Clause("sequence", ">", 0),
	)

	if got, want := NormalizeSessionType("   "), defaultSessionType; got != want {
		t.Fatalf("NormalizeSessionType(blank) = %q, want %q", got, want)
	}
	if got := NormalizeSessionType(" dream "); got != "dream" {
		t.Fatalf("NormalizeSessionType(value) = %q, want dream", got)
	}

	if got, want := len(where), 3; got != want {
		t.Fatalf("len(where) = %d, want %d (%v)", got, want, where)
	}
	if got, want := len(args), 3; got != want {
		t.Fatalf("len(args) = %d, want %d (%v)", got, want, args)
	}

	query := AppendWhere("SELECT * FROM events", where)
	if !strings.Contains(query, "WHERE type = ? AND timestamp >= ? AND sequence > ?") {
		t.Fatalf("AppendWhere() = %q", query)
	}

	invalidWhere, invalidArgs := BuildClauses(
		StringClause("bad-name", "value"),
		TimeClause("timestamp", "DROP TABLE", now),
		Int64Clause("sequence", "DROP TABLE", 3),
	)
	if got, want := invalidWhere, []string{"1 = 0", "1 = 0", "1 = 0"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("invalid where = %#v, want %#v", got, want)
	}
	if got, want := len(invalidArgs), 0; got != want {
		t.Fatalf("len(invalidArgs) = %d, want %d", got, want)
	}

	limitedQuery, limitedArgs := AppendLimit(query, args, 5)
	if !strings.HasSuffix(limitedQuery, " LIMIT ?") {
		t.Fatalf("AppendLimit() query = %q", limitedQuery)
	}
	if got, want := limitedArgs[len(limitedArgs)-1], any(5); got != want {
		t.Fatalf("AppendLimit() last arg = %#v, want %#v", got, want)
	}
	if got, want := AppendWhere("SELECT 1", nil), "SELECT 1"; got != want {
		t.Fatalf("AppendWhere(no clauses) = %q, want %q", got, want)
	}
	if gotQuery, gotArgs := AppendLimit("SELECT 1", nil, 0); gotQuery != "SELECT 1" || gotArgs != nil {
		t.Fatalf("AppendLimit(no limit) = (%q, %#v), want (%q, nil)", gotQuery, gotArgs, "SELECT 1")
	}
	if got := SQLNullString("   "); got.Valid {
		t.Fatalf("SQLNullString(blank) = %#v, want invalid", got)
	}
	if got := SQLNullString(" value "); !got.Valid || got.String != "value" {
		t.Fatalf("SQLNullString(value) = %#v, want trimmed valid value", got)
	}
	if got := SQLNullStringPointer(nil); got.Valid {
		t.Fatalf("SQLNullStringPointer(nil) = %#v, want invalid", got)
	}
}

func TestStoreSQLiteHelpers(t *testing.T) {
	t.Parallel()

	dsn := sqliteDSN("/tmp/example.db")
	parsedDSN, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("url.Parse(sqliteDSN()) error = %v", err)
	}
	if got, want := parsedDSN.Scheme, "file"; got != want {
		t.Fatalf("sqliteDSN() scheme = %q, want %q", got, want)
	}
	if got, want := parsedDSN.Path, "/tmp/example.db"; got != want {
		t.Fatalf("sqliteDSN() path = %q, want %q", got, want)
	}
	pragmas := parsedDSN.Query()["_pragma"]
	for _, want := range []string{
		"busy_timeout(5000)",
		"foreign_keys(ON)",
	} {
		var found bool
		if slices.Contains(pragmas, want) {
			found = true
		}
		if !found {
			t.Fatalf("sqliteDSN() pragmas = %#v, want %q", pragmas, want)
		}
	}
	if got, want := NullableInt64(nil), any(nil); got != want {
		t.Fatalf("NullableInt64(nil) = %#v, want nil", got)
	}
	value := int64(7)
	if got := NullableInt64(&value); got != int64(7) {
		t.Fatalf("NullableInt64(valid) = %#v, want 7", got)
	}
	if got, want := NullableFloat64(nil), any(nil); got != want {
		t.Fatalf("NullableFloat64(nil) = %#v, want nil", got)
	}
	floatValue := 1.5
	if got := NullableFloat64(&floatValue); got != 1.5 {
		t.Fatalf("NullableFloat64(valid) = %#v, want 1.5", got)
	}
	if got := NullString(sql.NullString{String: "   ", Valid: true}); got != nil {
		t.Fatalf("NullString(blank) = %#v, want nil", got)
	}
	if _, err := NormalizeSQLiteIdentifier("bad-name"); err == nil {
		t.Fatal("NormalizeSQLiteIdentifier(invalid) error = nil, want non-nil")
	}
	if got, err := NormalizeSQLiteIdentifier("valid_name_2"); err != nil || got != "valid_name_2" {
		t.Fatalf("NormalizeSQLiteIdentifier(valid) = (%q, %v), want (valid_name_2, nil)", got, err)
	}

	dbPath := filepath.Join(t.TempDir(), "shared.db")
	db, err := openSQLiteDatabaseOnce(testutil.Context(t), dbPath, func(ctx context.Context, db *sql.DB) error {
		return executeTestSchema(ctx, db, []string{
			`CREATE TABLE IF NOT EXISTS sample (id TEXT PRIMARY KEY, value TEXT NOT NULL);`,
			`INSERT INTO sample (id, value) VALUES ('row-1', 'alpha');`,
		})
	})
	if err != nil {
		t.Fatalf("openSQLiteDatabaseOnce() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("db.Close() error = %v", closeErr)
		}
	})

	if err := configureSQLite(testutil.Context(t), db); err != nil {
		t.Fatalf("configureSQLite() error = %v", err)
	}
	if err := Checkpoint(testutil.Context(t), db); err != nil {
		t.Fatalf("Checkpoint() error = %v", err)
	}

	if mode, err := querySingleString(
		testutil.Context(t),
		db,
		"PRAGMA journal_mode",
	); err != nil ||
		!strings.EqualFold(mode, "wal") {
		t.Fatalf("querySingleString(journal_mode) = (%q, %v), want wal", mode, err)
	}
	verifyConnPragmas := func(t *testing.T, conn *sql.Conn) {
		t.Helper()

		var busyTimeout int
		if err := conn.QueryRowContext(testutil.Context(t), "PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
			t.Fatalf("QueryRowContext(PRAGMA busy_timeout) error = %v", err)
		}
		if got, want := busyTimeout, defaultBusyTimeoutMS; got != want {
			t.Fatalf("PRAGMA busy_timeout = %d, want %d", got, want)
		}

		var foreignKeys int
		if err := conn.QueryRowContext(testutil.Context(t), "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			t.Fatalf("QueryRowContext(PRAGMA foreign_keys) error = %v", err)
		}
		if got, want := foreignKeys, 1; got != want {
			t.Fatalf("PRAGMA foreign_keys = %d, want %d", got, want)
		}
	}

	firstConn, err := db.Conn(testutil.Context(t))
	if err != nil {
		t.Fatalf("db.Conn(first) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := firstConn.Close(); closeErr != nil {
			t.Errorf("firstConn.Close() error = %v", closeErr)
		}
	})
	verifyConnPragmas(t, firstConn)

	secondConn, err := db.Conn(testutil.Context(t))
	if err != nil {
		t.Fatalf("db.Conn(second) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := secondConn.Close(); closeErr != nil {
			t.Errorf("secondConn.Close() error = %v", closeErr)
		}
	})
	verifyConnPragmas(t, secondConn)

	var count int
	if err := db.QueryRowContext(testutil.Context(t), `SELECT COUNT(*) FROM sample`).Scan(&count); err != nil {
		t.Fatalf("QueryRowContext(count) error = %v", err)
	}
	if count != 1 {
		t.Fatalf("sample row count = %d, want 1", count)
	}
}

func TestSQLiteDSNAppendsExtraPragmas(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve shared pragmas and append caller pragmas", func(t *testing.T) {
		t.Parallel()

		dsn := sqliteDSN("/tmp/example.db", "wal_autocheckpoint(0)", "  ")
		parsedDSN, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("url.Parse(sqliteDSN()) error = %v", err)
		}
		pragmas := parsedDSN.Query()["_pragma"]
		for _, want := range []string{
			"busy_timeout(5000)",
			"foreign_keys(ON)",
			"wal_autocheckpoint(0)",
		} {
			if !slices.Contains(pragmas, want) {
				t.Fatalf("sqliteDSN() pragmas = %#v, want %q", pragmas, want)
			}
		}
		if slices.Contains(pragmas, "") {
			t.Fatalf("sqliteDSN() pragmas = %#v, want no blank pragma", pragmas)
		}
	})
}
