package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"

	goosedatabase "github.com/pressly/goose/v3/database"
)

// MigrationQuerier is the read surface a reconciliation uses to detect its schema change.
type MigrationQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// MigrationReconciliation completes one named, already published migration on a
// database that carries the migration's schema change without its recorded
// version. Replaying the immutable migration would fail on such a database, so
// the owning stream supplies an idempotent form of the same statements.
//
// Apply runs it only when every earlier version is recorded, Version is not, and
// Detect reports the schema change. Completion and the version record commit in
// one immediate transaction; any other database state is left to Goose.
type MigrationReconciliation struct {
	// Name identifies the reconciliation in logs and errors.
	Name string
	// Version is the migration this reconciliation completes.
	Version int64
	// Detect reports whether the migration's schema change already exists.
	Detect func(context.Context, MigrationQuerier) (bool, error)
	// Complete runs the migration's remaining statements idempotently.
	Complete func(context.Context, *WriteTx) error
}

func reconcileMigrationStream(
	ctx context.Context,
	db *sql.DB,
	stream MigrationStream,
	inspection migrationInspection,
) error {
	reconciliation := stream.Reconciliation
	if reconciliation == nil {
		return nil
	}
	if reconciliation.Detect == nil || reconciliation.Complete == nil {
		return fmt.Errorf(
			"store: migration stream %q reconciliation %q requires detect and complete steps",
			stream.Name,
			reconciliation.Name,
		)
	}
	previous := reconciliation.Version - 1
	if !slices.Contains(inspection.directory.versions, reconciliation.Version) ||
		inspection.status.Version != previous || int64(inspection.status.AppliedCount) != previous {
		return nil
	}
	present, err := reconciliation.Detect(ctx, db)
	if err != nil {
		return fmt.Errorf(
			"store: detect migration stream %q reconciliation %q: %w", stream.Name, reconciliation.Name, err,
		)
	}
	if !present {
		return nil
	}
	reconciled, err := completeMigrationReconciliation(ctx, db, stream.VersionTable, reconciliation)
	if err != nil {
		return fmt.Errorf(
			"store: reconcile migration stream %q version %d (%s): %w",
			stream.Name,
			reconciliation.Version,
			reconciliation.Name,
			err,
		)
	}
	if reconciled {
		migrationLogger(ctx).WarnContext(ctx, "store.migrations.reconciled",
			"stream", stream.Name,
			"version", reconciliation.Version,
			"reconciliation", reconciliation.Name,
			"path", inspection.path,
		)
	}
	return nil
}

func completeMigrationReconciliation(
	ctx context.Context,
	db *sql.DB,
	versionTable string,
	reconciliation *MigrationReconciliation,
) (bool, error) {
	versionStore, err := goosedatabase.NewStore(goosedatabase.DialectSQLite3, versionTable)
	if err != nil {
		return false, fmt.Errorf("create migration version store: %w", err)
	}
	reconciled := false
	writeErr := ExecuteWrite(ctx, db, func(ctx context.Context, tx *WriteTx) error {
		reconciled = false
		// A concurrent owner may have applied the migration since the inspection.
		pending, pendingErr := migrationReconciliationPending(ctx, tx, versionTable, reconciliation.Version-1)
		if pendingErr != nil || !pending {
			return pendingErr
		}
		present, detectErr := reconciliation.Detect(ctx, tx)
		if detectErr != nil || !present {
			return detectErr
		}
		if completeErr := reconciliation.Complete(ctx, tx); completeErr != nil {
			return completeErr
		}
		if insertErr := versionStore.Insert(
			ctx, tx, goosedatabase.InsertRequest{Version: reconciliation.Version},
		); insertErr != nil {
			return fmt.Errorf("record reconciled migration version %d: %w", reconciliation.Version, insertErr)
		}
		reconciled = true
		return nil
	})
	if writeErr != nil {
		return false, writeErr
	}
	return reconciled, nil
}

// migrationReconciliationPending reports whether exactly the versions through
// previous are recorded as applied, read inside the reconciliation transaction.
func migrationReconciliationPending(
	ctx context.Context,
	tx *WriteTx,
	versionTable string,
	previous int64,
) (bool, error) {
	if tx == nil {
		return false, errors.New("migration reconciliation transaction is required")
	}
	var highest, applied int64
	// dynamic-sql: the Goose version table name is stream-specific and validated by validateMigrationInputs.
	if err := tx.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT COALESCE(MAX(version_id), 0), COUNT(DISTINCT version_id) FROM %s WHERE version_id > 0 AND is_applied`,
		quoteIdentifier(versionTable),
	)).Scan(&highest, &applied); err != nil {
		return false, fmt.Errorf("read recorded migration versions: %w", err)
	}
	return highest == previous && applied == previous, nil
}
