package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSQLiteDSN(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		path        string
		wantPrefix  string
		wantNoSlash bool
	}{
		{
			name:       "absolute unix path keeps triple-slash form",
			path:       "/home/user/.compozy/db.sqlite",
			wantPrefix: "file:///home/user/.compozy/db.sqlite",
		},
		{
			// Relative paths must not be converted to absolute file:/// URIs;
			// the leading slash would point to the filesystem root instead.
			name:        "relative path is not prefixed with slash",
			path:        "relative.db",
			wantNoSlash: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dsn := sqliteDSN(tt.path)
			if tt.wantPrefix != "" && !strings.HasPrefix(dsn, tt.wantPrefix) {
				t.Errorf("sqliteDSN(%q) = %q, want prefix %q", tt.path, dsn, tt.wantPrefix)
			}
			if tt.wantNoSlash && strings.HasPrefix(dsn, "file:///") {
				t.Errorf("sqliteDSN(%q) = %q, must not produce absolute file:/// URI for relative path", tt.path, dsn)
			}
		})
	}
}

func TestShouldRecoverSQLite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error returns false", err: nil, want: false},
		{name: "not a database error triggers recovery", err: errors.New("file is not a database"), want: true},
		{name: "malformed error triggers recovery", err: errors.New("database disk image is malformed"), want: true},
		{name: "malformed schema error triggers recovery", err: errors.New("malformed database schema"), want: true},
		{
			name: "encrypted file error triggers recovery",
			err:  errors.New("file is encrypted or is not a database"),
			want: true,
		},
		{name: "unrelated error does not trigger recovery", err: errors.New("connection refused"), want: false},
		{name: "partial keyword match is case-insensitive", err: errors.New("MALFORMED"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ShouldRecoverSQLite(tt.err)
			if got != tt.want {
				t.Errorf("ShouldRecoverSQLite(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestOpenSQLiteDatabase(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty path", func(t *testing.T) {
		t.Parallel()
		_, err := OpenSQLiteDatabase(context.Background(), "", nil)
		if err == nil {
			t.Fatal("OpenSQLiteDatabase(\"\") error = nil, want error")
		}
	})

	t.Run("opens and initializes a fresh database", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "test.db")
		ctx := context.Background()

		var initCalled bool
		initialize := func(ctx context.Context, db *sql.DB) error {
			initCalled = true
			return EnsureSchema(ctx, db, []string{
				"CREATE TABLE IF NOT EXISTS items (id TEXT PRIMARY KEY)",
			})
		}

		db, err := OpenSQLiteDatabase(ctx, dbPath, initialize)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase() error = %v", err)
		}
		if !initCalled {
			t.Error("initialize callback was not called")
		}

		if closeErr := CloseSQLiteDatabase(ctx, db); closeErr != nil {
			t.Errorf("CloseSQLiteDatabase() error = %v", closeErr)
		}
	})

	t.Run("opens without initialize callback", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "noninit.db")
		ctx := context.Background()

		db, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase() with nil initialize error = %v", err)
		}
		if closeErr := CloseSQLiteDatabase(ctx, db); closeErr != nil {
			t.Errorf("CloseSQLiteDatabase() error = %v", closeErr)
		}
	})

	t.Run("reopening an existing database succeeds", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "reopen.db")
		ctx := context.Background()

		db, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("first open error = %v", err)
		}
		if closeErr := CloseSQLiteDatabase(ctx, db); closeErr != nil {
			t.Fatalf("first close error = %v", closeErr)
		}

		db2, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("second open error = %v", err)
		}
		if closeErr := CloseSQLiteDatabase(ctx, db2); closeErr != nil {
			t.Errorf("second close error = %v", closeErr)
		}
	})

	t.Run("recovers from corrupt database file and returns fresh database", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "corrupt.db")
		ctx := context.Background()

		// Write garbage to trigger "file is not a database" from the SQLite driver.
		if err := os.WriteFile(dbPath, []byte("this is not a sqlite database file"), 0o644); err != nil {
			t.Fatalf("write corrupt file: %v", err)
		}

		db, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase() recovery error = %v", err)
		}
		defer closeQuietly(db)

		entries, readErr := os.ReadDir(dir)
		if readErr != nil {
			t.Fatalf("ReadDir: %v", readErr)
		}
		var foundCorrupt bool
		for _, e := range entries {
			if strings.Contains(e.Name(), ".corrupt") {
				foundCorrupt = true
				break
			}
		}
		if !foundCorrupt {
			t.Error("no .corrupt backup file created after recovery")
		}
	})

	t.Run("returns error when initialize callback fails", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "init_fail.db")
		ctx := context.Background()

		initErr := errors.New("initialization failed")
		_, err := OpenSQLiteDatabase(ctx, dbPath, func(_ context.Context, _ *sql.DB) error {
			return initErr
		})
		if !errors.Is(err, initErr) {
			t.Errorf("OpenSQLiteDatabase() error = %v, want wrapping %v", err, initErr)
		}
	})
}

func TestCheckpoint(t *testing.T) {
	t.Parallel()

	t.Run("nil db is a no-op", func(t *testing.T) {
		t.Parallel()
		if err := Checkpoint(context.Background(), nil); err != nil {
			t.Errorf("Checkpoint(nil) error = %v, want nil", err)
		}
	})

	t.Run("checkpoints a real WAL database", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "wal.db")
		ctx := context.Background()

		db, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase() error = %v", err)
		}
		defer func() { _ = db.Close() }()

		if err := Checkpoint(ctx, db); err != nil {
			t.Errorf("Checkpoint() error = %v", err)
		}
	})
}

func TestCloseSQLiteDatabaseNilCases(t *testing.T) {
	t.Parallel()

	t.Run("nil db returns nil without panicking", func(t *testing.T) {
		t.Parallel()
		if err := CloseSQLiteDatabase(context.Background(), nil); err != nil {
			t.Errorf("CloseSQLiteDatabase(nil db) error = %v, want nil", err)
		}
	})

	t.Run("nil context returns error", func(t *testing.T) {
		t.Parallel()
		var nilCtx context.Context
		err := CloseSQLiteDatabase(nilCtx, &sql.DB{})
		if err == nil {
			t.Fatal("CloseSQLiteDatabase(nil ctx) error = nil, want error")
		}
	})
}

func TestCloseQuietly(t *testing.T) {
	t.Parallel()

	t.Run("nil db does not panic", func(t *testing.T) {
		t.Parallel()
		closeQuietly(nil)
	})

	t.Run("non-nil db gets closed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "close_quietly.db")
		db, err := sql.Open(sqliteDriverName, sqliteDSN(dbPath))
		if err != nil {
			t.Fatalf("sql.Open() error = %v", err)
		}
		closeQuietly(db)
	})
}

func TestRecoverSQLiteDatabase(t *testing.T) {
	t.Parallel()

	t.Run("renames main file and existing companions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "db.sqlite")

		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.WriteFile(dbPath+suffix, []byte("content"), 0o644); err != nil {
				t.Fatalf("write %s: %v", suffix, err)
			}
		}

		corruptPath, err := recoverSQLiteDatabase(dbPath)
		if err != nil {
			t.Fatalf("recoverSQLiteDatabase() error = %v", err)
		}
		if corruptPath == "" {
			t.Fatal("recoverSQLiteDatabase() returned empty path")
		}
		if _, statErr := os.Stat(dbPath); statErr == nil {
			t.Error("original file still exists after recovery")
		}
	})

	t.Run("succeeds when companion files are absent", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "solo.sqlite")

		if err := os.WriteFile(dbPath, []byte("content"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		if _, err := recoverSQLiteDatabase(dbPath); err != nil {
			t.Fatalf("recoverSQLiteDatabase() without companions error = %v", err)
		}
	})
}

func TestEnsureSchema(t *testing.T) {
	t.Parallel()

	t.Run("returns error for invalid SQL statement", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "schema_err.db")
		ctx := context.Background()

		db, err := OpenSQLiteDatabase(ctx, dbPath, nil)
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase() error = %v", err)
		}
		defer closeQuietly(db)

		if err := EnsureSchema(ctx, db, []string{"NOT VALID SQL;;;"}); err == nil {
			t.Fatal("EnsureSchema() with invalid SQL error = nil, want error")
		}
	})
}

func TestCloseSQLiteDatabase(t *testing.T) {
	t.Run("Should checkpoint before closing the SQLite handle", func(t *testing.T) {
		originalCheckpoint := checkpointSQLiteWAL
		originalClose := closeSQLiteHandle
		t.Cleanup(func() {
			checkpointSQLiteWAL = originalCheckpoint
			closeSQLiteHandle = originalClose
		})

		var steps []string
		checkpointSQLiteWAL = func(ctx context.Context, db *sql.DB) error {
			if ctx == nil {
				t.Fatal("checkpoint context = nil, want non-nil")
			}
			if db == nil {
				t.Fatal("checkpoint db = nil, want non-nil")
			}
			steps = append(steps, "checkpoint")
			return nil
		}
		closeSQLiteHandle = func(db *sql.DB) error {
			if db == nil {
				t.Fatal("close db = nil, want non-nil")
			}
			steps = append(steps, "close")
			return nil
		}

		if err := CloseSQLiteDatabase(context.Background(), &sql.DB{}); err != nil {
			t.Fatalf("CloseSQLiteDatabase() error = %v", err)
		}
		if !reflect.DeepEqual(steps, []string{"checkpoint", "close"}) {
			t.Fatalf("close steps = %#v, want checkpoint then close", steps)
		}
	})

	t.Run("Should still close the SQLite handle when checkpointing fails", func(t *testing.T) {
		originalCheckpoint := checkpointSQLiteWAL
		originalClose := closeSQLiteHandle
		t.Cleanup(func() {
			checkpointSQLiteWAL = originalCheckpoint
			closeSQLiteHandle = originalClose
		})

		checkpointErr := errors.New("checkpoint failed")
		closeCalled := false
		checkpointSQLiteWAL = func(context.Context, *sql.DB) error {
			return checkpointErr
		}
		closeSQLiteHandle = func(*sql.DB) error {
			closeCalled = true
			return nil
		}

		err := CloseSQLiteDatabase(context.Background(), &sql.DB{})
		if !errors.Is(err, checkpointErr) {
			t.Fatalf("CloseSQLiteDatabase() error = %v, want %v", err, checkpointErr)
		}
		if !closeCalled {
			t.Fatal("expected close handler to run even when checkpoint fails")
		}
	})

	t.Run("Should return close error when checkpoint succeeds but close fails", func(t *testing.T) {
		originalCheckpoint := checkpointSQLiteWAL
		originalClose := closeSQLiteHandle
		t.Cleanup(func() {
			checkpointSQLiteWAL = originalCheckpoint
			closeSQLiteHandle = originalClose
		})

		closeErr := errors.New("close failed")
		checkpointSQLiteWAL = func(context.Context, *sql.DB) error { return nil }
		closeSQLiteHandle = func(*sql.DB) error { return closeErr }

		err := CloseSQLiteDatabase(context.Background(), &sql.DB{})
		if !errors.Is(err, closeErr) {
			t.Errorf("CloseSQLiteDatabase() error = %v, want %v", err, closeErr)
		}
	})

	t.Run("Should join errors when both checkpoint and close fail", func(t *testing.T) {
		originalCheckpoint := checkpointSQLiteWAL
		originalClose := closeSQLiteHandle
		t.Cleanup(func() {
			checkpointSQLiteWAL = originalCheckpoint
			closeSQLiteHandle = originalClose
		})

		checkpointErr := errors.New("checkpoint failed")
		closeErr := errors.New("close failed")
		checkpointSQLiteWAL = func(context.Context, *sql.DB) error { return checkpointErr }
		closeSQLiteHandle = func(*sql.DB) error { return closeErr }

		err := CloseSQLiteDatabase(context.Background(), &sql.DB{})
		if !errors.Is(err, checkpointErr) {
			t.Errorf("CloseSQLiteDatabase() error does not wrap checkpoint error: %v", err)
		}
		if !errors.Is(err, closeErr) {
			t.Errorf("CloseSQLiteDatabase() error does not wrap close error: %v", err)
		}
	})
}
