package globaldb

import (
	"context"
	"database/sql"
	"testing"
)

type globalDBCloseContextKey string

func TestGlobalDBCloseContextDelegatesToSQLiteCloser(t *testing.T) {
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
}
