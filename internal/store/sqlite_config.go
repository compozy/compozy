package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	// DefaultSQLiteBusyTimeoutMS is the shared SQLite busy timeout in milliseconds.
	DefaultSQLiteBusyTimeoutMS = defaultBusyTimeoutMS

	sqliteDriverName     = "sqlite"
	sqliteFileScheme     = "file"
	defaultBusyTimeoutMS = 5000
	defaultMaxOpenConns  = 8
	defaultMaxIdleConns  = 8
)

func sqliteDSN(path string, extraPragmas ...string) string {
	return sqliteDSNWithPragmas(path, true, extraPragmas...)
}

func sqliteInitializationDSN(path string) string {
	return sqliteDSNWithPragmas(path, false)
}

func sqliteImmutableDSN(path string) string {
	u := url.URL{
		Scheme: sqliteFileScheme,
		Path:   filepath.ToSlash(path),
	}
	query := u.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	u.RawQuery = query.Encode()
	return u.String()
}

func sqliteDSNWithPragmas(path string, runtimePragmas bool, extraPragmas ...string) string {
	u := url.URL{
		Scheme: sqliteFileScheme,
		Path:   filepath.ToSlash(path),
	}
	query := u.Query()
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", defaultBusyTimeoutMS))
	query.Add("_pragma", "foreign_keys(ON)")
	if runtimePragmas {
		query.Add("_pragma", "journal_mode(WAL)")
		query.Add("_pragma", "synchronous(NORMAL)")
	}
	for _, pragma := range extraPragmas {
		if trimmed := strings.TrimSpace(pragma); trimmed != "" {
			query.Add("_pragma", trimmed)
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func configureSQLite(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, fmt.Sprintf("PRAGMA busy_timeout = %d", defaultBusyTimeoutMS)); err != nil {
		return err
	}

	mode, err := querySingleString(ctx, db, "PRAGMA journal_mode = WAL")
	if err != nil {
		return err
	}
	if !strings.EqualFold(mode, "wal") {
		return fmt.Errorf("store: sqlite journal_mode = %q, want wal", mode)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = NORMAL"); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	return nil
}

func querySingleString(ctx context.Context, db *sql.DB, stmt string) (string, error) {
	var value string
	if err := db.QueryRowContext(ctx, stmt).Scan(&value); err != nil {
		return "", err
	}
	return value, nil
}
