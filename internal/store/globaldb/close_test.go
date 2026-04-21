package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

type globalDBCloseContextKey string

func TestGlobalDBCloseContext(t *testing.T) {
	t.Run("Should delegate to the SQLite closer with the caller context", func(t *testing.T) {
		originalCloser := closeGlobalSQLiteDatabase
		t.Cleanup(func() {
			closeGlobalSQLiteDatabase = originalCloser
		})

		var (
			gotCtx context.Context
			gotDB  *sql.DB
		)
		closeGlobalSQLiteDatabase = func(ctx context.Context, db *sql.DB) error {
			gotCtx = ctx
			gotDB = db
			return nil
		}

		db := &sql.DB{}
		global := &GlobalDB{db: db}
		ctx := context.WithValue(context.Background(), globalDBCloseContextKey("scope"), "catalog-close")
		if err := global.CloseContext(ctx); err != nil {
			t.Fatalf("CloseContext() error = %v", err)
		}
		if !global.closed.Load() {
			t.Fatal("expected CloseContext to mark the database as closed")
		}
		if gotCtx == nil || gotCtx.Value(globalDBCloseContextKey("scope")) != "catalog-close" {
			t.Fatalf("close context = %#v, want propagated caller context value", gotCtx)
		}
		if gotDB != db {
			t.Fatalf("close db = %#v, want original handle %#v", gotDB, db)
		}
	})

	t.Run("Should allow retry after a failed SQLite close", func(t *testing.T) {
		originalCloser := closeGlobalSQLiteDatabase
		t.Cleanup(func() {
			closeGlobalSQLiteDatabase = originalCloser
		})

		expectedErr := errors.New("close failed")
		var attempts int
		closeGlobalSQLiteDatabase = func(context.Context, *sql.DB) error {
			attempts++
			if attempts == 1 {
				return expectedErr
			}
			return nil
		}

		global := &GlobalDB{db: &sql.DB{}}
		if err := global.CloseContext(context.Background()); !errors.Is(err, expectedErr) {
			t.Fatalf("CloseContext(first) error = %v, want %v", err, expectedErr)
		}
		if global.closed.Load() {
			t.Fatal("expected failed close to leave the database retryable")
		}
		if err := global.CloseContext(context.Background()); err != nil {
			t.Fatalf("CloseContext(second) error = %v", err)
		}
		if !global.closed.Load() {
			t.Fatal("expected successful retry to mark the database as closed")
		}
		if attempts != 2 {
			t.Fatalf("close attempts = %d, want 2", attempts)
		}
	})
}
