package toolruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRegistryCheckpointsProcessLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should checkpoint and complete one process lifecycle", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := NewMemoryStore()
		now := fixedClock(time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC))
		registry := NewRegistry(store, WithNow(now), WithDaemonPID(4242))

		handle, err := registry.Register(ctx, RegisterConfig{
			ID:      "proc-test",
			Source:  ProcessSourceHook,
			Owner:   ProcessOwner{SessionID: "sess-1", TurnID: "turn-1", HookName: "hook.alpha"},
			Command: "hook-runner",
			Args:    []string{"--secret-token=redacted-by-bound", "ok"},
			Cwd:     "/workspace",
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		nextOwner := ProcessOwner{SessionID: "sess-1", TurnID: "turn-2", ToolCallID: "tool-1", HookName: "hook.alpha"}
		if err := handle.Checkpoint(ctx, ProcessCheckpoint{Owner: &nextOwner, Error: "running tool"}); err != nil {
			t.Fatalf("Checkpoint() error = %v", err)
		}
		exitCode := 7
		if err := handle.Complete(ctx, ProcessCompletion{ExitCode: &exitCode}); err != nil {
			t.Fatalf("Complete() error = %v", err)
		}

		records := listAllRecords(t, store)
		if got, want := len(records), 1; got != want {
			t.Fatalf("records = %d, want %d", got, want)
		}
		record := records[0]
		if record.ID != "proc-test" || record.State != ProcessStateCompleted {
			t.Fatalf("record = %#v, want completed proc-test", record)
		}
		if record.Owner.ToolCallID != "tool-1" || record.Owner.TurnID != "turn-2" {
			t.Fatalf("record.Owner = %#v, want updated owner", record.Owner)
		}
		if record.ExitCode == nil || *record.ExitCode != exitCode {
			t.Fatalf("record.ExitCode = %v, want %d", record.ExitCode, exitCode)
		}
		if record.StartedByPID != 4242 {
			t.Fatalf("record.StartedByPID = %d, want 4242", record.StartedByPID)
		}
		if record.CompletedAt == nil {
			t.Fatal("record.CompletedAt = nil, want completion timestamp")
		}
	})
}

func TestRegistryRegisterValidatesPIDStartTime(t *testing.T) {
	t.Parallel()

	t.Run("Should reject PID backed registrations when start time cannot be observed", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := NewMemoryStore()
		registry := NewRegistry(store)
		_, err := registry.Register(ctx, RegisterConfig{
			ID:      "proc-missing-pid",
			Source:  ProcessSourceSubprocess,
			Owner:   ProcessOwner{SessionID: "sess-1"},
			PID:     999999999,
			Command: "missing",
		})
		if err == nil {
			t.Fatal("Register() error = nil, want start-time lookup failure")
		}
		if records := listAllRecords(t, store); len(records) != 0 {
			t.Fatalf("records = %#v, want none", records)
		}
	})
}

func TestRegistryCompletionPersistsBeforeRetiringActiveHandle(t *testing.T) {
	t.Parallel()

	t.Run("Should allow completion retry after transient store update failure", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := &failOnceUpdateStore{
			MemoryStore: NewMemoryStore(),
			err:         errors.New("update failed"),
		}
		registry := NewRegistry(store)
		handle, err := registry.Register(ctx, RegisterConfig{
			ID:     "proc-retry-complete",
			Source: ProcessSourceHook,
			Owner:  ProcessOwner{HookName: "hook.retry"},
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		if err := handle.Complete(ctx, ProcessCompletion{}); !errors.Is(err, store.err) {
			t.Fatalf("Complete(first) error = %v, want %v", err, store.err)
		}
		records := listAllRecords(t, store.MemoryStore)
		if got, want := records[0].State, ProcessStateRunning; got != want {
			t.Fatalf("state after failed Complete() = %q, want %q", got, want)
		}

		if err := handle.Complete(ctx, ProcessCompletion{}); err != nil {
			t.Fatalf("Complete(retry) error = %v", err)
		}
		records = listAllRecords(t, store.MemoryStore)
		if got, want := records[0].State, ProcessStateCompleted; got != want {
			t.Fatalf("state after retry Complete() = %q, want %q", got, want)
		}
		if records[0].CompletedAt == nil {
			t.Fatal("CompletedAt after retry Complete() = nil, want timestamp")
		}
	})
}

func TestMemoryStoreProcessStateUpdateContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject state updates for missing records", func(t *testing.T) {
		t.Parallel()

		store := NewMemoryStore()
		err := store.UpdateProcessRecordState(context.Background(), ProcessStateUpdate{
			ID:        "proc-missing",
			State:     ProcessStateCompleted,
			UpdatedAt: time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, ErrProcessNotFound) {
			t.Fatalf("UpdateProcessRecordState(missing) error = %v, want ErrProcessNotFound", err)
		}
		if records := listAllRecords(t, store); len(records) != 0 {
			t.Fatalf("records = %#v, want none", records)
		}
	})
}

func TestRegistryReconcileBootValidatesRecoveredStartTime(t *testing.T) {
	t.Parallel()

	t.Run("Should recover valid records and stale invalid records", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := NewMemoryStore()
		startedAt := time.Date(2026, 4, 24, 9, 30, 0, 0, time.UTC)
		if err := store.UpsertProcessRecord(ctx, ProcessRecord{
			ID:        "proc-recovered",
			Source:    ProcessSourceSubprocess,
			Owner:     ProcessOwner{SessionID: "sess-1"},
			PID:       12345,
			StartedAt: startedAt,
			State:     ProcessStateRunning,
			CreatedAt: startedAt,
			UpdatedAt: startedAt,
		}); err != nil {
			t.Fatalf("UpsertProcessRecord() error = %v", err)
		}

		registry := NewRegistry(store, WithVerifier(func(pid int, got time.Time) bool {
			return pid == 12345 && got.Equal(startedAt)
		}))
		report, err := registry.ReconcileBoot(ctx)
		if err != nil {
			t.Fatalf("ReconcileBoot() error = %v", err)
		}
		if report.Checked != 1 || report.Recovered != 1 || report.Stale != 0 {
			t.Fatalf("ReconcileBoot() = %#v, want one recovered record", report)
		}
		if got := listAllRecords(t, store)[0].State; got != ProcessStateRunning {
			t.Fatalf("record.State = %q, want running", got)
		}

		staleRegistry := NewRegistry(store, WithVerifier(func(int, time.Time) bool { return false }))
		report, err = staleRegistry.ReconcileBoot(ctx)
		if err != nil {
			t.Fatalf("ReconcileBoot(stale) error = %v", err)
		}
		if report.Checked != 1 || report.Recovered != 0 || report.Stale != 1 {
			t.Fatalf("ReconcileBoot(stale) = %#v, want one stale record", report)
		}
		record := listAllRecords(t, store)[0]
		if record.State != ProcessStateStale {
			t.Fatalf("record.State = %q, want stale", record.State)
		}
		if record.CompletedAt == nil {
			t.Fatal("record.CompletedAt = nil, want stale cleanup timestamp")
		}
	})
}

func TestRegistryScopedInterruptSignalsOnlyMatchingLiveRecord(t *testing.T) {
	t.Parallel()

	t.Run("Should signal only matching live process records", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := NewMemoryStore()
		registry := NewRegistry(store)
		signaled := make(map[string]int)

		first, err := registry.Register(ctx, RegisterConfig{
			ID:     "proc-first",
			Source: ProcessSourceACPTerminal,
			Owner:  ProcessOwner{SessionID: "sess-1", TurnID: "turn-1", ToolCallID: "tool-a"},
			Interrupt: func(_ context.Context, record ProcessRecord) error {
				signaled[record.ID]++
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Register(first) error = %v", err)
		}
		t.Cleanup(func() {
			if err := first.Complete(context.Background(), ProcessCompletion{}); err != nil {
				t.Fatalf("Complete(first cleanup) error = %v", err)
			}
		})

		second, err := registry.Register(ctx, RegisterConfig{
			ID:     "proc-second",
			Source: ProcessSourceACPTerminal,
			Owner:  ProcessOwner{SessionID: "sess-1", TurnID: "turn-1", ToolCallID: "tool-b"},
			Interrupt: func(_ context.Context, record ProcessRecord) error {
				signaled[record.ID]++
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Register(second) error = %v", err)
		}
		t.Cleanup(func() {
			if err := second.Complete(context.Background(), ProcessCompletion{}); err != nil {
				t.Fatalf("Complete(second cleanup) error = %v", err)
			}
		})

		report, err := registry.Interrupt(
			ctx,
			InterruptScope{SessionID: "sess-1", TurnID: "turn-1", ToolCallID: "tool-b"},
		)
		if err != nil {
			t.Fatalf("Interrupt() error = %v", err)
		}
		if report.Matched != 1 || report.Signaled != 1 {
			t.Fatalf("Interrupt() = %#v, want one signaled match", report)
		}
		if signaled["proc-first"] != 0 || signaled["proc-second"] != 1 {
			t.Fatalf("signaled = %#v, want only proc-second", signaled)
		}

		records := listAllRecords(t, store)
		states := map[string]ProcessState{}
		for _, record := range records {
			states[record.ID] = record.State
		}
		if states["proc-first"] != ProcessStateRunning {
			t.Fatalf("proc-first state = %q, want running", states["proc-first"])
		}
		if states["proc-second"] != ProcessStateInterrupting {
			t.Fatalf("proc-second state = %q, want interrupting", states["proc-second"])
		}
	})
}

func TestRegistryInterruptDoesNotSignalRecoveredStalePID(t *testing.T) {
	t.Parallel()

	t.Run("Should mark recovered stale PIDs without signaling", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		store := NewMemoryStore()
		startedAt := time.Date(2026, 4, 24, 9, 0, 0, 0, time.UTC)
		if err := store.UpsertProcessRecord(ctx, ProcessRecord{
			ID:        "proc-stale",
			Source:    ProcessSourceSubprocess,
			Owner:     ProcessOwner{SessionID: "sess-stale"},
			PID:       22222,
			StartedAt: startedAt,
			State:     ProcessStateRunning,
			CreatedAt: startedAt,
			UpdatedAt: startedAt,
		}); err != nil {
			t.Fatalf("UpsertProcessRecord() error = %v", err)
		}

		interrupter := &recordingInterrupter{}
		registry := NewRegistry(
			store,
			WithVerifier(func(int, time.Time) bool { return false }),
			WithInterrupter(interrupter),
		)
		report, err := registry.Interrupt(ctx, InterruptScope{ProcessID: "proc-stale"})
		if err != nil {
			t.Fatalf("Interrupt() error = %v", err)
		}
		if report.Matched != 1 || report.Stale != 1 || report.Signaled != 0 {
			t.Fatalf("Interrupt() = %#v, want one stale unsignaled record", report)
		}
		if interrupter.calls != 0 {
			t.Fatalf("interrupter.calls = %d, want 0", interrupter.calls)
		}
		if got := listAllRecords(t, store)[0].State; got != ProcessStateStale {
			t.Fatalf("record.State = %q, want stale", got)
		}
	})
}

func TestRegistryInterruptPropagatesLiveCallbackError(t *testing.T) {
	t.Parallel()

	t.Run("Should return live callback errors", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		registry := NewRegistry(NewMemoryStore())
		wantErr := errors.New("interrupt failed")
		_, err := registry.Register(ctx, RegisterConfig{
			ID:     "proc-error",
			Source: ProcessSourceHook,
			Owner:  ProcessOwner{HookName: "hook.error"},
			Interrupt: func(context.Context, ProcessRecord) error {
				return wantErr
			},
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		_, err = registry.Interrupt(ctx, InterruptScope{HookName: "hook.error"})
		if !errors.Is(err, wantErr) {
			t.Fatalf("Interrupt() error = %v, want %v", err, wantErr)
		}
	})
}

type failOnceUpdateStore struct {
	*MemoryStore
	err       error
	remaining int
}

func (s *failOnceUpdateStore) UpdateProcessRecordState(ctx context.Context, update ProcessStateUpdate) error {
	if s.remaining == 0 {
		s.remaining++
		return s.err
	}
	return s.MemoryStore.UpdateProcessRecordState(ctx, update)
}

type recordingInterrupter struct {
	calls int
}

func (i *recordingInterrupter) InterruptProcess(context.Context, ProcessRecord) error {
	i.calls++
	return nil
}

func fixedClock(value time.Time) func() time.Time {
	return func() time.Time { return value }
}

func listAllRecords(t *testing.T, store *MemoryStore) []ProcessRecord {
	t.Helper()
	records, err := store.ListProcessRecords(context.Background(), ProcessQuery{})
	if err != nil {
		t.Fatalf("ListProcessRecords() error = %v", err)
	}
	return records
}
