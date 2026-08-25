package journal

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store/workspacedb"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

// Record appends one immutable command row.
func (s *Service) Record(ctx context.Context, workspaceID string, row terminalpkg.CommandRow) error {
	if err := validateCommandRow(workspaceID, row); err != nil {
		return err
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("terminal journal: open workspace store: %w", err)
	}
	err = db.InsertTerminalCommand(ctx, workspacedb.TerminalCommandWrite{
		ID: row.ID, TerminalID: terminalIDString(row.TerminalID), ProfileID: row.ProfileID,
		ActorKind: string(row.Actor.Kind), ActorID: row.Actor.ID,
		SessionID: optionalString(row.Actor.SessionID), RunID: optionalString(row.Actor.RunID),
		Command: scrubCommand(row.Command), ArgvDigest: row.ArgvDigest, Cwd: row.Cwd,
		StartedAt: row.StartedAt.UnixMilli(), DurationMs: row.DurationMs,
		ExitCode: row.ExitCode, ExitSignal: row.ExitSignal,
		ExitCause: row.ExitCause, DetectedBy: row.DetectedBy, Approval: row.Approval,
		OutputBytes: row.OutputBytes, Truncated: row.Truncated, RecordingID: row.RecordingID,
	})
	if err != nil {
		return fmt.Errorf("terminal journal: append command %q: %w", row.ID, err)
	}
	return nil
}

// LinkRecording persists a recording and links commands captured in its window.
func (s *Service) LinkRecording(
	ctx context.Context,
	workspaceID string,
	terminalID terminalpkg.ID,
	recording terminalpkg.RecordingRef,
) error {
	if strings.TrimSpace(recording.ID) == "" || strings.TrimSpace(recording.ProfileID) == "" {
		return errors.New("terminal journal: recording id and profile id are required")
	}
	db, err := s.databases.Open(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("terminal journal: open workspace store: %w", err)
	}
	stoppedAt := timeMillis(recording.StoppedAt)
	err = db.LinkTerminalRecording(ctx, workspacedb.TerminalRecordingWrite{
		ID: recording.ID, TerminalID: string(terminalID), ProfileID: recording.ProfileID,
		Digest: recording.Digest, Path: recording.Path, StartedAt: recording.StartedAt.UnixMilli(),
		StoppedAt: stoppedAt, Bytes: recording.Bytes,
		ExpiresAt: recording.ExpiresAt.UnixMilli(),
	})
	if err != nil {
		return fmt.Errorf("terminal journal: insert recording %q: %w", recording.ID, err)
	}
	return nil
}

func validateCommandRow(workspaceID string, row terminalpkg.CommandRow) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(row.ID) == "" ||
		strings.TrimSpace(row.ProfileID) == "" || strings.TrimSpace(row.Actor.ID) == "" {
		return errors.New("terminal journal: workspace, command, profile, and actor ids are required")
	}
	if row.StartedAt.IsZero() || strings.TrimSpace(row.Command) == "" || strings.TrimSpace(row.Cwd) == "" {
		return errors.New("terminal journal: command, cwd, and started_at are required")
	}
	return nil
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func terminalIDString(value *terminalpkg.ID) *string {
	if value == nil {
		return nil
	}
	result := string(*value)
	return &result
}

func timeMillis(value *time.Time) *int64 {
	if value == nil {
		return nil
	}
	result := value.UnixMilli()
	return &result
}
