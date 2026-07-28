package store

import (
	"errors"
	"fmt"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrSQLiteCorrupt = errors.New("store: sqlite database is corrupt")

// SQLiteCorruptionError reports corruption without attempting to mutate or
// replace the database family. Recovery belongs to a stopped composition root
// that can preserve every related database and companion file together.
type SQLiteCorruptionError struct {
	Path  string
	Cause error
}

func (e *SQLiteCorruptionError) Error() string {
	if e == nil {
		return ErrSQLiteCorrupt.Error()
	}
	return fmt.Sprintf("%s %q: %v", ErrSQLiteCorrupt, e.Path, e.Cause)
}

func (e *SQLiteCorruptionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *SQLiteCorruptionError) Is(target error) bool {
	return target == ErrSQLiteCorrupt
}

func classifySQLiteOpenError(path string, err error) error {
	if !isSQLiteCorruption(err) {
		return err
	}
	return &SQLiteCorruptionError{Path: path, Cause: err}
}

func isSQLiteCorruption(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() & sqlitePrimaryResultCodeMask {
	case sqlite3.SQLITE_CORRUPT, sqlite3.SQLITE_NOTADB:
		return true
	default:
		return false
	}
}
