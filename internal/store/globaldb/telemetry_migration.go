package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

const (
	telemetryExportVersion = 190
	telemetryMovedVersion  = 191
	// Each batch costs two FULL-synchronous commits; larger batches bound fsyncs
	// on large histories while keeping every acknowledged batch resumable.
	telemetryExportBatch = 1024
	// telemetryExportBatchBytes bounds the payload one batch holds in memory; a
	// batch always takes at least one row so an oversized row still advances.
	telemetryExportBatchBytes = 8 << 20

	// MigrationStageTelemetryRelocation names the #1241 telemetry relocation progress stage.
	MigrationStageTelemetryRelocation = "telemetry_relocation"
)

// MigrationProgress reports rows completed by one data-volume-dependent migration stage.
// Total is zero until the stage knows its size. Done marks the stage, including its
// trailing schema steps, as finished.
type MigrationProgress struct {
	Stage     string
	Completed int64
	Total     int64
	Done      bool
}

// MigrationProgressFunc observes migration progress. It runs synchronously on the
// migration goroutine between batches, so implementations must return promptly.
type MigrationProgressFunc func(MigrationProgress)

// applyTelemetryRelocation is the migration owner for the two SQLite files.
// Source rows are removed only after a FULL-synchronous target commit and exact
// value verification. An interruption at either commit resumes idempotently.
func applyTelemetryRelocation(
	ctx context.Context,
	db *sql.DB,
	path string,
	version int64,
	progress MigrationProgressFunc,
) (retErr error) {
	if version == 0 {
		if _, err := bootstrapGlobalTelemetry(ctx, db); err != nil {
			return err
		}
		status, err := store.Status(ctx, db, MigrationStream())
		if err != nil {
			return err
		}
		version = status.Version
	}
	if version >= telemetryMovedVersion {
		pending, err := pendingTelemetryInitialization(ctx, db)
		if err != nil {
			return err
		}
		if pending {
			if err := completeTelemetryInitialization(ctx, db, path); err != nil {
				return err
			}
		}
	}
	targetPath := telemetrydb.Path(path)
	if version >= telemetryExportVersion {
		if err := requireTelemetryDestination(ctx, db, targetPath); err != nil {
			return err
		}
	}
	if version >= telemetryMovedVersion {
		return store.Apply(ctx, db, MigrationStream())
	}
	target, err := telemetrydb.Open(ctx, targetPath)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, target.Close(context.WithoutCancel(ctx))) }()
	stepper, err := store.NewMigrationStepper(ctx, db, MigrationStream())
	if err != nil {
		return err
	}
	if version < telemetryExportVersion {
		if _, err := stepper.UpTo(ctx, telemetryExportVersion-1); err != nil {
			return err
		}
		if err := bindTelemetryDestination(ctx, db, target.DB()); err != nil {
			return err
		}
	}
	if _, err := stepper.UpTo(ctx, telemetryExportVersion); err != nil {
		return err
	}
	if err := exportTelemetry(ctx, db, target.DB(), progress); err != nil {
		return err
	}
	// The SQL boundary refuses to drop a nonempty source, including when a
	// caller tries to apply the stream without this cross-file coordinator.
	if err := store.Apply(ctx, db, MigrationStream()); err != nil {
		return err
	}
	if progress != nil {
		progress(MigrationProgress{Stage: MigrationStageTelemetryRelocation, Done: true})
	}
	return nil
}

// The destination exists before the source freeze commits. Once export can have
// acknowledged a batch, neither a missing file nor an empty replacement is safe.
func requireTelemetryDestination(ctx context.Context, source *sql.DB, path string) (err error) {
	db, err := store.OpenSQLiteReadPool(ctx, path)
	if err != nil {
		return fmt.Errorf("store: relocated telemetry database is required: %w", err)
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	status, err := store.Status(ctx, db, telemetrydb.MigrationStream())
	if err != nil {
		return fmt.Errorf("store: inspect relocated telemetry database: %w", err)
	}
	if status.Version == 0 {
		return errors.New("store: relocated telemetry database is uninitialized")
	}
	return verifyTelemetryIdentity(ctx, source, db)
}

func exportTelemetry(ctx context.Context, source, target *sql.DB, progress MigrationProgressFunc) error {
	report := func(completed, total int64) {
		if progress != nil {
			progress(MigrationProgress{Stage: MigrationStageTelemetryRelocation, Completed: completed, Total: total})
		}
	}
	// Announce the stage before counting: counting a large frozen table is itself slow.
	report(0, 0)
	// The export table is frozen at telemetryExportVersion, so its remaining rows bound the work.
	var total int64
	if err := source.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summary_export").Scan(&total); err != nil {
		return fmt.Errorf("store: count telemetry export: %w", err)
	}
	slog.InfoContext(ctx, "store: relocating telemetry", "rows", total)
	report(0, total)
	var completed int64
	for {
		columns, rows, err := readTelemetryExportBatch(ctx, source, telemetryExportBatchBytes)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			if err := preserveTelemetrySequence(ctx, source, target); err != nil {
				return err
			}
			report(total, total)
			slog.InfoContext(ctx, "store: relocated telemetry", "rows", completed)
			return nil
		}
		if err := commitTelemetryExportBatch(ctx, target, columns, rows); err != nil {
			return err
		}
		if err := verifyTelemetryIdentity(ctx, source, target); err != nil {
			return err
		}
		sequences := make([]any, len(rows))
		for i, row := range rows {
			sequences[i] = row[0]
		}
		if err := store.ExecuteWriteOperation(
			ctx,
			source,
			"acknowledge telemetry export",
			func(ctx context.Context, tx *store.WriteTx) error {
				// dynamic-sql: the bounded batch determines placeholder count; source sequence values are bound.
				_, err := tx.ExecContext(ctx, "DELETE FROM event_summary_export WHERE seq IN ("+
					strings.TrimSuffix(strings.Repeat("?,", len(sequences)), ",")+")", sequences...)
				return err
			},
		); err != nil {
			return err
		}
		completed += int64(len(rows))
		report(min(completed, total), total)
	}
}

// Retention may have removed the highest acknowledged cursor, including every
// row. Copy the sequence high-water mark before dropping the frozen source.
func preserveTelemetrySequence(ctx context.Context, source, target *sql.DB) error {
	var sequence int64
	if err := source.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(seq), 0) FROM sqlite_sequence WHERE name = 'event_summary_export'",
	).Scan(&sequence); err != nil {
		return err
	}
	return store.ExecuteWriteOperation(
		ctx,
		target,
		"preserve telemetry cursor",
		func(ctx context.Context, tx *store.WriteTx) error {
			result, err := tx.ExecContext(ctx,
				"UPDATE sqlite_sequence SET seq = MAX(seq, ?) WHERE name = 'event_summaries'", sequence)
			if err != nil {
				return err
			}
			changed, err := result.RowsAffected()
			if err != nil || changed != 0 {
				return err
			}
			_, err = tx.ExecContext(
				ctx,
				"INSERT INTO sqlite_sequence(name, seq) VALUES ('event_summaries', ?)",
				sequence,
			)
			return err
		},
	)
}

// readTelemetryExportBatch reads up to telemetryExportBatch rows in seq order, stopping early at the byte budget.
func readTelemetryExportBatch(
	ctx context.Context,
	db *sql.DB,
	byteBudget int,
) (columns []string, values [][]any, err error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM event_summary_export ORDER BY seq LIMIT ?", telemetryExportBatch)
	if err != nil {
		return nil, nil, fmt.Errorf("store: read telemetry export: %w", err)
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	columns, err = rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	if len(columns) == 0 || columns[0] != "seq" {
		return nil, nil, errors.New("store: telemetry export lacks its sequence identity")
	}
	retained := 0
	for retained < byteBudget && rows.Next() {
		row := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for i := range row {
			destinations[i] = &row[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, nil, err
		}
		values = append(values, row)
		retained += telemetryRowBytes(row)
	}
	return columns, values, rows.Err()
}

func telemetryRowBytes(row []any) int {
	size := 0
	for _, value := range row {
		switch typed := value.(type) {
		case []byte:
			size += len(typed)
		case string:
			size += len(typed)
		default:
			size += 8
		}
	}
	return size
}

func commitTelemetryExportBatch(ctx context.Context, target *sql.DB, columns []string, rows [][]any) error {
	quoted := make([]string, len(columns))
	comparisons := make([]string, len(columns))
	for i, column := range columns {
		quoted[i] = `"` + strings.ReplaceAll(column, `"`, `""`) + `"`
		comparisons[i] = quoted[i] + " IS ?"
	}
	// dynamic-sql: migration-owned column names preserve every historical value
	// independent of physical column order; all values remain parameters.
	insert := "INSERT INTO event_summaries (" + strings.Join(quoted, ",") + ") VALUES (" +
		strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",") + ") ON CONFLICT(id) DO NOTHING"
	verify := "SELECT COUNT(*) FROM event_summaries WHERE " + strings.Join(comparisons, " AND ")
	return store.ExecuteWriteOperation(
		ctx,
		target,
		"copy telemetry export",
		func(ctx context.Context, tx *store.WriteTx) error {
			for _, row := range rows {
				if _, err := tx.ExecContext(ctx, insert, row...); err != nil {
					return fmt.Errorf("store: copy telemetry export row: %w", err)
				}
				var identical int
				if err := tx.QueryRowContext(ctx, verify, row...).Scan(&identical); err != nil {
					return err
				}
				if identical != 1 {
					return errors.New("store: telemetry export identity collides with different stored values")
				}
			}
			return nil
		},
	)
}
