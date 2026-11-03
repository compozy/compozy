// Package telemetrydb owns append-only observability independently of the core database.
package telemetrydb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb/sqlcgen"
)

const retentionBatchSize = 256

// Store acknowledges telemetry only after its own WAL commits.
type Store struct {
	db         *sql.DB
	readDB     *sql.DB
	background *store.BackgroundWriter
	closed     atomic.Bool
}

func Path(globalPath string) string { return globalPath + ".telemetry.db" }

func Open(ctx context.Context, path string) (*Store, error) {
	db, err := store.OpenSQLiteDatabaseWithPragmas(
		ctx,
		path,
		[]string{"synchronous(FULL)"},
		func(ctx context.Context, db *sql.DB) error {
			return store.Apply(ctx, db, MigrationStream())
		},
	)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	// Shared configuration sets NORMAL on its initial connection. Both that
	// connection and future driver connections must sync before export removes source rows.
	if _, err := db.ExecContext(ctx, "PRAGMA synchronous = FULL"); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		// #nosec G703 -- restrict the operator-selected SQLite family opened above.
		if err := os.Chmod(path+suffix, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(err, db.Close())
		}
	}
	readDB, err := store.OpenSQLiteReadPool(ctx, path)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &Store{db: db, readDB: readDB, background: store.NewBackgroundWriter(db)}, nil
}

// DB is reserved for the owning cross-file migration coordinator.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) Status(ctx context.Context) (store.StreamStatus, error) {
	return store.Status(ctx, s.readDB, MigrationStream())
}

func (s *Store) Write(ctx context.Context, summaries []store.EventSummary, validate func(context.Context) error) error {
	return s.background.Submit(ctx, "write event summaries", func(ctx context.Context, tx *store.WriteTx) error {
		if err := validate(ctx); err != nil {
			return err
		}
		queries := sqlcgen.New(tx)
		for _, summary := range summaries {
			if err := insertEventSummary(ctx, queries, summary); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Sweep(ctx context.Context, cutoff time.Time) (int64, error) {
	value := store.FormatTimestamp(cutoff)
	return s.deleteBatches(ctx, "sweep event summaries", func(ctx context.Context, q *sqlcgen.Queries) (int64, error) {
		return q.DeleteEventSummariesBefore(ctx, sqlcgen.DeleteEventSummariesBeforeParams{
			Cutoff: value, BatchSize: retentionBatchSize,
		})
	})
}

func (s *Store) DeleteScope(ctx context.Context, kind, id string) (int64, error) {
	return s.deleteBatches(
		ctx,
		"delete scoped event summaries",
		func(ctx context.Context, q *sqlcgen.Queries) (int64, error) {
			switch kind {
			case "workspace":
				return q.DeleteWorkspaceEventSummaries(ctx, sqlcgen.DeleteWorkspaceEventSummariesParams{
					WorkspaceID: id, BatchSize: retentionBatchSize,
				})
			case "profile":
				return q.DeleteProfileEventSummaries(ctx, sqlcgen.DeleteProfileEventSummariesParams{
					ProfileID: id, BatchSize: retentionBatchSize,
				})
			default:
				return 0, fmt.Errorf("store: unsupported telemetry cleanup scope %q", kind)
			}
		},
	)
}

func (s *Store) deleteBatches(
	ctx context.Context, operation string, run func(context.Context, *sqlcgen.Queries) (int64, error),
) (int64, error) {
	var total int64
	for {
		var deleted int64
		err := s.background.Submit(ctx, operation, func(ctx context.Context, tx *store.WriteTx) error {
			var err error
			deleted, err = run(ctx, sqlcgen.New(tx))
			return err
		})
		if err != nil {
			return total, err
		}
		total += deleted
		if deleted < retentionBatchSize {
			return total, nil
		}
	}
}

func (s *Store) Close(ctx context.Context) error {
	if s == nil || s.closed.Swap(true) {
		return nil
	}
	backgroundErr := s.background.Close(ctx)
	readErr := s.readDB.Close()
	checkpointErr := store.CheckpointPassive(ctx, s.db)
	return errors.Join(backgroundErr, readErr, checkpointErr, s.db.Close())
}
