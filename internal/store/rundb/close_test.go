package rundb

import (
	"context"
	"database/sql"
	"testing"
)

type runDBCloseContextKey string

func TestRunDBCloseContextDelegatesToSQLiteCloser(t *testing.T) {
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
}
