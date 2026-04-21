package rundb

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type runDBCloseContextKey string

func TestRunDBCloseContext(t *testing.T) {
	t.Run("Should delegate to the SQLite closer with the caller context", func(t *testing.T) {
		originalCloser := closeRunSQLiteDatabase
		t.Cleanup(func() {
			closeRunSQLiteDatabase = originalCloser
		})

		var (
			gotCtx context.Context
			gotDB  *sql.DB
		)
		closeRunSQLiteDatabase = func(ctx context.Context, db *sql.DB) error {
			gotCtx = ctx
			gotDB = db
			return nil
		}

		db := &sql.DB{}
		runDB := &RunDB{db: db}
		ctx := context.WithValue(context.Background(), runDBCloseContextKey("scope"), "run-close")
		if err := runDB.CloseContext(ctx); err != nil {
			t.Fatalf("CloseContext() error = %v", err)
		}
		if runDB.db != nil {
			t.Fatal("expected CloseContext to clear the cached sql.DB handle")
		}
		if gotCtx == nil || gotCtx.Value(runDBCloseContextKey("scope")) != "run-close" {
			t.Fatalf("close context = %#v, want propagated caller context value", gotCtx)
		}
		if gotDB != db {
			t.Fatalf("close db = %#v, want original handle %#v", gotDB, db)
		}
	})

	t.Run("Should preserve the handle for retry when SQLite close fails", func(t *testing.T) {
		originalCloser := closeRunSQLiteDatabase
		t.Cleanup(func() {
			closeRunSQLiteDatabase = originalCloser
		})

		expectedErr := errors.New("close failed")
		var attempts int
		db := &sql.DB{}
		runDB := &RunDB{db: db}
		closeRunSQLiteDatabase = func(context.Context, *sql.DB) error {
			attempts++
			if attempts == 1 {
				return expectedErr
			}
			return nil
		}

		if err := runDB.CloseContext(context.Background()); !errors.Is(err, expectedErr) {
			t.Fatalf("CloseContext(first) error = %v, want %v", err, expectedErr)
		}
		if runDB.db != db {
			t.Fatal("expected failed close to preserve the cached sql.DB handle")
		}
		if err := runDB.CloseContext(context.Background()); err != nil {
			t.Fatalf("CloseContext(second) error = %v", err)
		}
		if runDB.db != nil {
			t.Fatal("expected successful retry to clear the cached sql.DB handle")
		}
		if attempts != 2 {
			t.Fatalf("close attempts = %d, want 2", attempts)
		}
	})
}
