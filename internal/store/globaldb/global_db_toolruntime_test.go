package globaldb

import (
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/toolruntime"
)

func TestGlobalDBToolProcessRecords(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and update tool process records", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		startedAt := time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC)
		record := toolruntime.ProcessRecord{
			ID:             "proc-globaldb",
			Source:         toolruntime.ProcessSourceACPTerminal,
			Owner:          toolruntime.ProcessOwner{SessionID: "sess-1", TurnID: "turn-1", TerminalID: "term-1"},
			PID:            1234,
			ProcessGroupID: 1234,
			Command:        "sleep",
			Args:           []string{"60"},
			Cwd:            "/workspace",
			StartedAt:      startedAt,
			StartedByPID:   4321,
			State:          toolruntime.ProcessStateRunning,
			CreatedAt:      startedAt,
			UpdatedAt:      startedAt,
		}

		if err := globalDB.UpsertProcessRecord(ctx, record); err != nil {
			t.Fatalf("UpsertProcessRecord() error = %v", err)
		}
		records, err := globalDB.ListProcessRecords(ctx, toolruntime.ProcessQuery{
			Scope: toolruntime.InterruptScope{SessionID: "sess-1", TurnID: "turn-1"},
		})
		if err != nil {
			t.Fatalf("ListProcessRecords() error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("records = %d, want %d", got, want)
		}
		if records[0].Owner.TerminalID != "term-1" || records[0].Args[0] != "60" {
			t.Fatalf("record = %#v, want persisted owner and args", records[0])
		}

		exitCode := 130
		completedAt := startedAt.Add(time.Minute)
		if err := globalDB.UpdateProcessRecordState(ctx, toolruntime.ProcessStateUpdate{
			ID:          record.ID,
			State:       toolruntime.ProcessStateInterrupted,
			ExitCode:    &exitCode,
			Error:       "canceled",
			UpdatedAt:   completedAt,
			CompletedAt: &completedAt,
		}); err != nil {
			t.Fatalf("UpdateProcessRecordState() error = %v", err)
		}
		records, err = globalDB.ListProcessRecords(ctx, toolruntime.ProcessQuery{
			IDs:    []string{record.ID},
			States: []toolruntime.ProcessState{toolruntime.ProcessStateInterrupted},
		})
		if err != nil {
			t.Fatalf("ListProcessRecords(updated) error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("updated records = %d, want %d", got, want)
		}
		updated := records[0]
		if updated.ExitCode == nil || *updated.ExitCode != exitCode {
			t.Fatalf("updated.ExitCode = %v, want %d", updated.ExitCode, exitCode)
		}
		if updated.CompletedAt == nil || !updated.CompletedAt.Equal(completedAt) {
			t.Fatalf("updated.CompletedAt = %v, want %s", updated.CompletedAt, completedAt)
		}
	})

	t.Run("Should reject state updates for missing records", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		err := globalDB.UpdateProcessRecordState(ctx, toolruntime.ProcessStateUpdate{
			ID:        "proc-missing",
			State:     toolruntime.ProcessStateCompleted,
			UpdatedAt: time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, toolruntime.ErrProcessNotFound) {
			t.Fatalf("UpdateProcessRecordState(missing) error = %v, want ErrProcessNotFound", err)
		}
	})
}
