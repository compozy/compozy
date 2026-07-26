package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestOpenSQLiteDatabaseRecoveryContract(t *testing.T) {
	t.Parallel()

	t.Run("Should classify corruption without mutating database family files", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), "corrupt.db")
		family := map[string][]byte{
			dbPath:          []byte("not a sqlite database"),
			dbPath + "-wal": []byte("preserve wal companion"),
			dbPath + "-shm": []byte("preserve shm companion"),
		}
		for path, contents := range family {
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", filepath.Base(path), err)
			}
		}

		db, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if db != nil {
			if closeErr := db.Close(); closeErr != nil {
				t.Fatalf("Close(unexpected database) error = %v", closeErr)
			}
			t.Fatal("OpenSQLiteDatabase(corrupt) database is non-nil, want nil")
		}
		if !errors.Is(err, ErrSQLiteCorrupt) {
			t.Fatalf("OpenSQLiteDatabase(corrupt) error = %v, want ErrSQLiteCorrupt", err)
		}
		var corruptionErr *SQLiteCorruptionError
		if !errors.As(err, &corruptionErr) {
			t.Fatalf("OpenSQLiteDatabase(corrupt) error = %T, want *SQLiteCorruptionError", err)
		}
		if corruptionErr.Path != dbPath {
			t.Fatalf("SQLiteCorruptionError.Path = %q, want %q", corruptionErr.Path, dbPath)
		}

		for path, want := range family {
			got, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("ReadFile(%s) error = %v", filepath.Base(path), readErr)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("ReadFile(%s) = %q, want preserved %q", filepath.Base(path), got, want)
			}
		}
		matches, globErr := filepath.Glob(dbPath + ".corrupt.*")
		if globErr != nil {
			t.Fatalf("Glob(corrupt files) error = %v", globErr)
		}
		if len(matches) != 0 {
			t.Fatalf("corrupt quarantine files = %v, want none", matches)
		}
	})

	t.Run("Should not quarantine healthy database after malformed initialization error", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		dbPath := filepath.Join(t.TempDir(), "healthy.db")
		db, err := OpenSQLiteDatabase(ctx, dbPath, func(ctx context.Context, db *sql.DB) error {
			return executeTestSchema(ctx, db, []string{
				`CREATE TABLE IF NOT EXISTS sentinel (id TEXT PRIMARY KEY, value TEXT NOT NULL);`,
				`INSERT INTO sentinel (id, value) VALUES ('row-1', 'alpha');`,
			})
		})
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(seed) error = %v", err)
		}
		if err := db.Close(); err != nil {
			t.Fatalf("Close(seed) error = %v", err)
		}

		initErr := errors.New("malformed config")
		db, err = OpenSQLiteDatabase(ctx, dbPath, func(context.Context, *sql.DB) error {
			return initErr
		})
		if err == nil {
			if db != nil {
				if closeErr := db.Close(); closeErr != nil {
					t.Fatalf("Close(unexpected) error = %v", closeErr)
				}
			}
			t.Fatal("OpenSQLiteDatabase(init fail) error = nil, want initialization error")
		}
		if !errors.Is(err, initErr) {
			t.Fatalf("OpenSQLiteDatabase(init fail) error = %v, want %v", err, initErr)
		}
		if errors.Is(err, ErrSQLiteCorrupt) {
			t.Fatalf("OpenSQLiteDatabase(init fail) error = %v, want non-corruption classification", err)
		}
		matches, err := filepath.Glob(dbPath + ".corrupt.*")
		if err != nil {
			t.Fatalf("Glob(corrupt files) error = %v", err)
		}
		if len(matches) != 0 {
			t.Fatalf("corrupt quarantine files = %v, want none", matches)
		}

		reopened, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(); closeErr != nil {
				t.Errorf("Close(reopen) error = %v", closeErr)
			}
		})

		var value string
		if err := reopened.QueryRowContext(ctx, `SELECT value FROM sentinel WHERE id = 'row-1'`).
			Scan(&value); err != nil {
			t.Fatalf("QueryRowContext(sentinel) error = %v", err)
		}
		if value != "alpha" {
			t.Fatalf("sentinel value = %q, want alpha", value)
		}
	})
}

func TestCloseSQLiteAfterError(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve both the primary and cleanup failures", func(t *testing.T) {
		t.Parallel()

		primaryErr := errors.New("initialization failed")
		closeErr := errors.New("close failed")
		closer := &sqliteFailureCloser{err: closeErr}

		got := closeSQLiteAfterError(closer, primaryErr, "test database after initialization failure")
		if !errors.Is(got, primaryErr) || !errors.Is(got, closeErr) {
			t.Fatalf("closeSQLiteAfterError() error = %v, want both primary and close failures", got)
		}
		if !closer.closed {
			t.Fatal("closeSQLiteAfterError() did not close the database handle")
		}
	})
}

type sqliteFailureCloser struct {
	err    error
	closed bool
}

func (c *sqliteFailureCloser) Close() error {
	c.closed = true
	return c.err
}
