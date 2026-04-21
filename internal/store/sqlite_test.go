package store

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestCloseSQLiteDatabaseCheckpointsBeforeClose(t *testing.T) {
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
}

func TestCloseSQLiteDatabaseStillClosesWhenCheckpointFails(t *testing.T) {
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
}
