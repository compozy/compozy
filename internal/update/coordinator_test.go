package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCoordinatorRuntimeLifecycle(t *testing.T) {
	t.Run("Should run runtime first then leave the app staged and dormant", func(t *testing.T) {
		t.Parallel()

		store, _, events := newOperationTestStore(t)
		operation, err := store.Acquire(t.Context(), operationTestRequest(testOperationNow))
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		manager := &coordinatorManagerStub{}
		runtime := &coordinatorRuntimeStub{}
		coordinator := newCoordinatorForTest(t, store, manager, runtime)
		if err := coordinator.Run(t.Context(), operation.ID, "generation-1"); err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		live, err := store.Read(t.Context())
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if live == nil || live.Runtime.Phase != PhaseFinalized || live.App.Phase != PhaseStaged ||
			live.Waiting != WaitingForApp || live.Holder != nil {
			t.Fatalf("live operation = %#v, want finalized runtime and dormant staged app", live)
		}
		if runtime.restartCalls != 1 || runtime.healthCalls != 1 || runtime.lock.releaseCalls != 1 ||
			manager.finalizeCalls != 1 || manager.restoreCalls != 0 {
			t.Fatalf("runtime/manager calls = %#v/%#v, want one successful lifecycle", runtime, manager)
		}
		assertEventOrder(t, events.snapshot(), []OperationEventName{
			EventOperationAcquired,
			EventDownloadProgress,
			EventVerifyCompleted,
			EventSwapCompleted,
			EventRestartCompleted,
			EventHealthCompleted,
			EventFinalized,
			EventAppStaged,
			EventOperationWaiting,
		})
	})

	t.Run("Should restore and archive when the replacement daemon fails health", func(t *testing.T) {
		t.Parallel()

		store, paths, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetRuntime}
		request.App = nil
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		manager := &coordinatorManagerStub{}
		runtime := &coordinatorRuntimeStub{healthErr: errors.New("replacement is unhealthy")}
		coordinator := newCoordinatorForTest(t, store, manager, runtime)
		err = coordinator.Run(t.Context(), operation.ID, "generation-1")
		if err == nil || !errors.Is(err, runtime.healthErr) {
			t.Fatalf("Run() error = %v, want health failure", err)
		}
		if manager.restoreCalls != 1 || manager.finalizeCalls != 0 {
			t.Fatalf("manager restore/finalize = %d/%d, want 1/0", manager.restoreCalls, manager.finalizeCalls)
		}
		assertArchivedOperationPhase(t, paths.UpdateHistoryFile, operation.ID, PhaseRolledBack)
	})

	t.Run("Should refuse a runtime-only compatibility violation before acquiring the swap lock", func(t *testing.T) {
		t.Parallel()

		store, paths, _ := newOperationTestStore(t)
		request := operationTestRequest(testOperationNow)
		request.Targets = []Target{TargetRuntime}
		request.App = nil
		operation, err := store.Acquire(t.Context(), request)
		if err != nil {
			t.Fatalf("Acquire() error = %v", err)
		}
		manager := &coordinatorManagerStub{
			compatibility: Compatibility{RuntimeVersion: "v1.1.0", MinAppVersion: "v2.0.0"},
		}
		runtime := &coordinatorRuntimeStub{installedApp: InstalledApp{Present: true, Version: "v1.0.0"}}
		coordinator := newCoordinatorForTest(t, store, manager, runtime)
		err = coordinator.Run(t.Context(), operation.ID, "generation-1")
		if err == nil {
			t.Fatal("Run() error = nil, want compatibility refusal")
		}
		if manager.swapStarted || runtime.lockAcquireCalls != 0 {
			t.Fatalf("swap/lock = %t/%d, want no mutation", manager.swapStarted, runtime.lockAcquireCalls)
		}
		assertArchivedOperationPhase(t, paths.UpdateHistoryFile, operation.ID, PhaseFailed)
	})

	t.Run("Should recover every interrupted runtime phase", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name           string
			phase          OperationPhase
			backupPresent  bool
			wantRestore    int
			wantRestart    int
			wantHealth     int
			wantFinalized  bool
			wantRolledBack bool
		}{
			{name: "swap with backup", phase: PhaseSwapping, backupPresent: true, wantRestore: 1, wantRolledBack: true},
			{name: "swap without backup", phase: PhaseSwapping, wantRestart: 1, wantHealth: 1, wantFinalized: true},
			{name: "daemon restart", phase: PhaseRestarting, backupPresent: true, wantRestart: 1, wantHealth: 1, wantFinalized: true},
			{name: "health check", phase: PhaseHealthChecking, backupPresent: true, wantHealth: 1, wantFinalized: true},
		} {
			t.Run("Should recover "+test.name, func(t *testing.T) {
				t.Parallel()

				store, paths, _ := newOperationTestStore(t)
				request := operationTestRequest(testOperationNow)
				request.Targets = []Target{TargetRuntime}
				request.App = nil
				operation, err := store.Acquire(t.Context(), request)
				if err != nil {
					t.Fatalf("Acquire() error = %v", err)
				}
				backupPath := filepath.Join(t.TempDir(), "compozy.backup")
				if test.backupPresent {
					if err := os.WriteFile(backupPath, []byte("backup"), 0o700); err != nil {
						t.Fatalf("WriteFile(backup) error = %v", err)
					}
				}
				operation = advanceRuntimeForRecovery(t, store, operation, test.phase, backupPath)
				manager := &coordinatorManagerStub{}
				runtime := &coordinatorRuntimeStub{}
				coordinator := newCoordinatorForTest(t, store, manager, runtime)
				runErr := coordinator.Run(t.Context(), operation.ID, "generation-1")
				if test.wantRolledBack {
					if runErr == nil || !strings.Contains(runErr.Error(), "interrupted runtime swap") {
						t.Fatalf("Run() error = %v, want interrupted-swap rollback", runErr)
					}
					assertArchivedOperationPhase(t, paths.UpdateHistoryFile, operation.ID, PhaseRolledBack)
				} else if runErr != nil {
					t.Fatalf("Run() error = %v", runErr)
				}
				if manager.restoreCalls != test.wantRestore || runtime.restartCalls != test.wantRestart ||
					runtime.healthCalls != test.wantHealth {
					t.Fatalf(
						"restore/restart/health = %d/%d/%d, want %d/%d/%d",
						manager.restoreCalls,
						runtime.restartCalls,
						runtime.healthCalls,
						test.wantRestore,
						test.wantRestart,
						test.wantHealth,
					)
				}
				if test.wantFinalized {
					assertArchivedOperationPhase(t, paths.UpdateHistoryFile, operation.ID, PhaseFinalized)
				}
			})
		}
	})
}

type coordinatorManagerStub struct {
	compatibility Compatibility
	swapStarted   bool
	restoreCalls  int
	finalizeCalls int
	restored      AppliedBinary
}

func (s *coordinatorManagerStub) ResolveReleaseByTag(context.Context, string) (*Release, error) {
	return &Release{Version: "v1.1.0"}, nil
}

func (s *coordinatorManagerStub) ApplyReleaseObserved(
	ctx context.Context,
	_ *Release,
	observer ApplyObserver,
) (AppliedBinary, error) {
	compatibility := s.compatibility
	if compatibility.RuntimeVersion == "" {
		compatibility = Compatibility{RuntimeVersion: "v1.1.0", MinAppVersion: "v1.0.0"}
	}
	steps := []ApplyStep{
		{Phase: PhaseDownloading, Percent: 0},
		{Phase: PhaseVerifying, Percent: -1},
		{Phase: PhaseSwapping, Percent: -1, BackupPath: "/tmp/compozy.backup", Compatibility: &compatibility},
	}
	for _, step := range steps {
		if err := observer(ctx, step); err != nil {
			return AppliedBinary{}, err
		}
	}
	s.swapStarted = true
	return AppliedBinary{TargetPath: "/tmp/compozy", BackupPath: "/tmp/compozy.backup", Version: "v1.1.0"}, nil
}

func (s *coordinatorManagerStub) RuntimeTargetPath() string { return "/tmp/compozy" }

func (s *coordinatorManagerStub) Restore(applied AppliedBinary) error {
	s.restoreCalls++
	s.restored = applied
	return nil
}

func (s *coordinatorManagerStub) Finalize(AppliedBinary) error {
	s.finalizeCalls++
	return nil
}

type coordinatorRuntimeStub struct {
	lock             coordinatorMutationLockStub
	lockAcquireCalls int
	restartCalls     int
	healthCalls      int
	restartErr       error
	healthErr        error
	installedApp     InstalledApp
}

func (s *coordinatorRuntimeStub) AcquireMutationLock(context.Context) (MutationLock, error) {
	s.lockAcquireCalls++
	return &s.lock, nil
}

func (s *coordinatorRuntimeStub) RestartDaemon(context.Context) error {
	s.restartCalls++
	return s.restartErr
}

func (s *coordinatorRuntimeStub) HealthCheck(context.Context) error {
	s.healthCalls++
	return s.healthErr
}

func (s *coordinatorRuntimeStub) InstalledApp(context.Context) (InstalledApp, error) {
	return s.installedApp, nil
}

type coordinatorMutationLockStub struct{ releaseCalls int }

func (s *coordinatorMutationLockStub) Release() error {
	s.releaseCalls++
	return nil
}

func newCoordinatorForTest(
	t *testing.T,
	store *OperationStore,
	manager CoordinatorManager,
	runtime CoordinatorRuntime,
) *Coordinator {
	t.Helper()
	coordinator, err := NewCoordinator(CoordinatorConfig{
		Store: store, ReleaseManager: manager, BinaryManager: manager, Runtime: runtime,
		LeaseDuration: time.Hour, RenewInterval: 30 * time.Minute,
		Now: func() time.Time { return testOperationNow },
	})
	if err != nil {
		t.Fatalf("NewCoordinator() error = %v", err)
	}
	return coordinator
}

func advanceRuntimeForRecovery(
	t *testing.T,
	store *OperationStore,
	operation *Operation,
	targetPhase OperationPhase,
	backupPath string,
) *Operation {
	t.Helper()
	for _, phase := range []OperationPhase{
		PhaseDownloading,
		PhaseVerifying,
		PhaseSwapping,
		PhaseRestarting,
		PhaseHealthChecking,
	} {
		if phase == PhaseRestarting && targetPhase == PhaseSwapping {
			break
		}
		if phase == PhaseHealthChecking && targetPhase == PhaseRestarting {
			break
		}
		transition := Transition{
			Kind: TransitionPhase, Actor: ActorCLI, Target: TargetRuntime,
			Phase: phase, Percent: -1, Outcome: operationOutcomeStarted,
		}
		if phase == PhaseSwapping {
			transition.BackupPath = backupPath
		}
		updated, err := store.Transition(
			t.Context(), operation.ID, "generation-1", operation.Revision, transition,
		)
		if err != nil {
			t.Fatalf("Transition(%s) error = %v", phase, err)
		}
		operation = updated
		if phase == targetPhase {
			break
		}
	}
	return operation
}

func assertEventOrder(t *testing.T, events []OperationEvent, names []OperationEventName) {
	t.Helper()
	if len(events) != len(names) {
		t.Fatalf("event count = %d, want %d: %#v", len(events), len(names), events)
	}
	for index, name := range names {
		if events[index].Name != name {
			t.Fatalf("event[%d] = %q, want %q", index, events[index].Name, name)
		}
		if events[index].OperationID == "" || events[index].Revision < 1 || events[index].Outcome == "" {
			t.Fatalf("event[%d] missing correlation fields: %#v", index, events[index])
		}
	}
}

func assertArchivedOperationPhase(t *testing.T, historyPath string, operationID string, phase OperationPhase) {
	t.Helper()
	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("ReadFile(history) error = %v", err)
	}
	for _, line := range splitNonEmptyLines(string(data)) {
		var operation Operation
		if err := json.Unmarshal([]byte(line), &operation); err != nil {
			t.Fatalf("Unmarshal(history) error = %v", err)
		}
		if operation.ID == operationID {
			if operation.Runtime == nil || operation.Runtime.Phase != phase {
				t.Fatalf("archived operation = %#v, want runtime phase %q", operation, phase)
			}
			return
		}
	}
	t.Fatalf("history missing operation %q", operationID)
}
