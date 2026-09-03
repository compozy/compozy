package workspacedb

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/compozy/compozy/internal/store/workspacedb/sqlcgen"
)

// InsertTerminalArtifact retains metadata for one command spill artifact.
func (d *DB) InsertTerminalArtifact(ctx context.Context, artifact TerminalArtifactWrite) error {
	err := sqlcgen.New(d.db).InsertTerminalArtifact(ctx, sqlcgen.InsertTerminalArtifactParams{
		ID: artifact.ID, TerminalID: nullableString(artifact.TerminalID), CommandID: artifact.CommandID,
		ProfileID: artifact.ProfileID, Digest: artifact.Digest, Path: artifact.Path,
		Bytes: artifact.Bytes, ExpiresAt: artifact.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("store: insert terminal artifact %q: %w", artifact.ID, err)
	}
	return nil
}

// TerminalArtifact returns retained metadata by id.
func (d *DB) TerminalArtifact(ctx context.Context, id string) (TerminalArtifactRecord, error) {
	row, err := sqlcgen.New(d.db).GetTerminalArtifact(ctx, id)
	if err != nil {
		return TerminalArtifactRecord{}, fmt.Errorf("store: get terminal artifact %q: %w", id, err)
	}
	return TerminalArtifactRecord{
		ID: row.ID, TerminalID: stringPointer(row.TerminalID), CommandID: row.CommandID,
		ProfileID: row.ProfileID, Digest: row.Digest, Path: row.Path,
		Bytes: row.Bytes, ExpiresAt: row.ExpiresAt,
	}, nil
}

// TerminalRecording returns retained recording metadata by id.
func (d *DB) TerminalRecording(ctx context.Context, id string) (TerminalRecordingRecord, error) {
	row, err := sqlcgen.New(d.db).GetTerminalRecording(ctx, id)
	if err != nil {
		return TerminalRecordingRecord{}, fmt.Errorf("store: get terminal recording %q: %w", id, err)
	}
	return TerminalRecordingRecord{
		ID: row.ID, TerminalID: row.TerminalID, ProfileID: row.ProfileID,
		Digest: row.Digest, Path: row.Path, StartedAt: row.StartedAt,
		StoppedAt: int64Pointer(row.StoppedAt), Bytes: row.Bytes, ExpiresAt: row.ExpiresAt,
	}, nil
}

// SweepExpiredTerminalFiles removes expired rows after their unshared files are removed.
func (d *DB) SweepExpiredTerminalFiles(
	ctx context.Context,
	expiresAt int64,
	remove func(string) error,
) error {
	queries := sqlcgen.New(d.db)
	artifacts, err := queries.ListExpiredTerminalArtifacts(ctx, expiresAt)
	if err != nil {
		return fmt.Errorf("store: list expired terminal artifacts: %w", err)
	}
	if err := queries.ClearExpiredTerminalRecordingLinks(ctx, expiresAt); err != nil {
		return fmt.Errorf("store: clear expired terminal recording links: %w", err)
	}
	recordings, err := queries.ListExpiredTerminalRecordings(ctx, expiresAt)
	if err != nil {
		return fmt.Errorf("store: list expired terminal recordings: %w", err)
	}
	for _, artifact := range artifacts {
		refs, countErr := queries.CountOtherTerminalArtifactPathRefs(
			ctx,
			sqlcgen.CountOtherTerminalArtifactPathRefsParams{Path: artifact.Path, ID: artifact.ID},
		)
		if countErr != nil {
			return fmt.Errorf("store: count terminal artifact path references %q: %w", artifact.ID, countErr)
		}
		if refs == 0 {
			if err := remove(artifact.Path); err != nil {
				return err
			}
		}
		if err := queries.DeleteTerminalArtifact(ctx, artifact.ID); err != nil {
			return fmt.Errorf("store: delete expired terminal artifact %q: %w", artifact.ID, err)
		}
	}
	for _, recording := range recordings {
		refs, countErr := queries.CountOtherTerminalRecordingPathRefs(
			ctx,
			sqlcgen.CountOtherTerminalRecordingPathRefsParams{Path: recording.Path, ID: recording.ID},
		)
		if countErr != nil {
			return fmt.Errorf("store: count terminal recording path references %q: %w", recording.ID, countErr)
		}
		if refs == 0 {
			if err := remove(recording.Path); err != nil {
				return err
			}
		}
		if err := queries.DeleteTerminalRecording(ctx, recording.ID); err != nil {
			return fmt.Errorf("store: delete expired terminal recording %q: %w", recording.ID, err)
		}
	}
	return nil
}

func stringPointer(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	result := value.String
	return &result
}

func int64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
