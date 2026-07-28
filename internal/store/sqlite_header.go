package store

import (
	"errors"
	"fmt"
	"io"
	"os"
)

const sqliteDatabaseHeader = "SQLite format 3\x00"

func validateSQLiteDatabaseHeader(path string, size int64) error {
	if size == 0 {
		return nil
	}
	if size < int64(len(sqliteDatabaseHeader)) {
		return &SQLiteCorruptionError{
			Path:  path,
			Cause: fmt.Errorf("store: sqlite database header is truncated: got %d bytes", size),
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("store: open sqlite database %q for header validation: %w", path, err)
	}

	header := make([]byte, len(sqliteDatabaseHeader))
	if _, err := io.ReadFull(file, header); err != nil {
		primaryErr := fmt.Errorf("store: read sqlite database header %q: %w", path, err)
		return closeSQLiteAfterError(file, primaryErr, fmt.Sprintf("database header %q", path))
	}
	if string(header) != sqliteDatabaseHeader {
		primaryErr := &SQLiteCorruptionError{
			Path:  path,
			Cause: errors.New("store: sqlite database header is invalid"),
		}
		return closeSQLiteAfterError(file, primaryErr, fmt.Sprintf("database header %q", path))
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("store: close sqlite database header %q: %w", path, err)
	}
	return nil
}
