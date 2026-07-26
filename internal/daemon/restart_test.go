package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

func TestRestartStoreRoundTripAndReadyTransition(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	oldStartedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	times := []time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 5, 0, 0, time.UTC),
	}
	store := newRestartStore(homePaths, sequentialTime(times))

	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-roundtrip",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       oldStartedAt,
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 3,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if got, want := operation.StartedAt, times[0]; !got.Equal(want) {
		t.Fatalf("Create() started_at = %v, want %v", got, want)
	}
	if got, want := operation.UpdatedAt, times[0]; !got.Equal(want) {
		t.Fatalf("Create() updated_at = %v, want %v", got, want)
	}

	for _, status := range []RestartStatus{
		RestartStatusStopping,
		RestartStatusWaitingRelease,
		RestartStatusStarting,
	} {
		operation, err = store.Transition(operation.OperationID, restartTransition{status: status})
		if err != nil {
			t.Fatalf("Transition(%s) error = %v", status, err)
		}
	}

	ready, err := store.Transition(operation.OperationID, restartTransition{
		status: RestartStatusReady,
		newPID: 9393,
	})
	if err != nil {
		t.Fatalf("Transition(ready) error = %v", err)
	}
	if ready.CompletedAt == nil {
		t.Fatal("ready.CompletedAt = nil, want populated completion timestamp")
	}
	if got, want := ready.NewPID, 9393; got != want {
		t.Fatalf("ready.NewPID = %d, want %d", got, want)
	}

	loaded, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got, want := loaded.Status, RestartStatusReady; got != want {
		t.Fatalf("loaded.Status = %q, want %q", got, want)
	}
	if got, want := loaded.OldPID, 4242; got != want {
		t.Fatalf("loaded.OldPID = %d, want %d", got, want)
	}
	if got, want := loaded.ActiveSessionCount, 3; got != want {
		t.Fatalf("loaded.ActiveSessionCount = %d, want %d", got, want)
	}
	if got, want := loaded.NewPID, 9393; got != want {
		t.Fatalf("loaded.NewPID = %d, want %d", got, want)
	}
	if got, want := loaded.StartedAt, times[0]; !got.Equal(want) {
		t.Fatalf("loaded.StartedAt = %v, want %v", got, want)
	}
	if got, want := loaded.UpdatedAt, times[4]; !got.Equal(want) {
		t.Fatalf("loaded.UpdatedAt = %v, want %v", got, want)
	}
	if loaded.CompletedAt == nil || !loaded.CompletedAt.Equal(times[4]) {
		t.Fatalf("loaded.CompletedAt = %v, want %v", loaded.CompletedAt, times[4])
	}
}

func TestRestartOperationValidateRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	validBase := RestartOperation{
		OperationID:        "restart-validate",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      "/tmp/agh.sock",
		ActiveSessionCount: 1,
		StartedAt:          time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		UpdatedAt:          time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
	}

	testCases := []struct {
		name      string
		mutate    func(*RestartOperation)
		wantError string
	}{
		{
			name: "Should reject blank operation id",
			mutate: func(op *RestartOperation) {
				op.OperationID = ""
			},
			wantError: "restart operation id is required",
		},
		{
			name: "Should reject path separator in operation id",
			mutate: func(op *RestartOperation) {
				op.OperationID = "bad/id"
			},
			wantError: "contains path separators",
		},
		{
			name: "Should reject blank old socket path",
			mutate: func(op *RestartOperation) {
				op.OldSocketPath = ""
			},
			wantError: "old socket path is required",
		},
		{
			name: "Should reject non terminal new pid",
			mutate: func(op *RestartOperation) {
				op.NewPID = 9393
			},
			wantError: "must not set new_pid before ready",
		},
		{
			name: "Should reject ready without completed_at",
			mutate: func(op *RestartOperation) {
				op.Status = RestartStatusReady
				op.NewPID = 9393
			},
			wantError: "ready restart operation requires completed_at",
		},
		{
			name: "Should reject failed without reason",
			mutate: func(op *RestartOperation) {
				now := time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC)
				op.Status = RestartStatusFailed
				op.CompletedAt = &now
			},
			wantError: "failed restart operation requires failure_reason",
		},
		{
			name: "Should reject unknown status",
			mutate: func(op *RestartOperation) {
				op.Status = RestartStatus("mystery")
			},
			wantError: "unsupported restart status",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			operation := validBase
			tc.mutate(&operation)
			err := operation.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.wantError)
			}
		})
	}
}

func TestRestartStoreRejectsRegressionsAndDoubleTerminalTransitions(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	oldStartedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))

	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-invalid-transition",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       oldStartedAt,
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusWaitingRelease})
	if err != nil {
		t.Fatalf("Transition(waiting_release) error = %v", err)
	}
	if _, err := store.Transition(
		operation.OperationID,
		restartTransition{status: RestartStatusStopping},
	); !errors.Is(
		err,
		errInvalidRestartTransition,
	) {
		t.Fatalf("Transition(regression) error = %v, want errInvalidRestartTransition", err)
	}

	failed, err := store.Transition(operation.OperationID, restartTransition{
		status:        RestartStatusFailed,
		failureReason: "replacement crashed",
	})
	if err != nil {
		t.Fatalf("Transition(failed) error = %v", err)
	}
	if got, want := failed.Status, RestartStatusFailed; got != want {
		t.Fatalf("failed.Status = %q, want %q", got, want)
	}

	if _, err := store.Transition(operation.OperationID, restartTransition{
		status: RestartStatusReady,
		newPID: 9393,
	}); !errors.Is(err, errInvalidRestartTransition) {
		t.Fatalf("Transition(double terminal) error = %v, want errInvalidRestartTransition", err)
	}
}

func TestRestartStoreDefaultsAndLookupErrors(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, nil)
	if store.now == nil {
		t.Fatal("newRestartStore(nil now) left now function nil")
	}

	_, err := store.Get("missing-restart")
	if !errors.Is(err, ErrRestartOperationNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrRestartOperationNotFound", err)
	}

	if _, err := store.operationPath("bad/id"); err == nil || !strings.Contains(err.Error(), "path separators") {
		t.Fatalf("operationPath(bad/id) error = %v, want path separator validation", err)
	}

	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-duplicate",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.Create(operation); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("Create(duplicate) error = %v, want duplicate guard", err)
	}
}

func TestRequestRestartPersistsHelperLaunchFailure(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.config = cfg
	d.startedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	d.pid = func() int { return 4242 }
	d.sessions = &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}, {ID: "sess-b"}},
	}
	d.executable = func() (string, error) { return "/usr/bin/agh", nil }
	d.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		return nil, errors.New("helper exploded")
	}

	operation, err := d.RequestRestart(testutil.Context(t))
	if err == nil || !strings.Contains(err.Error(), "spawn relaunch helper") {
		t.Fatalf("RequestRestart() error = %v, want helper spawn failure", err)
	}
	if got, want := operation.Status, RestartStatusFailed; got != want {
		t.Fatalf("operation.Status = %q, want %q", got, want)
	}

	persisted, err := d.GetRestartOperation(testutil.Context(t), operation.OperationID)
	if err != nil {
		t.Fatalf("GetRestartOperation() error = %v", err)
	}
	if got, want := persisted.OldPID, 4242; got != want {
		t.Fatalf("persisted.OldPID = %d, want %d", got, want)
	}
	if got, want := persisted.ActiveSessionCount, 2; got != want {
		t.Fatalf("persisted.ActiveSessionCount = %d, want %d", got, want)
	}
	if got := persisted.NewPID; got != 0 {
		t.Fatalf("persisted.NewPID = %d, want 0 before successful replacement boot", got)
	}
	if !strings.Contains(persisted.FailureReason, "helper exploded") {
		t.Fatalf("persisted.FailureReason = %q, want helper spawn context", persisted.FailureReason)
	}
	if persisted.CompletedAt == nil {
		t.Fatal("persisted.CompletedAt = nil, want terminal failure timestamp")
	}
}

func TestRequestRestartExecutableFailurePersistsFailedOperation(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.config = cfg
	d.startedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	d.pid = func() int { return 4242 }
	d.executable = func() (string, error) { return "", errors.New("no executable") }

	operation, err := d.RequestRestart(testutil.Context(t))
	if err == nil || !strings.Contains(err.Error(), "resolve relaunch helper executable") {
		t.Fatalf("RequestRestart() error = %v, want executable failure", err)
	}
	if got, want := operation.Status, RestartStatusFailed; got != want {
		t.Fatalf("operation.Status = %q, want %q", got, want)
	}
	if !strings.Contains(operation.FailureReason, "no executable") {
		t.Fatalf("operation.FailureReason = %q, want executable failure", operation.FailureReason)
	}
}

func TestFailRestartOperationSurfacesPersistenceFailures(t *testing.T) {
	t.Parallel()

	store := newRestartStore(testHomePaths(t), sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
	}))
	actionErr := errors.New("signal failed")

	_, err := failRestartOperation(store, RestartOperation{OperationID: "bad/id"}, "signal daemon shutdown", actionErr)
	if err == nil {
		t.Fatal("failRestartOperation() error = nil, want joined action and persistence failures")
	}
	if !errors.Is(err, actionErr) {
		t.Fatalf("failRestartOperation() error = %v, want wrapped action error", err)
	}
	if !strings.Contains(err.Error(), `persist failed restart operation "bad/id"`) {
		t.Fatalf("failRestartOperation() error = %q, want persistence failure context", err.Error())
	}
}

func TestRequestRestartWritesOperationBeforeShutdownSignal(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.config = cfg
	d.startedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	d.pid = func() int { return 4242 }
	d.sessions = &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}, {ID: "sess-b"}, {ID: "sess-c"}},
	}
	d.executable = func() (string, error) { return "/usr/bin/agh", nil }

	var helperRequest detachedStartRequest
	d.startDetached = func(_ context.Context, req detachedStartRequest) (restartProcess, error) {
		helperRequest = req
		return restartProcessStub{pid: 9001}, nil
	}

	signalObserved := false
	d.signalProcess = func(pid int, sig syscall.Signal) error {
		signalObserved = true
		if got, want := pid, 4242; got != want {
			t.Fatalf("signal pid = %d, want %d", got, want)
		}
		if got, want := sig, syscall.SIGTERM; got != want {
			t.Fatalf("signal = %v, want %v", got, want)
		}

		store := newRestartStore(homePaths, d.now)
		operationID := envValue(helperRequest.sandbox, RestartOperationEnvKey)
		if strings.TrimSpace(operationID) == "" {
			t.Fatal("helper launch env did not include restart operation id")
		}
		persisted, err := store.Get(operationID)
		if err != nil {
			t.Fatalf("store.Get(%q) error = %v", operationID, err)
		}
		if got, want := persisted.Status, RestartStatusStopping; got != want {
			t.Fatalf("persisted.Status = %q, want %q before shutdown signal", got, want)
		}
		if got, want := persisted.ActiveSessionCount, 3; got != want {
			t.Fatalf("persisted.ActiveSessionCount = %d, want %d", got, want)
		}
		if got, want := persisted.OldSocketPath, cfg.Daemon.Socket; got != want {
			t.Fatalf("persisted.OldSocketPath = %q, want %q", got, want)
		}
		return nil
	}

	operation, err := d.RequestRestart(testutil.Context(t))
	if err != nil {
		t.Fatalf("RequestRestart() error = %v", err)
	}
	if !signalObserved {
		t.Fatal("signalProcess() was not called")
	}
	if got, want := operation.Status, RestartStatusStopping; got != want {
		t.Fatalf("operation.Status = %q, want %q", got, want)
	}
}

func TestRequestRestartSignalFailurePersistsFailedOperation(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.config = cfg
	d.startedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	d.pid = func() int { return 4242 }
	d.sessions = &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}},
	}
	d.executable = func() (string, error) { return "/usr/bin/agh", nil }
	d.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		return restartProcessStub{pid: 9001}, nil
	}
	d.signalProcess = func(int, syscall.Signal) error {
		return errors.New("signal failed")
	}

	operation, err := d.RequestRestart(testutil.Context(t))
	if err == nil || !strings.Contains(err.Error(), "signal daemon shutdown") {
		t.Fatalf("RequestRestart() error = %v, want signal failure", err)
	}
	if got, want := operation.Status, RestartStatusFailed; got != want {
		t.Fatalf("operation.Status = %q, want %q", got, want)
	}
	if !strings.Contains(operation.FailureReason, "signal failed") {
		t.Fatalf("operation.FailureReason = %q, want signal failure", operation.FailureReason)
	}
}

func TestRelaunchHelperWaitsForReleaseBeforeLaunchingReplacement(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	oldStartedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))

	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-release-wait",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       oldStartedAt,
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}

	lock, err := AcquireLock(homePaths.DaemonLock, operation.OldPID)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	t.Cleanup(func() {
		_ = lock.Release()
	})
	if err := WriteInfo(homePaths.DaemonInfo, Info{
		PID:       operation.OldPID,
		Port:      2123,
		StartedAt: operation.OldStartedAt,
	}); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}
	if err := os.WriteFile(operation.OldSocketPath, []byte("busy"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(old socket marker) error = %v", err)
	}

	var oldAlive atomic.Bool
	oldAlive.Store(true)

	launchStarted := make(chan struct{})
	observedOldAlive := make(chan struct{})
	var observedOldAliveOnce atomic.Bool
	allowExit := make(chan struct{})
	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: time.Second,
		ReadyTimeout:   time.Second,
	})
	helper.processAlive = func(int) bool {
		if !oldAlive.Load() {
			return false
		}
		if observedOldAliveOnce.CompareAndSwap(false, true) {
			close(observedOldAlive)
		}
		return true
	}
	helper.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		close(launchStarted)
		if _, err := store.Transition(operation.OperationID, restartTransition{
			status: RestartStatusReady,
			newPID: 9393,
		}); err != nil {
			return nil, err
		}
		return restartProcessStub{
			pid: 9393,
			wait: func() error {
				<-allowExit
				return nil
			},
		}, nil
	}

	runCtx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
	defer cancel()
	resultCh := make(chan error, 1)
	go func() {
		resultCh <- helper.run(runCtx)
	}()

	select {
	case <-observedOldAlive:
	case <-runCtx.Done():
		t.Fatalf("helper.run() did not observe the old daemon before timeout: %v", runCtx.Err())
	}

	select {
	case <-launchStarted:
		t.Fatal("replacement launch started before singleton resources were released")
	default:
	}

	oldAlive.Store(false)
	if err := os.Remove(operation.OldSocketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Remove(old socket) error = %v", err)
	}
	if err := RemoveInfo(homePaths.DaemonInfo); err != nil {
		t.Fatalf("RemoveInfo() error = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("lock.Release() error = %v", err)
	}

	if err := <-resultCh; err != nil {
		t.Fatalf("helper.run() error = %v", err)
	}
	close(allowExit)

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusReady; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
}

func TestRelaunchHelperReturnsWithoutWorkForTerminalFailure(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-terminal-failure",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 0,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	_, err = store.Transition(operation.OperationID, restartTransition{
		status:        RestartStatusFailed,
		failureReason: "already failed",
	})
	if err != nil {
		t.Fatalf("Transition(failed) error = %v", err)
	}

	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:   homePaths,
		OperationID: operation.OperationID,
	})
	helper.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		t.Fatal("helper.startDetached() was called for an already-terminal operation")
		return nil, nil
	}

	if err := helper.run(testutil.Context(t)); err != nil {
		t.Fatalf("helper.run() error = %v, want nil for terminal operation", err)
	}
}

func TestRelaunchHelperExecutableFailurePersistsFailedOperation(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-helper-exec-failure",
		Status:             RestartStatusPending,
		OldPID:             999999,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 0,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}

	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: 200 * time.Millisecond,
		ReadyTimeout:   200 * time.Millisecond,
		Executable: func() (string, error) {
			return "", errors.New("missing replacement binary")
		},
	})
	helper.processAlive = func(int) bool { return false }

	err = helper.run(testutil.Context(t))
	if err == nil || !strings.Contains(err.Error(), "resolve replacement executable") {
		t.Fatalf("helper.run() error = %v, want executable failure", err)
	}

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusFailed; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
}

func TestRelaunchHelperReleaseTimeoutPersistsFailure(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-release-timeout",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 0,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}

	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: 50 * time.Millisecond,
		ReadyTimeout:   50 * time.Millisecond,
	})
	helper.processAlive = func(int) bool { return true }
	helper.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		t.Fatal("helper.startDetached() was called before release conditions were met")
		return nil, nil
	}

	err = helper.run(testutil.Context(t))
	if err == nil || !strings.Contains(err.Error(), "release wait timed out") {
		t.Fatalf("helper.run() error = %v, want release timeout", err)
	}

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusFailed; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
}

func TestRelaunchHelperStoppingTimeoutPersistsFailure(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-stopping-timeout",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 0,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: 50 * time.Millisecond,
		ReadyTimeout:   50 * time.Millisecond,
	})
	helper.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		t.Fatal("helper.startDetached() was called before restart entered stopping")
		return nil, nil
	}

	err = helper.run(testutil.Context(t))
	if err == nil || !strings.Contains(err.Error(), "restart operation did not enter stopping before timeout") {
		t.Fatalf("helper.run() error = %v, want stopping timeout", err)
	}

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusFailed; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
	if !strings.Contains(persisted.FailureReason, "did not enter stopping before timeout") {
		t.Fatalf("persisted.FailureReason = %q, want stopping timeout", persisted.FailureReason)
	}
}

func TestRelaunchHelperReleaseConditionsMetBranches(t *testing.T) {
	t.Parallel()

	baseOperation := RestartOperation{
		OperationID:   "restart-release-branches",
		OldPID:        4242,
		OldStartedAt:  time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath: filepath.Join(t.TempDir(), "daemon.sock"),
	}

	t.Run("Should Socket still exists", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		operation := baseOperation
		operation.OldSocketPath = filepath.Join(t.TempDir(), "daemon.sock")
		if err := os.WriteFile(operation.OldSocketPath, []byte("busy"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(socket) error = %v", err)
		}

		helper := newRelaunchHelper(RelaunchHelperConfig{HomePaths: homePaths})
		helper.processAlive = func(int) bool { return false }
		helper.acquireLock = func(string, int) (*Lock, error) {
			t.Fatal("helper.acquireLock() was called before old socket release")
			return nil, nil
		}

		released, err := helper.releaseConditionsMet(operation)
		if err != nil {
			t.Fatalf("releaseConditionsMet() error = %v", err)
		}
		if released {
			t.Fatal("releaseConditionsMet() = true, want false while socket exists")
		}
	})

	t.Run("Should Daemon info still exists", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		helper := newRelaunchHelper(RelaunchHelperConfig{HomePaths: homePaths})
		helper.processAlive = func(int) bool { return false }
		helper.acquireLock = func(string, int) (*Lock, error) {
			t.Fatal("helper.acquireLock() was called before daemon info release")
			return nil, nil
		}
		if err := WriteInfo(homePaths.DaemonInfo, Info{
			PID:       7777,
			Port:      4123,
			StartedAt: time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		}); err != nil {
			t.Fatalf("WriteInfo() error = %v", err)
		}

		released, err := helper.releaseConditionsMet(baseOperation)
		if err != nil {
			t.Fatalf("releaseConditionsMet() error = %v", err)
		}
		if released {
			t.Fatal("releaseConditionsMet() = true, want false while daemon info exists")
		}
	})

	t.Run("Should Daemon lock still held", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		helper := newRelaunchHelper(RelaunchHelperConfig{HomePaths: homePaths})
		helper.processAlive = func(int) bool { return false }
		helper.acquireLock = func(string, int) (*Lock, error) {
			return nil, errAlreadyRunning{pid: 5151}
		}

		released, err := helper.releaseConditionsMet(baseOperation)
		if err != nil {
			t.Fatalf("releaseConditionsMet() error = %v", err)
		}
		if released {
			t.Fatal("releaseConditionsMet() = true, want false while lock is held")
		}
	})

	t.Run("Should Lock probe release failure", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		helper := newRelaunchHelper(RelaunchHelperConfig{HomePaths: homePaths})
		helper.processAlive = func(int) bool { return false }
		helper.acquireLock = func(string, int) (*Lock, error) {
			return &Lock{
				path: homePaths.DaemonLock,
				releaseFn: func() error {
					return errors.New("lock release probe failed")
				},
			}, nil
		}

		released, err := helper.releaseConditionsMet(baseOperation)
		if err == nil || !strings.Contains(err.Error(), "lock release probe failed") {
			t.Fatalf("releaseConditionsMet() error = %v, want release probe failure", err)
		}
		if released {
			t.Fatal("releaseConditionsMet() = true, want false on release probe failure")
		}
	})

	t.Run("Should Unexpected lock probe error", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		helper := newRelaunchHelper(RelaunchHelperConfig{HomePaths: homePaths})
		helper.processAlive = func(int) bool { return false }
		helper.acquireLock = func(string, int) (*Lock, error) {
			return nil, errors.New("probe blew up")
		}

		released, err := helper.releaseConditionsMet(baseOperation)
		if err == nil || !strings.Contains(err.Error(), "probe blew up") {
			t.Fatalf("releaseConditionsMet() error = %v, want probe failure", err)
		}
		if released {
			t.Fatal("releaseConditionsMet() = true, want false on probe failure")
		}
	})
}

func TestMarkRestartReadyRequiresFreshDaemonInfo(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.config = cfg

	oldStartedAt := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))

	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-ready-freshness",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       oldStartedAt,
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusWaitingRelease})
	if err != nil {
		t.Fatalf("Transition(waiting_release) error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStarting})
	if err != nil {
		t.Fatalf("Transition(starting) error = %v", err)
	}

	d.getenv = func(key string) string {
		if key == RestartOperationEnvKey {
			return operation.OperationID
		}
		return ""
	}

	sameInfo := Info{
		PID:       operation.OldPID,
		Port:      2123,
		StartedAt: operation.OldStartedAt,
	}
	if err := d.markRestartReadyIfRequested(sameInfo); err == nil {
		t.Fatal("markRestartReadyIfRequested() error = nil, want stale discovery rejection")
	}

	stillStarting, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() after stale info error = %v", err)
	}
	if got, want := stillStarting.Status, RestartStatusStarting; got != want {
		t.Fatalf("stillStarting.Status = %q, want %q", got, want)
	}
	if got := stillStarting.NewPID; got != 0 {
		t.Fatalf("stillStarting.NewPID = %d, want 0 before success", got)
	}

	freshInfo := Info{
		PID:       9393,
		Port:      2123,
		StartedAt: operation.OldStartedAt.Add(time.Second),
	}
	if err := d.markRestartReadyIfRequested(freshInfo); err != nil {
		t.Fatalf("markRestartReadyIfRequested(fresh) error = %v", err)
	}

	ready, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() ready error = %v", err)
	}
	if got, want := ready.Status, RestartStatusReady; got != want {
		t.Fatalf("ready.Status = %q, want %q", got, want)
	}
	if got, want := ready.NewPID, 9393; got != want {
		t.Fatalf("ready.NewPID = %d, want %d", got, want)
	}
}

func TestWaitForReadyReturnsFailureContextWhenPollingReadBreaks(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-read-failure",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, status := range []RestartStatus{
		RestartStatusStopping,
		RestartStatusWaitingRelease,
		RestartStatusStarting,
	} {
		operation, err = store.Transition(operation.OperationID, restartTransition{status: status})
		if err != nil {
			t.Fatalf("Transition(%s) error = %v", status, err)
		}
	}

	if err := os.RemoveAll(homePaths.RestartsDir); err != nil {
		t.Fatalf("os.RemoveAll(%q) error = %v", homePaths.RestartsDir, err)
	}
	if err := os.WriteFile(homePaths.RestartsDir, []byte("broken"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", homePaths.RestartsDir, err)
	}

	waitBlocked := make(chan struct{})
	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:    homePaths,
		OperationID:  operation.OperationID,
		PollInterval: 5 * time.Millisecond,
		ReadyTimeout: 100 * time.Millisecond,
	})
	err = helper.waitForReady(testutil.Context(t), store, operation.OperationID, restartProcessStub{
		pid: 9393,
		wait: func() error {
			<-waitBlocked
			return nil
		},
	})
	close(waitBlocked)

	if err == nil || !strings.Contains(err.Error(), `load restart operation "restart-read-failure"`) {
		t.Fatalf("waitForReady() error = %v, want restart-operation read failure context", err)
	}
}

func TestWaitForReadyPreservesCancellationCause(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-ready-canceled",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 1,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	for _, status := range []RestartStatus{
		RestartStatusStopping,
		RestartStatusWaitingRelease,
		RestartStatusStarting,
	} {
		operation, err = store.Transition(operation.OperationID, restartTransition{status: status})
		if err != nil {
			t.Fatalf("Transition(%s) error = %v", status, err)
		}
	}

	waitBlocked := make(chan struct{})
	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:     homePaths,
		OperationID:   operation.OperationID,
		PollInterval:  5 * time.Millisecond,
		ReadyTimeout:  time.Second,
		ExitDrainWait: 50 * time.Millisecond,
	})
	ctx, cancel := context.WithCancelCause(testutil.Context(t))
	cancel(errors.New("operator canceled restart"))

	err = helper.waitForReady(ctx, store, operation.OperationID, restartProcessStub{
		pid: 9393,
		wait: func() error {
			<-waitBlocked
			return nil
		},
	})
	close(waitBlocked)

	if err == nil {
		t.Fatal("waitForReady(canceled) error = nil, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForReady(canceled) error = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "operator canceled restart") ||
		!strings.Contains(err.Error(), "replacement daemon readiness canceled") {
		t.Fatalf("waitForReady(canceled) error = %v, want cancellation cause", err)
	}
	if strings.Contains(err.Error(), "did not become ready before timeout") {
		t.Fatalf("waitForReady(canceled) error = %v, want cancellation instead of timeout", err)
	}

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusFailed; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
	if !strings.Contains(persisted.FailureReason, "operator canceled restart") {
		t.Fatalf("persisted.FailureReason = %q, want cancellation cause", persisted.FailureReason)
	}
}

func TestRestartOperationFreshInfoCheck(t *testing.T) {
	t.Parallel()

	operation := RestartOperation{
		OldPID:       4242,
		OldStartedAt: time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
	}

	if operation.hasFreshDaemonInfo(Info{}) {
		t.Fatal("hasFreshDaemonInfo(invalid info) = true, want false")
	}
	if !operation.hasFreshDaemonInfo(Info{
		PID:       4242,
		Port:      2123,
		StartedAt: operation.OldStartedAt.Add(time.Second),
	}) {
		t.Fatal("hasFreshDaemonInfo(same pid new start time) = false, want true")
	}
}

func TestRestartOperationIDFromEnvTrimsWhitespace(t *testing.T) {
	t.Parallel()

	got := restartOperationIDFromEnv(func(string) string { return "  restart-op  " })
	if want := "restart-op"; got != want {
		t.Fatalf("restartOperationIDFromEnv() = %q, want %q", got, want)
	}
}

func TestRestartOperationIDFromEnvHandlesNilGetter(t *testing.T) {
	t.Parallel()

	if got := restartOperationIDFromEnv(nil); got != "" {
		t.Fatalf("restartOperationIDFromEnv(nil) = %q, want empty string", got)
	}
}

type restartProcessStub struct {
	pid  int
	wait func() error
}

func (p restartProcessStub) PID() int {
	if p.pid <= 0 {
		return 1
	}
	return p.pid
}

func (p restartProcessStub) Wait() error {
	if p.wait != nil {
		return p.wait()
	}
	return nil
}

func envValue(sandbox []string, key string) string {
	prefix := key + "="
	for _, entry := range sandbox {
		if after, ok := strings.CutPrefix(entry, prefix); ok {
			return after
		}
	}
	return ""
}

func TestWithRestartOperationEnvReplacesAndAppends(t *testing.T) {
	t.Parallel()

	updated := withRestartOperationEnv([]string{"A=1", RestartOperationEnvKey + "=old"}, "new")
	if got, want := envValue(updated, RestartOperationEnvKey), "new"; got != want {
		t.Fatalf("envValue(updated) = %q, want %q", got, want)
	}

	appended := withRestartOperationEnv([]string{"A=1"}, "new")
	if got, want := envValue(appended, RestartOperationEnvKey), "new"; got != want {
		t.Fatalf("envValue(appended) = %q, want %q", got, want)
	}
}

func sequentialTime(values []time.Time) func() time.Time {
	var index atomic.Int64
	return func() time.Time {
		i := max(int(index.Add(1))-1, 0)
		if i >= len(values) {
			return values[len(values)-1]
		}
		return values[i]
	}
}

func TestWithTimeoutCapHonorsEarlierDeadlineAndShortensLongerParent(t *testing.T) {
	t.Parallel()

	baseCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	sameCtx, sameCancel := withTimeoutCap(baseCtx, time.Second)
	defer sameCancel()
	if sameCtx != baseCtx {
		t.Fatal("withTimeoutCap(later timeout) returned a different context")
	}

	shortenedCtx, shortenedCancel := withTimeoutCap(baseCtx, 10*time.Millisecond)
	defer shortenedCancel()
	if shortenedCtx == baseCtx {
		t.Fatal("withTimeoutCap(shorter timeout) returned the parent context")
	}
	shortenedDeadline, ok := shortenedCtx.Deadline()
	if !ok {
		t.Fatal("withTimeoutCap(shorter timeout) did not add a deadline")
	}
	parentDeadline, ok := baseCtx.Deadline()
	if !ok {
		t.Fatal("baseCtx.Deadline() missing")
	}
	if !shortenedDeadline.Before(parentDeadline) {
		t.Fatalf("shortened deadline %v is not earlier than parent deadline %v", shortenedDeadline, parentDeadline)
	}

	derivedCtx, derivedCancel := withTimeoutCap(context.Background(), 50*time.Millisecond)
	defer derivedCancel()
	deadline, ok := derivedCtx.Deadline()
	if !ok {
		t.Fatal("withTimeoutCap(background) did not add a deadline")
	}
	if time.Until(deadline) <= 0 {
		t.Fatalf("derived deadline %v is not in the future", deadline)
	}
}

func TestRunRelaunchHelperReplacementFailurePersistsFailedOperation(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-helper-failure",
		Status:             RestartStatusPending,
		OldPID:             4242,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 4,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}

	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: 500 * time.Millisecond,
		ReadyTimeout:   500 * time.Millisecond,
	})
	helper.processAlive = func(int) bool { return false }
	helper.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		return restartProcessStub{
			pid: 9393,
			wait: func() error {
				return errors.New("replacement boot failed")
			},
		}, nil
	}

	if err := helper.run(testutil.Context(t)); err == nil || !errors.Is(err, errReplacementDaemonExitedBeforeReady) {
		t.Fatalf("helper.run() error = %v, want replacement-daemon exit sentinel", err)
	}

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusFailed; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
	if got := persisted.NewPID; got != 0 {
		t.Fatalf("persisted.NewPID = %d, want 0 when replacement never became ready", got)
	}
	if !strings.Contains(persisted.FailureReason, "replacement boot failed") {
		t.Fatalf("persisted.FailureReason = %q, want replacement failure", persisted.FailureReason)
	}
}

func TestRunRelaunchHelperWrapperUsesDefaultLauncherAndPersistsFailure(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-wrapper-defaults",
		Status:             RestartStatusPending,
		OldPID:             999999,
		OldStartedAt:       time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		OldSocketPath:      homePaths.DaemonSocket,
		ActiveSessionCount: 0,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		t.Fatalf("Transition(stopping) error = %v", err)
	}

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "agh-helper.sh")
	script := "#!/bin/sh\nexit 0\n"
	mode := os.FileMode(0o755)
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(dir, "agh-helper.cmd")
		script = "@echo off\r\nexit /b 0\r\n"
		mode = 0o600
	}
	if err := os.WriteFile(scriptPath, []byte(script), mode); err != nil {
		t.Fatalf("os.WriteFile(script) error = %v", err)
	}

	err = RunRelaunchHelper(testutil.Context(t), RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		Executable:     func() (string, error) { return scriptPath, nil },
		Sandbox:        []string{"PATH=" + os.Getenv("PATH")},
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: 200 * time.Millisecond,
		ReadyTimeout:   200 * time.Millisecond,
	})
	if err == nil || !errors.Is(err, errReplacementDaemonExitedBeforeReady) {
		t.Fatalf("RunRelaunchHelper() error = %v, want replacement-daemon exit sentinel", err)
	}

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusFailed; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
}
