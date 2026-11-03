package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/telemetrydb"
)

const (
	telemetryExportVersion = 187
	telemetryMovedVersion  = 188
	telemetryExportBatch   = 128
)

// applyTelemetryRelocation is the migration owner for the two SQLite files.
// Source rows are removed only after a FULL-synchronous target commit and exact
// value verification. An interruption at either commit resumes idempotently.
func applyTelemetryRelocation(ctx context.Context, db *sql.DB, path string, version int64) (retErr error) {
	targetPath := telemetrydb.Path(path)
	if version >= telemetryMovedVersion {
		if err := store.Apply(ctx, db, MigrationStream()); err != nil {
			return err
		}
		if _, err := os.Stat(targetPath); err != nil {
			return fmt.Errorf("store: relocated telemetry database is required: %w", err)
		}
		return nil
	}
	target, err := telemetrydb.Open(ctx, targetPath)
	if err != nil {
		return err
	}
	defer func() { retErr = errors.Join(retErr, target.Close(context.WithoutCancel(ctx))) }()
	if version == 0 {
		var count int64
		if err := target.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summaries").Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return errors.New("store: refusing to attach existing telemetry to a new core database")
		}
		return store.Apply(ctx, db, MigrationStream())
	}
	stepper, err := store.NewMigrationStepper(ctx, db, MigrationStream())
	if err != nil {
		return err
	}
	if _, err := stepper.UpTo(ctx, telemetryExportVersion); err != nil {
		return err
	}
	if err := exportTelemetry(ctx, db, target.DB()); err != nil {
		return err
	}
	// The SQL boundary refuses to drop a nonempty source, including when a
	// caller tries to apply the stream without this cross-file coordinator.
	return store.Apply(ctx, db, MigrationStream())
}

func exportTelemetry(ctx context.Context, source, target *sql.DB) error {
	for {
		columns, rows, err := readTelemetryExportBatch(ctx, source)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return preserveTelemetrySequence(ctx, source, target)
		}
		if err := commitTelemetryExportBatch(ctx, target, columns, rows); err != nil {
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
	return store.ExecuteWriteOperation(ctx, target, "preserve telemetry cursor", func(ctx context.Context, tx *store.WriteTx) error {
		result, err := tx.ExecContext(ctx,
			"UPDATE sqlite_sequence SET seq = MAX(seq, ?) WHERE name = 'event_summaries'", sequence)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil || changed != 0 {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO sqlite_sequence(name, seq) VALUES ('event_summaries', ?)", sequence)
		return err
	})
}

func readTelemetryExportBatch(ctx context.Context, db *sql.DB) (columns []string, values [][]any, err error) {
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
	for rows.Next() {
		row := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for i := range row {
			destinations[i] = &row[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, nil, err
		}
		values = append(values, row)
	}
	return columns, values, rows.Err()
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
