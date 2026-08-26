package workspacedb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/workspacedb/sqlcgen"
)

// InsertTerminalCommand appends one immutable terminal command.
func (d *DB) InsertTerminalCommand(ctx context.Context, row TerminalCommandWrite) error {
	err := sqlcgen.New(d.db).InsertTerminalCommand(ctx, sqlcgen.InsertTerminalCommandParams{
		ID: row.ID, TerminalID: nullableString(row.TerminalID), ProfileID: row.ProfileID,
		ActorKind: row.ActorKind, ActorID: row.ActorID,
		SessionID: nullableString(row.SessionID), RunID: nullableString(row.RunID),
		Command: row.Command, ArgvDigest: nullableString(row.ArgvDigest), Cwd: row.Cwd,
		StartedAt: row.StartedAt, DurationMs: nullableInt64(row.DurationMs),
		ExitCode: nullableInt(row.ExitCode), ExitSignal: nullableString(row.ExitSignal),
		ExitCause: row.ExitCause, DetectedBy: row.DetectedBy, Approval: row.Approval,
		OutputBytes: row.OutputBytes, Truncated: boolInteger(row.Truncated),
		RecordingID: nullableString(row.RecordingID),
	})
	if err != nil {
		return fmt.Errorf("store: append terminal command %q: %w", row.ID, err)
	}
	return nil
}

// LinkTerminalRecording inserts recording metadata and links covered commands atomically.
func (d *DB) LinkTerminalRecording(ctx context.Context, recording TerminalRecordingWrite) error {
	err := store.ExecuteWrite(ctx, d.db, func(ctx context.Context, tx *store.WriteTx) error {
		queries := sqlcgen.New(tx)
		if err := queries.InsertTerminalRecording(ctx, sqlcgen.InsertTerminalRecordingParams{
			ID: recording.ID, TerminalID: recording.TerminalID, ProfileID: recording.ProfileID,
			Digest: recording.Digest, Path: recording.Path, StartedAt: recording.StartedAt,
			StoppedAt: nullableInt64(recording.StoppedAt), Bytes: recording.Bytes,
			ExpiresAt: recording.ExpiresAt,
		}); err != nil {
			return fmt.Errorf("insert recording: %w", err)
		}
		err := queries.UpdateTerminalCommandRecording(ctx, sqlcgen.UpdateTerminalCommandRecordingParams{
			RecordingID: sql.NullString{String: recording.ID, Valid: true},
			TerminalID:  sql.NullString{String: recording.TerminalID, Valid: true},
			ProfileID:   recording.ProfileID, RecordingStartedAt: recording.StartedAt,
			RecordingStoppedAt: nullableInt64(recording.StoppedAt),
		})
		if err != nil {
			return fmt.Errorf("link commands: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: link terminal recording %q: %w", recording.ID, err)
	}
	return nil
}

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func nullableInt(value *int) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*value), Valid: true}
}

func boolInteger(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
