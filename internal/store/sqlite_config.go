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

// SqliteFilePath normalizes a filesystem path for use as a file:// URL Path in a SQLite DSN.
// url.URL requires an absolute path starting with "/" to produce the three-slash form
// "file:///...". On Windows, filepath.IsAbs returns true for drive-letter paths (e.g.
// "C:\..."), but filepath.ToSlash yields "C:/..." which url.URL serializes as "file://C:/..."
// — the SQLite URI parser then interprets "C:" as the network authority rather than a drive
// letter (failing with "invalid uri authority: C:"). Prepend "/" only for absolute paths
// that lack one; relative paths are left unchanged so callers that pass relative paths are
// unaffected. Exported so sibling packages (e.g. internal/store/sessiondb) build SQLite
// DSNs through the same canonical normalization instead of duplicating the logic.
func SqliteFilePath(path string) string {
	slashPath := filepath.ToSlash(path)
	if filepath.IsAbs(path) && !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}
	return slashPath
}

func sqliteReadOnlyDSN(path string) string {
	u := url.URL{
		Scheme: sqliteFileScheme,
		Path:   SqliteFilePath(path),
	}
	query := u.Query()
	query.Set("mode", "ro")
	u.RawQuery = query.Encode()
	return u.String()
}

func sqliteDSNWithPragmas(path string, runtimePragmas bool, extraPragmas ...string) string {
	u := url.URL{
		Scheme: sqliteFileScheme,
		Path:   SqliteFilePath(path),
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
