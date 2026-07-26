//go:build integration

package daemon

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

func TestRequestRestartPersistsPreRestartContextBeforeShutdownSignal(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.config = cfg
	d.startedAt = time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	d.pid = func() int { return 5151 }
	d.sessions = &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}, {ID: "sess-b"}},
	}
	d.executable = func() (string, error) { return "/usr/bin/agh", nil }

	var helperRequest detachedStartRequest
	d.startDetached = func(_ context.Context, req detachedStartRequest) (restartProcess, error) {
		helperRequest = req
		return restartProcessStub{pid: 9001}, nil
	}

	d.signalProcess = func(pid int, sig syscall.Signal) error {
		if got, want := pid, 5151; got != want {
			t.Fatalf("signal pid = %d, want %d", got, want)
		}
		if got, want := sig, syscall.SIGTERM; got != want {
			t.Fatalf("signal = %v, want %v", got, want)
		}

		store := newRestartStore(homePaths, d.now)
		operationID := envValue(helperRequest.sandbox, RestartOperationEnvKey)
		persisted, err := store.Get(operationID)
		if err != nil {
			t.Fatalf("store.Get(%q) error = %v", operationID, err)
		}
		if got, want := persisted.Status, RestartStatusStopping; got != want {
			t.Fatalf("persisted.Status = %q, want %q", got, want)
		}
		if got, want := persisted.OldPID, 5151; got != want {
			t.Fatalf("persisted.OldPID = %d, want %d", got, want)
		}
		if got, want := persisted.ActiveSessionCount, 2; got != want {
			t.Fatalf("persisted.ActiveSessionCount = %d, want %d", got, want)
		}
		return nil
	}

	operation, err := d.RequestRestart(testutil.Context(t))
	if err != nil {
		t.Fatalf("RequestRestart() error = %v", err)
	}
	if got, want := operation.Status, RestartStatusStopping; got != want {
		t.Fatalf("operation.Status = %q, want %q", got, want)
	}
}

func TestRelaunchHelperFailurePersistsAfterOldDaemonExit(t *testing.T) {
	homePaths := integrationHomePaths(t)
	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-helper-integration",
		Status:             RestartStatusPending,
		OldPID:             5151,
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

	lock, err := AcquireLock(homePaths.DaemonLock, operation.OldPID)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
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

	helper := newRelaunchHelper(RelaunchHelperConfig{
		HomePaths:      homePaths,
		OperationID:    operation.OperationID,
		PollInterval:   10 * time.Millisecond,
		ReleaseTimeout: time.Second,
		ReadyTimeout:   time.Second,
	})
	helper.processAlive = func(int) bool { return oldAlive.Load() }
	helper.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
		return restartProcessStub{
			pid: 9393,
			wait: func() error {
				return errors.New("replacement boot failed after old daemon exit")
			},
		}, nil
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

	err = helper.run(testutil.Context(t))
	if err == nil || !errors.Is(err, errReplacementDaemonExitedBeforeReady) {
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
		t.Fatalf("persisted.NewPID = %d, want 0 after failed replacement boot", got)
	}
}

func TestBootMarksRestartOperationReadyAfterFreshDaemonInfo(t *testing.T) {
	homePaths := integrationHomePaths(t)
	cfg := testConfig(t, homePaths)

	store := newRestartStore(homePaths, sequentialTime([]time.Time{
		time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 2, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 3, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 4, 0, 0, time.UTC),
	}))
	operation, err := store.Create(RestartOperation{
		OperationID:        "restart-ready-integration",
		Status:             RestartStatusPending,
		OldPID:             5151,
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

	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.pid = func() int { return 9393 }
	d.getenv = func(key string) string {
		if key == RestartOperationEnvKey {
			return operation.OperationID
		}
		if key == "HOME" {
			return homePaths.HomeDir
		}
		return os.Getenv(key)
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	persisted, err := store.Get(operation.OperationID)
	if err != nil {
		t.Fatalf("store.Get() error = %v", err)
	}
	if got, want := persisted.Status, RestartStatusReady; got != want {
		t.Fatalf("persisted.Status = %q, want %q", got, want)
	}
	if got, want := persisted.NewPID, 9393; got != want {
		t.Fatalf("persisted.NewPID = %d, want %d", got, want)
	}
}
