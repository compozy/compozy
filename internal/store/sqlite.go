package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// OpenSQLiteDatabase opens a SQLite database, applies shared configuration,
// and retries once after moving aside a corrupt file.
func OpenSQLiteDatabase(
	ctx context.Context,
	path string,
	initialize func(context.Context, *sql.DB) error,
) (*sql.DB, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil, errors.New("store: database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o755); err != nil {
		return nil, fmt.Errorf("store: create database directory for %q: %w", cleanPath, err)
	}

	db, err := openSQLiteDatabaseOnce(ctx, cleanPath, initialize)
	if err == nil {
		return db, nil
	}
	if !ShouldRecoverSQLite(err) {
		return nil, err
	}
	if _, statErr := os.Stat(cleanPath); statErr != nil {
		return nil, err
	}
	if _, recoverErr := recoverSQLiteDatabase(cleanPath); recoverErr != nil {
		return nil, errors.Join(err, fmt.Errorf("store: recover sqlite database %q: %w", cleanPath, recoverErr))
	}

	db, reopenErr := openSQLiteDatabaseOnce(ctx, cleanPath, initialize)
	if reopenErr != nil {
		return nil, errors.Join(
			err,
			fmt.Errorf("store: reopen sqlite database %q after recovery: %w", cleanPath, reopenErr),
		)
	}
	return db, nil
}

func openSQLiteDatabaseOnce(
	ctx context.Context,
	path string,
	initialize func(context.Context, *sql.DB) error,
) (*sql.DB, error) {
	db, err := sql.Open(sqliteDriverName, sqliteDSN(path))
	if err != nil {
		return nil, fmt.Errorf("store: open sqlite database %q: %w", path, err)
	}

	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)

	if err := db.PingContext(ctx); err != nil {
		closeQuietly(db)
		return nil, fmt.Errorf("store: ping sqlite database %q: %w", path, err)
	}
	if err := configureSQLite(ctx, db); err != nil {
		closeQuietly(db)
		return nil, fmt.Errorf("store: configure sqlite database %q: %w", path, err)
	}
	if initialize != nil {
		if err := initialize(ctx, db); err != nil {
			closeQuietly(db)
			return nil, fmt.Errorf("store: initialize sqlite database %q: %w", path, err)
		}
	}

	return db, nil
}

func sqliteDSN(path string) string {
	u := url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(path),
	}
	query := u.Query()
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", defaultBusyTimeoutMS))
	query.Add("_pragma", "foreign_keys(ON)")
	query.Add("_pragma", "journal_mode(WAL)")
	query.Add("_pragma", "synchronous(NORMAL)")
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

// ShouldRecoverSQLite reports whether the open error indicates recoverable corruption.
func ShouldRecoverSQLite(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"not a database",
		"database disk image is malformed",
		"malformed database schema",
		"malformed",
		"file is encrypted or is not a database",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}

	return false
}

// Checkpoint truncates the WAL for an open SQLite database.
func Checkpoint(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	if _, err := db.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("store: checkpoint sqlite wal: %w", err)
	}
	return nil
}

func recoverSQLiteDatabase(path string) (string, error) {
	corruptPath := fmt.Sprintf("%s.corrupt.%s", path, time.Now().UTC().Format("20060102T150405.000000000Z0700"))
	if err := os.Rename(path, corruptPath); err != nil {
		return "", err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := renameSQLiteCompanion(path+suffix, corruptPath+suffix); err != nil {
			return "", err
		}
	}
	return corruptPath, nil
}

func renameSQLiteCompanion(source string, target string) error {
	if err := os.Rename(source, target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

func closeQuietly(db *sql.DB) {
	if db != nil {
		_ = db.Close()
	}
}
