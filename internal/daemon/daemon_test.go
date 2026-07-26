package daemon

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	devcycle "github.com/compozy/agh/extensions/dev-cycle"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/doctor"
	eventspkg "github.com/compozy/agh/internal/events"
	extensionpkg "github.com/compozy/agh/internal/extension"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/memory/consolidation"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/skills"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/subprocess"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gofrs/flock"
	"go.uber.org/goleak"
)

func TestAcquireLockSucceedsWithoutExistingLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")

	lock, err := AcquireLock(lockPath, os.Getpid())
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lock.Release(); err != nil {
			t.Fatalf("lock.Release() error = %v", err)
		}
	})

	if got := lock.StalePID(); got != 0 {
		t.Fatalf("lock.StalePID() = %d, want 0", got)
	}

	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("os.ReadFile(lock) error = %v", err)
	}
	if got, want := strings.TrimSpace(string(data)), strconvString(os.Getpid()); got != want {
		t.Fatalf("lock file contents = %q, want %q", got, want)
	}
}

func TestRuntimeMemoryMonitor(t *testing.T) {
	t.Run("Should report baseline periodic and joined shutdown snapshots", func(t *testing.T) {
		// not parallel: UT-061 compares the process-wide goroutine count around shutdown.
		baselineGoroutines := runtime.NumGoroutine()

		startedAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
		now := startedAt
		ticker := &runtimeMemoryManualTicker{ticks: make(chan time.Time, 1)}
		reports := make(chan doctor.RuntimeMemorySnapshot, 3)
		monitor := newRuntimeMemoryMonitor(
			time.Minute,
			startedAt,
			slog.Default(),
			func(options *runtimeMemoryMonitorOptions) {
				options.now = func() time.Time { return now }
				options.residentMemory = func() (uint64, string, error) { return 8192, "current", nil }
				options.newTicker = func(time.Duration) runtimeMemoryTicker { return ticker }
				options.report = func(snapshot doctor.RuntimeMemorySnapshot) { reports <- snapshot }
			},
		)
		if err := monitor.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		baseline := receiveRuntimeMemorySnapshot(t, reports)
		if baseline.Phase != runtimeMemoryPhaseBaseline || !baseline.Active || baseline.ResidentMemoryBytes != 8192 {
			t.Fatalf("baseline snapshot = %#v, want active populated baseline", baseline)
		}

		now = startedAt.Add(time.Minute)
		ticker.ticks <- now
		periodic := receiveRuntimeMemorySnapshot(t, reports)
		if periodic.Phase != runtimeMemoryPhasePeriodic || periodic.UptimeSeconds != 60 || periodic.Goroutines <= 0 {
			t.Fatalf("periodic snapshot = %#v, want one-minute populated sample", periodic)
		}

		now = startedAt.Add(2 * time.Minute)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := monitor.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		shutdown := receiveRuntimeMemorySnapshot(t, reports)
		if shutdown.Phase != runtimeMemoryPhaseShutdown || shutdown.Active || shutdown.UptimeSeconds != 120 {
			t.Fatalf("shutdown snapshot = %#v, want inactive joined snapshot", shutdown)
		}
		if !ticker.stopped {
			t.Fatal("Shutdown() ticker stopped = false, want true")
		}

		deadline := time.Now().Add(time.Second)
		for runtime.NumGoroutine() > baselineGoroutines && time.Now().Before(deadline) {
			runtime.Gosched()
		}
		if got := runtime.NumGoroutine(); got > baselineGoroutines {
			t.Fatalf("goroutines after shutdown = %d, want at most baseline %d", got, baselineGoroutines)
		}
	})

	t.Run("Should disable sampling without starting a ticker", func(t *testing.T) {
		t.Parallel()

		tickerStarted := false
		monitor := newRuntimeMemoryMonitor(
			0,
			time.Now(),
			slog.Default(),
			func(options *runtimeMemoryMonitorOptions) {
				options.newTicker = func(time.Duration) runtimeMemoryTicker {
					tickerStarted = true
					return &runtimeMemoryManualTicker{ticks: make(chan time.Time)}
				}
			},
		)
		if err := monitor.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if tickerStarted {
			t.Fatal("Start() created a ticker for interval zero")
		}
		snapshot := monitor.RuntimeMemorySnapshot()
		if snapshot.Enabled || snapshot.Active || snapshot.Phase != runtimeMemoryPhaseDisabled {
			t.Fatalf("RuntimeMemorySnapshot() = %#v, want deterministic disabled state", snapshot)
		}
		if err := monitor.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
}

type runtimeMemoryManualTicker struct {
	ticks   chan time.Time
	stopped bool
}

func (t *runtimeMemoryManualTicker) C() <-chan time.Time { return t.ticks }

func (t *runtimeMemoryManualTicker) Stop() { t.stopped = true }

func receiveRuntimeMemorySnapshot(
	t *testing.T,
	reports <-chan doctor.RuntimeMemorySnapshot,
) doctor.RuntimeMemorySnapshot {
	t.Helper()
	select {
	case snapshot := <-reports:
		return snapshot
	case <-time.After(time.Second):
		t.Fatal("runtime memory snapshot was not reported")
		return doctor.RuntimeMemorySnapshot{}
	}
}

func TestAcquireLockFailsWhenAnotherDaemonHoldsTheLock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")

	first, err := AcquireLock(lockPath, os.Getpid())
	if err != nil {
		t.Fatalf("AcquireLock(first) error = %v", err)
	}
	t.Cleanup(func() {
		if err := first.Release(); err != nil {
			t.Fatalf("first.Release() error = %v", err)
		}
	})

	second, err := AcquireLock(lockPath, os.Getpid())
	if second != nil {
		t.Fatalf("AcquireLock(second) lock = %#v, want nil", second)
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("AcquireLock(second) error = %v, want ErrAlreadyRunning", err)
	}
}

func TestAcquireLockReclaimsStalePID(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "daemon.lock")
	if err := os.WriteFile(lockPath, []byte("999999\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(lock) error = %v", err)
	}

	lock, err := acquireLock(lockPath, 1234, lockDeps{
		newFlock:     func(path string) *flock.Flock { return flock.New(path) },
		processAlive: func(_ int) bool { return false },
	})
	if err != nil {
		t.Fatalf("acquireLock() error = %v", err)
	}
	t.Cleanup(func() {
		if err := lock.Release(); err != nil {
			t.Fatalf("lock.Release() error = %v", err)
		}
	})

	if got, want := lock.StalePID(), 999999; got != want {
		t.Fatalf("lock.StalePID() = %d, want %d", got, want)
	}
}

func TestInfoWriteReadAndRemoveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.json")
	now := time.Date(2026, 4, 3, 12, 30, 0, 0, time.UTC)
	info := Info{
		PID:       4242,
		Port:      2123,
		StartedAt: now,
		Network: &NetworkInfo{
			Enabled: true,
			Status:  network.StatusActive,
		},
	}

	if err := WriteInfo(path, info); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	got, err := ReadInfo(path)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if got.PID != info.PID || got.Port != info.Port || !got.StartedAt.Equal(info.StartedAt) {
		t.Fatalf("ReadInfo() = %#v, want %#v", got, info)
	}

	if err := RemoveInfo(path); err != nil {
		t.Fatalf("RemoveInfo() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("daemon.json exists after RemoveInfo(): stat error = %v, want os.ErrNotExist", err)
	}
}

func TestBootWithNetworkDisabledKeepsDaemonOperational(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = false
	registry := &recordingRegistry{path: homePaths.DatabaseFile}
	ownerSawDisabledAvailability := false

	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return registry, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		availability, err := registry.GetNetworkAvailability(testutil.Context(t))
		if err != nil {
			t.Fatalf("GetNetworkAvailability() before session owner construction error = %v", err)
		}
		ownerSawDisabledAvailability = !availability.Enabled &&
			availability.UpdatedBy == "config.reload.boot"
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.network == nil {
		t.Fatal("boot() network runtime = nil, want paused manager when disabled")
	}
	if d.networkWakeRunner == nil {
		t.Fatal("boot() network wake runner = nil, want paused runner when disabled")
	}
	if d.info.Network == nil {
		t.Fatal("boot() daemon info network = nil, want disabled diagnostics")
	}
	if d.info.Network.Enabled {
		t.Fatalf("boot() daemon info network enabled = %v, want false", d.info.Network.Enabled)
	}
	if got, want := d.info.Network.Status, network.StatusDisabled; got != want {
		t.Fatalf("boot() daemon info network status = %q, want %q", got, want)
	}
	if !ownerSawDisabledAvailability {
		t.Fatal("session owner constructed before disabled network availability was persisted")
	}
	if got, want := registry.networkAvailabilityWrites, []bool{false}; !slices.Equal(got, want) {
		t.Fatalf("network availability boot writes = %#v, want %#v", got, want)
	}
}

func TestBootWithRegistryMissingResourceDBLeavesResourceServiceUnavailable(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = false

	var httpSawNilResources bool
	var udsSawNilResources bool

	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
		httpSawNilResources = deps.Resources == nil
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
		udsSawNilResources = deps.Resources == nil
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if !httpSawNilResources {
		t.Fatal("httpFactory() received non-nil resource service, want nil when registry has no SQL handle")
	}
	if !udsSawNilResources {
		t.Fatal("udsFactory() received non-nil resource service, want nil when registry has no SQL handle")
	}
}

func TestBootRunsResourceReconcileBeforeObserverReconcile(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = false

	var mu sync.Mutex
	var order []string
	appendOrder := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	driver := &fakeResourceReconcileDriver{
		onRunBoot: func() {
			appendOrder("driver")
		},
	}

	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{
			onReconcile: func() {
				appendOrder("observer")
			},
		}, nil
	}
	d.newResourceReconcile = func(context.Context, resourceReconcileDriverDeps) (resources.ReconcileDriver, error) {
		return driver, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	mu.Lock()
	gotOrder := append([]string(nil), order...)
	mu.Unlock()
	wantOrder := []string{"driver", "observer"}
	if !slices.Equal(gotOrder, wantOrder) {
		t.Fatalf("boot order = %#v, want %#v", gotOrder, wantOrder)
	}
}

func TestBootRunsObserverReconcileBeforeTaskRoleRecovery(t *testing.T) {
	t.Run("Should fence recovered task activation until session reconciliation finishes", func(t *testing.T) {
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		cfg.Network.Enabled = false

		var mu sync.Mutex
		observerReconciled := false
		queuedRecoveryObserved := false
		queuedRecoveryBeforeObserver := false
		registry := &recordingRegistry{
			path: homePaths.DatabaseFile,
			onListTaskRunsByStatus: func(statuses []taskpkg.RunStatus) {
				if !slices.Equal(statuses, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued}) {
					return
				}
				mu.Lock()
				defer mu.Unlock()
				queuedRecoveryObserved = true
				queuedRecoveryBeforeObserver = queuedRecoveryBeforeObserver || !observerReconciled
			},
		}

		d := newTestDaemon(t, homePaths, &cfg)
		d.openRegistry = func(context.Context, string) (Registry, error) {
			return registry, nil
		}
		d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{onReconcile: func() {
				mu.Lock()
				observerReconciled = true
				mu.Unlock()
			}}, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}

		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})

		mu.Lock()
		observed := queuedRecoveryObserved
		beforeObserver := queuedRecoveryBeforeObserver
		mu.Unlock()
		if !observed {
			t.Fatal("boot() did not run task-role recovery")
		}
		if beforeObserver {
			t.Fatal("boot() ran task-role recovery before observer reconciliation")
		}
	})
}

func TestLoopCoordinatorBootGate(t *testing.T) {
	t.Run("Should suppress observer and scheduler activation until reconciliation finishes", func(t *testing.T) {
		t.Parallel()

		backstop := &recordingLoopBackstopRunner{}
		gate := newLoopCoordinatorBootGate(backstop)
		actor, err := taskpkg.DeriveDaemonActorContext("boot-gate-test", "daemon.test")
		if err != nil {
			t.Fatalf("DeriveDaemonActorContext() error = %v", err)
		}
		now := time.Date(2026, 7, 11, 15, 0, 0, 0, time.UTC)

		started, err := gate.RunLoopCoordinatorBackstop(testutil.Context(t), now, actor)
		if err != nil {
			t.Fatalf("observer backstop before activation error = %v", err)
		}
		if started != 0 || len(backstop.calls) != 0 {
			t.Fatalf("observer backstop before activation = (%d, %d calls), want (0, 0)", started, len(backstop.calls))
		}

		schedulerSource := schedulerTaskSource{coordinatorBackstop: gate}
		started, err = schedulerSource.RunLoopCoordinatorBackstop(testutil.Context(t), now, actor)
		if err != nil {
			t.Fatalf("scheduler backstop before activation error = %v", err)
		}
		if started != 0 || len(backstop.calls) != 0 {
			t.Fatalf("scheduler backstop before activation = (%d, %d calls), want (0, 0)", started, len(backstop.calls))
		}

		gate.Activate()
		started, err = schedulerSource.RunLoopCoordinatorBackstop(testutil.Context(t), now, actor)
		if err != nil {
			t.Fatalf("scheduler backstop after activation error = %v", err)
		}
		if started != 1 || len(backstop.calls) != 1 {
			t.Fatalf("scheduler backstop after activation = (%d, %d calls), want (1, 1)", started, len(backstop.calls))
		}
	})
}

func TestShutdownClosesResourceReconcileDriver(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = false

	driver := &fakeResourceReconcileDriver{}
	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.newResourceReconcile = func(context.Context, resourceReconcileDriverDeps) (resources.ReconcileDriver, error) {
		return driver, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if got, want := driver.closeCalls, 1; got != want {
		t.Fatalf("resource reconcile Close() calls = %d, want %d", got, want)
	}
}

func TestBootRegistersOperatorHomeAsDefaultWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should register operator home before serving workspaces", func(t *testing.T) {
		t.Parallel()

		operatorHome := filepath.Join(t.TempDir(), "operator-home")
		if err := os.MkdirAll(operatorHome, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", operatorHome, err)
		}
		homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(operatorHome, aghconfig.DirName))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		homePaths.DaemonSocket = shortSocketPath(t)
		if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		cfg := testConfig(t, homePaths)
		cfg.Network.Enabled = false

		d := newTestDaemon(t, homePaths, &cfg)
		d.getenv = func(key string) string {
			if key == "HOME" {
				return operatorHome
			}
			return os.Getenv(key)
		}
		d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{}, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}

		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})

		wantRoot := canonicalDaemonRoot(t, operatorHome)
		workspaces, err := d.registry.ListWorkspaces(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListWorkspaces() error = %v", err)
		}
		if len(workspaces) != 1 {
			t.Fatalf("ListWorkspaces() = %#v, want one default workspace", workspaces)
		}
		if got := workspaces[0].RootDir; got != wantRoot {
			t.Fatalf("default workspace root = %q, want operator home %q", got, wantRoot)
		}
		if got := workspaces[0].RootDir; got == homePaths.HomeDir {
			t.Fatalf("default workspace root = AGH home %q, want operator home %q", got, wantRoot)
		}

		resolved, err := d.workspaceResolver.Resolve(testutil.Context(t), operatorHome)
		if err != nil {
			t.Fatalf("Resolve(operator home) error = %v", err)
		}
		if resolved.ID != workspaces[0].ID {
			t.Fatalf("Resolve(operator home) ID = %q, want %q", resolved.ID, workspaces[0].ID)
		}

		again, err := d.workspaceResolver.ResolveOrRegister(testutil.Context(t), operatorHome)
		if err != nil {
			t.Fatalf("ResolveOrRegister(operator home) error = %v", err)
		}
		if again.ID != workspaces[0].ID {
			t.Fatalf("ResolveOrRegister(operator home) ID = %q, want %q", again.ID, workspaces[0].ID)
		}
		after, err := d.registry.ListWorkspaces(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListWorkspaces(after idempotent resolve) error = %v", err)
		}
		if len(after) != 1 {
			t.Fatalf("ListWorkspaces(after idempotent resolve) = %#v, want one workspace", after)
		}
	})
}

func TestBootEnabledNetworkLateBindsSessionCallbacksAndPersistsSafeDiagnostics(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	if !cfg.Network.Enabled {
		t.Fatal("testConfig() Network.Enabled = false, want true by default")
	}

	bindableSessions := newFakeNetworkBindableSessionManager()
	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return bindableSessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.network == nil {
		t.Fatal("boot() network runtime = nil, want initialized manager")
	}
	if bindableSessions.currentNetworkPeerLifecycle() == nil {
		t.Fatal("boot() did not late-bind network peer lifecycle")
	}
	if bindableSessions.currentTurnEndNotifier() == nil {
		t.Fatal("boot() did not late-bind turn-end notifier")
	}

	info, err := ReadInfo(homePaths.DaemonInfo)
	if err != nil {
		t.Fatalf("ReadInfo(daemon.json) error = %v", err)
	}
	if info.Network == nil {
		t.Fatal("daemon info network diagnostics = nil, want populated diagnostics")
	}
	if !info.Network.Enabled {
		t.Fatal("daemon info network enabled = false, want true")
	}
	if got, want := info.Network.Status, network.StatusReady; got != want {
		t.Fatalf("daemon info network status = %q, want %q", got, want)
	}

	rawInfo, err := os.ReadFile(homePaths.DaemonInfo)
	if err != nil {
		t.Fatalf("os.ReadFile(daemon.json) error = %v", err)
	}
	if strings.Contains(strings.ToLower(string(rawInfo)), "token") {
		t.Fatalf("daemon info leaked credentials: %s", string(rawInfo))
	}
}

func TestBootEnabledNetworkRejectsSessionManagersMissingBindingSurface(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Network.Enabled = true

	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		base := &fakeSessionManager{}
		return nonBindableHarnessSessionManager{
			SessionManager:    base,
			syntheticPrompter: base,
		}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}

	err := d.boot(testutil.Context(t))
	if !errors.Is(err, errMissingNetworkBindingSurface) {
		t.Fatalf("boot() error = %v, want errMissingNetworkBindingSurface", err)
	}
}

func TestBootRejectsSessionManagersMissingWorkspaceRemovalPreparation(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessionManagerWithoutWorkspaceRemoval{SessionManager: &fakeSessionManager{}}, nil
	}

	err := d.boot(testutil.Context(t))
	if !errors.Is(err, errMissingWorkspaceRemovalPreparation) {
		t.Fatalf("boot() error = %v, want %v", err, errMissingWorkspaceRemovalPreparation)
	}
}

func TestBootRemovesStaleSocketAndCleansOrphans(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	staleSocket := cfg.Daemon.Socket
	if err := os.WriteFile(staleSocket, []byte("stale"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(socket) error = %v", err)
	}

	d := newTestDaemon(t, homePaths, &cfg)
	d.pid = func() int { return 777 }
	d.acquireLock = func(path string, _ int) (*Lock, error) {
		return &Lock{path: path, stalePID: 444}, nil
	}

	registry := &recordingRegistry{path: homePaths.DatabaseFile}
	observer := &fakeObserver{result: store.ReconcileResult{Indexed: []string{"sess-a"}}}
	sessionManager := &fakeSessionManager{}
	var signals []string
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return registry, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessionManager, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return observer, nil
	}
	d.listProcesses = func(context.Context) ([]processInfo, error) {
		return []processInfo{{PID: 1001, PPID: 444}, {PID: 2002, PPID: 111}}, nil
	}
	d.orphanGraceWait = 2 * time.Millisecond
	d.orphanPollWait = time.Millisecond
	d.signalProcess = func(pid int, sig syscall.Signal) error {
		signals = append(signals, sig.String()+":"+strconvString(pid))
		return nil
	}
	d.processAlive = func(pid int) bool { return pid == 1001 }

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	socketInfo, err := os.Lstat(staleSocket)
	if err != nil {
		t.Fatalf("os.Lstat(socket) error = %v", err)
	}
	if socketInfo.Mode()&os.ModeSocket == 0 {
		t.Fatalf("socket mode = %v, want unix socket", socketInfo.Mode())
	}
	if !observer.reconciled {
		t.Fatal("boot() did not call observer.Reconcile")
	}
	if got, want := signals, []string{"terminated:1001", "killed:1001"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("cleanup orphan signals = %#v, want %#v", got, want)
	}

	info, err := ReadInfo(homePaths.DaemonInfo)
	if err != nil {
		t.Fatalf("ReadInfo(daemon.json) error = %v", err)
	}
	if got, want := info.PID, 777; got != want {
		t.Fatalf("daemon info pid = %d, want %d", got, want)
	}
}

func TestCleanupOrphansAllowsGracefulExitBeforeSIGKILL(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)

	var (
		signals   []string
		aliveCall int
	)
	d.listProcesses = func(context.Context) ([]processInfo, error) {
		return []processInfo{{PID: 1001, PPID: 444}}, nil
	}
	d.orphanGraceWait = 10 * time.Millisecond
	d.orphanPollWait = time.Millisecond
	d.signalProcess = func(pid int, sig syscall.Signal) error {
		signals = append(signals, sig.String()+":"+strconvString(pid))
		return nil
	}
	d.processAlive = func(_ int) bool {
		aliveCall++
		return aliveCall == 1
	}

	if err := d.cleanupOrphans(testutil.Context(t), 444); err != nil {
		t.Fatalf("cleanupOrphans() error = %v", err)
	}
	if got, want := signals, []string{"terminated:1001"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("cleanup orphan signals = %#v, want %#v", got, want)
	}
}

func TestBootRejectsConcurrentCallWhileFirstBootIsInProgress(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)

	loadStarted := make(chan struct{})
	releaseLoad := make(chan struct{})
	d.loadConfig = func() (aghconfig.Config, error) {
		close(loadStarted)
		<-releaseLoad
		return cfg, nil
	}
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	firstBoot := make(chan error, 1)
	go func() {
		firstBoot <- d.boot(testutil.Context(t))
	}()

	<-loadStarted
	if err := d.boot(testutil.Context(t)); err == nil || !strings.Contains(err.Error(), "already booted") {
		t.Fatalf("concurrent boot error = %v, want already booted", err)
	}

	close(releaseLoad)
	if err := <-firstBoot; err != nil {
		t.Fatalf("first boot error = %v", err)
	}
	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestShutdownTearsDownInRequiredOrder(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)

	var events []string
	d.extensions = &fakeExtensionRuntime{
		onStop: func() {
			events = append(events, "extensions")
		},
	}
	d.automation = &fakeAutomationManager{
		onShutdown: func() {
			events = append(events, "automation")
		},
	}
	d.sessions = &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}, {ID: "sess-b"}},
		onStop: func(id string) {
			events = append(events, "session:"+id)
		},
	}
	d.tasks = &taskRuntime{}
	d.network = &fakeNetworkRuntime{
		onShutdown: func() {
			events = append(events, "network")
		},
	}
	d.httpServer = &fakeServer{name: "http", onShutdown: func() { events = append(events, "http") }}
	d.udsServer = &fakeServer{name: "uds", onShutdown: func() { events = append(events, "uds") }}
	d.registry = &recordingRegistry{
		path: homePaths.DatabaseFile,
		onClose: func() {
			events = append(events, "db")
		},
	}
	d.hooks = &fakeHookRuntime{
		onClose: func() {
			events = append(events, "hooks")
		},
	}
	d.lock = &Lock{
		path: homePaths.DaemonLock,
		releaseFn: func() error {
			events = append(events, "lock")
			return nil
		},
	}
	d.closeLogger = func() error {
		events = append(events, "logger")
		return nil
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if d.tasks != nil {
		t.Fatalf("Shutdown() left task runtime = %#v, want nil", d.tasks)
	}

	want := []string{
		"extensions",
		"automation",
		"session:sess-a",
		"session:sess-b",
		"http",
		"uds",
		"network",
		"hooks",
		"db",
		"lock",
		"logger",
	}
	if !testutil.EqualStringSlices(events, want) {
		t.Fatalf("Shutdown() order = %#v, want %#v", events, want)
	}
}

func TestBootExtensionsBuildsManagerWhenNoExtensionsInstalled(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))

	runtime := &fakeExtensionRuntime{}
	var managerBuilt bool
	d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
		managerBuilt = true
		return runtime
	}

	rebuilds := 0
	state := &bootState{
		logger:   discardLogger(),
		registry: db,
		sessions: &fakeSessionManager{},
		observer: &fakeObserver{},
		hooks: &fakeHookRuntime{
			onRebuild: func(context.Context) error {
				rebuilds++
				return nil
			},
		},
	}
	cleanup := &bootCleanup{}

	if err := d.bootExtensions(testutil.Context(t), state, cleanup); err != nil {
		t.Fatalf("bootExtensions() error = %v", err)
	}

	if !managerBuilt {
		t.Fatal("bootExtensions() did not build an extension manager")
	}
	if runtime.startCount != 1 {
		t.Fatalf("extension runtime start count = %d, want 1", runtime.startCount)
	}
	if rebuilds != 1 {
		t.Fatalf("hook rebuild count = %d, want 1", rebuilds)
	}
	if state.extensions != runtime {
		t.Fatalf("state.extensions = %#v, want runtime", state.extensions)
	}
	if state.deps.Extensions == nil {
		t.Fatal("state.deps.Extensions = nil, want extension service")
	}
	if len(cleanup.fns) != 1 {
		t.Fatalf("cleanup fns = %d, want 1", len(cleanup.fns))
	}
	extRegistry := extensionpkg.NewRegistry(db.DB())
	info, err := extRegistry.Get(devcycle.Name)
	if err != nil {
		t.Fatalf("registry.Get(%s) error = %v", devcycle.Name, err)
	}
	if !info.Enabled || info.Source != extensionpkg.SourceBundled {
		t.Fatalf("dev-cycle info = %#v, want enabled bundled extension", info)
	}
	if got, want := filepath.Base(info.ManifestPath), "extension.json"; got != want {
		t.Fatalf("dev-cycle manifest file = %q, want %q", got, want)
	}
}

func TestBootExtensionsPreservesDevCycleDisableEnableState(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve dev-cycle disable and enable state across boots", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))
		runtime := &fakeExtensionRuntime{}
		d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
			return runtime
		}
		state := &bootState{
			logger:   discardLogger(),
			registry: db,
			sessions: &fakeSessionManager{},
			observer: &fakeObserver{},
			hooks:    &fakeHookRuntime{},
		}

		if err := d.bootExtensions(testutil.Context(t), state, &bootCleanup{}); err != nil {
			t.Fatalf("bootExtensions(first) error = %v", err)
		}
		extRegistry := extensionpkg.NewRegistry(db.DB())
		if err := extRegistry.Disable(devcycle.Name); err != nil {
			t.Fatalf("registry.Disable(%s) error = %v", devcycle.Name, err)
		}
		if err := d.bootExtensions(testutil.Context(t), state, &bootCleanup{}); err != nil {
			t.Fatalf("bootExtensions(after disable) error = %v", err)
		}
		disabled, err := extRegistry.Get(devcycle.Name)
		if err != nil {
			t.Fatalf("registry.Get(%s disabled) error = %v", devcycle.Name, err)
		}
		if disabled.Enabled {
			t.Fatalf("dev-cycle enabled after disabled boot = true, want false")
		}

		if err := extRegistry.Enable(devcycle.Name); err != nil {
			t.Fatalf("registry.Enable(%s) error = %v", devcycle.Name, err)
		}
		if err := d.bootExtensions(testutil.Context(t), state, &bootCleanup{}); err != nil {
			t.Fatalf("bootExtensions(after enable) error = %v", err)
		}
		enabled, err := extRegistry.Get(devcycle.Name)
		if err != nil {
			t.Fatalf("registry.Get(%s enabled) error = %v", devcycle.Name, err)
		}
		if !enabled.Enabled {
			t.Fatalf("dev-cycle enabled after enable boot = false, want true")
		}
	})
}

func TestNewHostAPISessionManagerAdapter(t *testing.T) {
	t.Parallel()

	t.Run("ShouldExposeBridgePromptMethodsWhenSourceSupportsThem", func(t *testing.T) {
		t.Parallel()

		source := newFakeNetworkBindableSessionManager()
		source.prompting["sess-bridge"] = true
		promptNetworkCalls := 0
		source.promptNetworkFn = func(_ context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
			promptNetworkCalls++
			if got, want := id, "sess-bridge"; got != want {
				t.Fatalf("PromptNetwork() id = %q, want %q", got, want)
			}
			if got, want := msg, "bridge message"; got != want {
				t.Fatalf("PromptNetwork() message = %q, want %q", got, want)
			}
			events := make(chan acp.AgentEvent)
			close(events)
			return events, nil
		}

		adapter := newHostAPISessionManagerAdapter(source)
		bridgePrompts, ok := adapter.(hostAPIBridgePromptSessionManager)
		if !ok {
			t.Fatalf("newHostAPISessionManagerAdapter() = %T, want bridge prompt methods", adapter)
		}
		if !bridgePrompts.IsPrompting("sess-bridge") {
			t.Fatal("IsPrompting(sess-bridge) = false, want forwarded true")
		}

		events, err := bridgePrompts.PromptNetwork(
			testutil.Context(t),
			"sess-bridge",
			"bridge message",
			acp.PromptNetworkMeta{MessageID: "msg-1", Kind: "message", From: "peer-1"},
		)
		if err != nil {
			t.Fatalf("PromptNetwork() error = %v", err)
		}
		for event := range events {
			t.Fatalf("PromptNetwork() emitted unexpected event %#v", event)
		}
		if got, want := promptNetworkCalls, 1; got != want {
			t.Fatalf("PromptNetwork() calls = %d, want %d", got, want)
		}
		if got := len(source.promptCalls); got != 0 {
			t.Fatalf("Prompt() fallback calls = %d, want 0", got)
		}

		promptOpts, ok := adapter.(hostAPIPromptOptsSessionManager)
		if !ok {
			t.Fatalf("newHostAPISessionManagerAdapter() = %T, want PromptWithOpts", adapter)
		}
		optsEvents, err := promptOpts.PromptWithOpts(
			testutil.Context(t),
			"sess-bridge",
			session.PromptOpts{
				Message:    "bridge opts message",
				TurnSource: session.TurnSourceNetwork,
				PromptMeta: acp.PromptMeta{
					Network: &acp.PromptNetworkMeta{MessageID: "msg-2", Kind: "message", From: "peer-1"},
				},
			},
		)
		if err != nil {
			t.Fatalf("PromptWithOpts() error = %v", err)
		}
		for event := range optsEvents {
			t.Fatalf("PromptWithOpts() emitted unexpected event %#v", event)
		}
		if got, want := source.promptWithOptsCalls, 1; got != want {
			t.Fatalf("PromptWithOpts() calls = %d, want %d", got, want)
		}
	})
}

func TestBootExtensionsBuildsManagerDepsAndRebuildsHooks(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	installDaemonTestExtension(t, db, "ext-present", daemonTestExtensionOptions{}, true)

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	memStore := memory.NewStore(t.TempDir())
	memProviders := extensionpkg.NewMemoryProviderRegistry()
	skillsRegistry := skills.NewRegistry(skills.RegistryConfig{})
	sessions := &fakeSessionManager{}
	observer := &fakeObserver{}
	logger := discardLogger()
	runtime := &fakeExtensionRuntime{}

	var captured extensionManagerDeps
	d := newTestDaemon(t, homePaths, &cfg)
	d.newExtensionManager = func(deps extensionManagerDeps) extensionRuntime {
		captured = deps
		return runtime
	}

	rebuilds := 0
	state := &bootState{
		logger:                 logger,
		registry:               db,
		memoryStore:            memStore,
		memoryProviderRegistry: memProviders,
		skillsRegistry:         skillsRegistry,
		sessions:               sessions,
		observer:               observer,
		hooks: &fakeHookRuntime{
			onRebuild: func(context.Context) error {
				rebuilds++
				return nil
			},
		},
	}
	cleanup := &bootCleanup{}

	if err := d.bootExtensions(testutil.Context(t), state, cleanup); err != nil {
		t.Fatalf("bootExtensions() error = %v", err)
	}

	if runtime.startCount != 1 {
		t.Fatalf("extension runtime start count = %d, want 1", runtime.startCount)
	}
	if rebuilds != 1 {
		t.Fatalf("hook rebuild count = %d, want 1", rebuilds)
	}
	if captured.Registry == nil {
		t.Fatal("captured extension registry = nil")
	}
	if captured.Sessions != sessions {
		t.Fatal("captured sessions dependency mismatch")
	}
	if captured.MemoryStore != memStore {
		t.Fatal("captured memory store dependency mismatch")
	}
	if captured.MemoryProviderRegistry != memProviders {
		t.Fatal("captured memory provider registry dependency mismatch")
	}
	if captured.Observer != observer {
		t.Fatal("captured observer dependency mismatch")
	}
	if captured.SkillsRegistry != skillsRegistry {
		t.Fatal("captured skills registry dependency mismatch")
	}
	if captured.Logger != logger {
		t.Fatal("captured logger dependency mismatch")
	}
	if state.extensions != runtime {
		t.Fatalf("state.extensions = %#v, want runtime", state.extensions)
	}
	if len(cleanup.fns) != 1 {
		t.Fatalf("cleanup fns = %d, want 1", len(cleanup.fns))
	}
}

func TestExtensionManagerDepsIncludeResourceHandlesAndTrigger(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	logger := discardLogger()
	memStore := memory.NewStore(t.TempDir())
	memProviders := extensionpkg.NewMemoryProviderRegistry()
	skillsRegistry := skills.NewRegistry(skills.RegistryConfig{})
	sessions := &fakeSessionManager{}
	observer := &fakeObserver{}
	automation := &fakeAutomationManager{}
	reconcile := &fakeResourceReconcileDriver{}
	bridges := &bridgeRuntime{broker: bridgepkg.NewBroker(nil)}
	codecs := resources.NewCodecRegistry()
	extRegistry := extensionpkg.NewRegistry(db.DB())

	d := newTestDaemon(t, homePaths, &cfg)
	deps := d.extensionManagerDeps(&bootState{
		cfg:                    cfg,
		logger:                 logger,
		sessions:               sessions,
		deps:                   RuntimeDeps{},
		memoryStore:            memStore,
		memoryProviderRegistry: memProviders,
		observer:               observer,
		skillsRegistry:         skillsRegistry,
		bridges:                bridges,
		resourceKernel:         kernel,
		resourceCodecs:         codecs,
		resourceReconcile:      reconcile,
		automation:             automation,
	}, extRegistry)

	if deps.Registry != extRegistry {
		t.Fatal("deps.Registry mismatch")
	}
	if deps.Sessions != sessions {
		t.Fatal("deps.Sessions mismatch")
	}
	if deps.MemoryStore != memStore {
		t.Fatal("deps.MemoryStore mismatch")
	}
	if deps.MemoryProviderRegistry != memProviders {
		t.Fatal("deps.MemoryProviderRegistry mismatch")
	}
	if deps.Observer != observer {
		t.Fatal("deps.Observer mismatch")
	}
	if deps.SkillsRegistry != skillsRegistry {
		t.Fatal("deps.SkillsRegistry mismatch")
	}
	if deps.Logger != logger {
		t.Fatal("deps.Logger mismatch")
	}
	if deps.ResourceCodecs != codecs {
		t.Fatal("deps.ResourceCodecs mismatch")
	}
	if got, ok := deps.ResourceStore.(*resources.Kernel); !ok || got != kernel {
		t.Fatalf("deps.ResourceStore = %#v, want kernel-backed raw store", deps.ResourceStore)
	}
	if got, ok := deps.SourceSessions.(*resources.Kernel); !ok || got != kernel {
		t.Fatalf("deps.SourceSessions = %#v, want kernel-backed source sessions", deps.SourceSessions)
	}
	if got := deps.Automation(); got != automation {
		t.Fatalf("deps.Automation() = %#v, want automation runtime", got)
	}
	if err := deps.ResourceTrigger(
		testutil.Context(t),
		hookBindingResourceKind,
		resources.ReconcileReasonWrite,
	); err != nil {
		t.Fatalf("deps.ResourceTrigger() error = %v", err)
	}
	if reconcile.triggerCalls != 1 {
		t.Fatalf("resource trigger calls = %d, want 1", reconcile.triggerCalls)
	}
	if reconcile.lastKind != hookBindingResourceKind || reconcile.lastReason != resources.ReconcileReasonWrite {
		t.Fatalf(
			"resource trigger = (%q, %q), want (%q, %q)",
			reconcile.lastKind,
			reconcile.lastReason,
			hookBindingResourceKind,
			resources.ReconcileReasonWrite,
		)
	}
}

func TestBootHooksBuildsResourceBackedRuntimeAndAttachesObserver(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	codec, err := newHookBindingCodec()
	if err != nil {
		t.Fatalf("newHookBindingCodec() error = %v", err)
	}
	codecs := resources.NewCodecRegistry()
	if err := resources.RegisterCodec(codecs, codec); err != nil {
		t.Fatalf("RegisterCodec() error = %v", err)
	}
	store, err := newHookBindingStore(kernel, codec)
	if err != nil {
		t.Fatalf("newHookBindingStore() error = %v", err)
	}

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	observer := &hookAwareTestObserver{}
	reconcile := &fakeResourceReconcileDriver{}
	d := newTestDaemon(t, homePaths, &cfg)
	participationResolver := &hookAwareParticipationResolver{}
	state := &bootState{
		cfg:    cfg,
		logger: discardLogger(),
		notifier: newHooksNotifier(
			discardLogger(),
			func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
		),
		observer:              observer,
		skillsRegistry:        skills.NewRegistry(skills.RegistryConfig{}),
		resourceKernel:        kernel,
		resourceCodecs:        codecs,
		resourceReconcile:     reconcile,
		participationResolver: participationResolver,
	}
	cleanup := &bootCleanup{}

	if err := d.bootHooks(testutil.Context(t), state, cleanup); err != nil {
		t.Fatalf("bootHooks() error = %v", err)
	}
	t.Cleanup(func() {
		for i, v := range slices.Backward(cleanup.fns) {
			if err := v(testutil.Context(t)); err != nil {
				t.Fatalf("cleanup[%d]() error = %v", i, err)
			}
		}
	})

	if observer.attached == nil {
		t.Fatal("observer attached hooks = nil, want runtime source")
	}
	if state.hooks == nil || state.hookDispatcher == nil || state.hookBindings == nil {
		t.Fatalf("hook state = %#v, want populated runtime, dispatcher, and bindings", state)
	}
	if participationResolver.hooks.Load() != state.hooks {
		t.Fatal("participation resolver hooks were not attached during hook boot")
	}
	if len(cleanup.fns) < 2 {
		t.Fatalf("cleanup fns = %d, want hook close plus skills watcher stop", len(cleanup.fns))
	}
	if reconcile.triggerCalls == 0 {
		t.Fatal("resource reconcile trigger calls = 0, want resource-backed hook sync")
	}

	records, err := store.List(testutil.Context(t), resources.MutationActor{
		Kind:     resources.MutationActorKindDaemon,
		ID:       "reader",
		Source:   resources.ResourceSource{Kind: resources.ResourceSourceKind("daemon"), ID: "reader"},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}, resources.ResourceFilter{})
	if err != nil {
		t.Fatalf("store.List() error = %v", err)
	}
	if len(records) == 0 {
		t.Fatal("store.List() count = 0, want native hook bindings")
	}
}

func TestAttachExtensionRuntimeUsesHookBindingSyncBeforeRebuild(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	extRegistry := extensionpkg.NewRegistry(db.DB())
	homePaths := testHomePaths(t)
	d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))
	manager := &fakeExtensionRuntime{}

	t.Run("Should syncs hook bindings when available", func(t *testing.T) {
		t.Parallel()

		syncCalls := 0
		rebuilds := 0
		state := &bootState{
			logger:       discardLogger(),
			hookBindings: hookBindingPublisherFunc(func(context.Context) error { syncCalls++; return nil }),
			hooks: &fakeHookRuntime{onRebuild: func(context.Context) error {
				rebuilds++
				return nil
			}},
		}

		d.attachExtensionRuntime(testutil.Context(t), state, extRegistry, manager)

		if syncCalls != 1 {
			t.Fatalf("hook binding sync calls = %d, want 1", syncCalls)
		}
		if rebuilds != 0 {
			t.Fatalf("hook rebuild count = %d, want 0", rebuilds)
		}
		if state.deps.Extensions == nil {
			t.Fatal("state.deps.Extensions = nil, want extension service")
		}
	})

	t.Run("Should falls back to rebuild without hook bindings", func(t *testing.T) {
		t.Parallel()

		rebuilds := 0
		state := &bootState{
			logger: discardLogger(),
			hooks: &fakeHookRuntime{onRebuild: func(context.Context) error {
				rebuilds++
				return nil
			}},
		}

		d.attachExtensionRuntime(testutil.Context(t), state, extRegistry, manager)

		if rebuilds != 1 {
			t.Fatalf("hook rebuild count = %d, want 1", rebuilds)
		}
		if state.deps.Extensions == nil {
			t.Fatal("state.deps.Extensions = nil, want extension service")
		}
	})

	t.Run("Should logs sync failures without rebuilding", func(t *testing.T) {
		t.Parallel()

		syncCalls := 0
		rebuilds := 0
		state := &bootState{
			logger: discardLogger(),
			hookBindings: hookBindingPublisherFunc(func(context.Context) error {
				syncCalls++
				return errors.New("boom")
			}),
			hooks: &fakeHookRuntime{onRebuild: func(context.Context) error {
				rebuilds++
				return nil
			}},
		}

		d.attachExtensionRuntime(testutil.Context(t), state, extRegistry, manager)

		if syncCalls != 1 {
			t.Fatalf("hook binding sync calls = %d, want 1", syncCalls)
		}
		if rebuilds != 0 {
			t.Fatalf("hook rebuild count = %d, want 0", rebuilds)
		}
	})
}

func TestNewDaemonExtensionServiceHandlesNilRegistryAndDefaults(t *testing.T) {
	t.Parallel()

	if svc := newDaemonExtensionService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		aghconfig.HomePaths{},
		nil,
		nil,
	); svc != nil {
		t.Fatalf("newDaemonExtensionService(nil) = %#v, want nil", svc)
	}

	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	if svc := newDaemonExtensionService(
		registry,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		aghconfig.HomePaths{},
		nil,
		nil,
	); svc == nil {
		t.Fatal("newDaemonExtensionService(defaults) = nil, want service")
	}
}

func TestBootExtensionsLogsStartFailureAndKeepsPartialRuntime(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	installDaemonTestExtension(t, db, "ext-broken", daemonTestExtensionOptions{}, true)

	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
	runtime := &fakeExtensionRuntime{startErr: errors.New("boom")}
	homePaths := testHomePaths(t)
	d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))
	d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
		return runtime
	}

	rebuilds := 0
	state := &bootState{
		logger:   logger,
		registry: db,
		sessions: &fakeSessionManager{},
		observer: &fakeObserver{},
		bridges:  &bridgeRuntime{broker: bridgepkg.NewBroker(nil)},
		hooks: &fakeHookRuntime{
			onRebuild: func(context.Context) error {
				rebuilds++
				return nil
			},
		},
	}
	cleanup := &bootCleanup{}

	if err := d.bootExtensions(testutil.Context(t), state, cleanup); err != nil {
		t.Fatalf("bootExtensions() error = %v, want nil", err)
	}

	if runtime.startCount != 1 {
		t.Fatalf("extension runtime start count = %d, want 1", runtime.startCount)
	}
	if rebuilds != 1 {
		t.Fatalf("hook rebuild count = %d, want 1 after failed start", rebuilds)
	}
	if len(cleanup.fns) != 1 {
		t.Fatalf("cleanup fns = %d, want 1", len(cleanup.fns))
	}
	if state.currentExtensionRuntime() != runtime {
		t.Fatalf("state.extensions = %#v, want runtime after failed start", state.currentExtensionRuntime())
	}
	if state.deps.Extensions == nil {
		t.Fatal("state.deps.Extensions = nil, want extension service after failed start")
	}
	if state.bridges.extensions != runtime {
		t.Fatalf("state.bridges.extensions = %#v, want runtime after failed start", state.bridges.extensions)
	}
	if !strings.Contains(logBuffer.String(), "extension manager start failed") {
		t.Fatalf("log output = %q, want extension start failure message", logBuffer.String())
	}
}

func TestBootExtensionsKeepsHealthyRegisteredExtensionsAfterPartialStartFailure(t *testing.T) {
	t.Parallel()

	t.Run("ShouldKeepHealthyRegisteredExtensionsAfterPartialStartFailure", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "ext-healthy", daemonTestExtensionOptions{}, true)
		installDaemonTestExtension(t, db, "ext-bad", daemonTestExtensionOptions{}, true)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
		runtime := &fakeExtensionRuntime{
			startErr: errors.New("boom"),
			getFn: func(name string) (*extensionpkg.Extension, error) {
				switch name {
				case "ext-healthy":
					return &extensionpkg.Extension{
						Info: extensionpkg.ExtensionInfo{
							Name:    "ext-healthy",
							Enabled: true,
						},
						Status: extensionpkg.ExtensionStatus{
							Name:       "ext-healthy",
							Enabled:    true,
							Registered: true,
						},
					}, nil
				case "ext-bad":
					return nil, extensionpkg.ErrExtensionNotFound
				default:
					return nil, extensionpkg.ErrExtensionNotFound
				}
			},
		}
		homePaths := testHomePaths(t)
		d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))
		d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
			return runtime
		}

		rebuilds := 0
		state := &bootState{
			logger:   logger,
			registry: db,
			sessions: &fakeSessionManager{},
			observer: &fakeObserver{},
			bridges:  &bridgeRuntime{broker: bridgepkg.NewBroker(nil)},
			hooks: &fakeHookRuntime{
				onRebuild: func(context.Context) error {
					rebuilds++
					return nil
				},
			},
		}
		cleanup := &bootCleanup{}

		if err := d.bootExtensions(testutil.Context(t), state, cleanup); err != nil {
			t.Fatalf("bootExtensions() error = %v, want nil", err)
		}

		if runtime.startCount != 1 {
			t.Fatalf("extension runtime start count = %d, want 1", runtime.startCount)
		}
		if rebuilds != 1 {
			t.Fatalf("hook rebuild count = %d, want 1 after partial start", rebuilds)
		}
		if len(cleanup.fns) != 1 {
			t.Fatalf("cleanup fns = %d, want 1", len(cleanup.fns))
		}
		if state.currentExtensionRuntime() != runtime {
			t.Fatalf("state.extensions = %#v, want runtime", state.currentExtensionRuntime())
		}
		if state.deps.Extensions == nil {
			t.Fatal("state.deps.Extensions = nil, want extension service")
		}
		if state.bridges.extensions != runtime {
			t.Fatalf("state.bridges.extensions = %#v, want runtime", state.bridges.extensions)
		}
		healthy, err := state.deps.Extensions.Status(testutil.Context(t), "ext-healthy")
		if err != nil {
			t.Fatalf("Extensions.Status(ext-healthy) error = %v", err)
		}
		if got, want := healthy.State, "registered"; got != want {
			t.Fatalf("ext-healthy state = %q, want %q", got, want)
		}
		bad, err := state.deps.Extensions.Status(testutil.Context(t), "ext-bad")
		if err != nil {
			t.Fatalf("Extensions.Status(ext-bad) error = %v", err)
		}
		if got, want := bad.State, "enabled"; got != want {
			t.Fatalf("ext-bad state = %q, want %q", got, want)
		}
		if !strings.Contains(logBuffer.String(), "healthy extensions only") {
			t.Fatalf("log output = %q, want partial start continuation message", logBuffer.String())
		}
	})
}

func TestBootExtensionsContinuesAfterExtensionLocalContextFailure(t *testing.T) {
	t.Parallel()

	t.Run("ShouldContinueAfterExtensionInitializeDeadline", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		installDaemonTestExtension(t, db, "ext-timeout", daemonTestExtensionOptions{}, true)

		var logBuffer bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logBuffer, nil))
		runtime := &fakeExtensionRuntime{startErr: context.DeadlineExceeded}
		homePaths := testHomePaths(t)
		d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))
		d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
			return runtime
		}

		rebuilds := 0
		state := &bootState{
			logger:   logger,
			registry: db,
			sessions: &fakeSessionManager{},
			observer: &fakeObserver{},
			bridges:  &bridgeRuntime{broker: bridgepkg.NewBroker(nil)},
			hooks: &fakeHookRuntime{
				onRebuild: func(context.Context) error {
					rebuilds++
					return nil
				},
			},
		}
		cleanup := &bootCleanup{}

		if err := d.bootExtensions(testutil.Context(t), state, cleanup); err != nil {
			t.Fatalf("bootExtensions() error = %v, want nil", err)
		}
		if runtime.startCount != 1 {
			t.Fatalf("extension runtime start count = %d, want 1", runtime.startCount)
		}
		if rebuilds != 1 {
			t.Fatalf("hook rebuild count = %d, want 1 after local extension deadline", rebuilds)
		}
		if state.currentExtensionRuntime() != runtime {
			t.Fatalf(
				"state.extensions = %#v, want runtime after local extension deadline",
				state.currentExtensionRuntime(),
			)
		}
		if state.deps.Extensions == nil {
			t.Fatal("state.deps.Extensions = nil, want extension service after local extension deadline")
		}
		if !strings.Contains(logBuffer.String(), "extension manager start failed") {
			t.Fatalf("log output = %q, want extension start failure message", logBuffer.String())
		}
	})
}

func TestBootExtensionsPropagatesContextCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		build   func(*testing.T) context.Context
		wantErr error
	}{
		{
			name: "ShouldPropagateCanceledBootContext",
			build: func(t *testing.T) context.Context {
				t.Helper()
				ctx, cancel := context.WithCancel(testutil.Context(t))
				cancel()
				return ctx
			},
			wantErr: context.Canceled,
		},
		{
			name: "ShouldPropagateExpiredBootContext",
			build: func(t *testing.T) context.Context {
				t.Helper()
				ctx, cancel := context.WithDeadline(testutil.Context(t), time.Now().Add(-time.Second))
				t.Cleanup(cancel)
				return ctx
			},
			wantErr: context.DeadlineExceeded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := openDaemonTestGlobalDB(t)
			installDaemonTestExtension(t, db, "ext-canceled", daemonTestExtensionOptions{}, true)

			runtime := &fakeExtensionRuntime{startErr: tc.wantErr}
			homePaths := testHomePaths(t)
			d := newTestDaemon(t, homePaths, testConfigPtr(t, homePaths))
			d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
				return runtime
			}

			state := &bootState{
				logger:   discardLogger(),
				registry: db,
				sessions: &fakeSessionManager{},
				observer: &fakeObserver{},
				bridges:  &bridgeRuntime{broker: bridgepkg.NewBroker(nil)},
				hooks: &fakeHookRuntime{
					onRebuild: func(context.Context) error {
						t.Fatal("hooks should not rebuild when extension start is canceled")
						return nil
					},
				},
			}
			cleanup := &bootCleanup{}

			err := d.bootExtensions(tc.build(t), state, cleanup)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("bootExtensions() error = %v, want %v", err, tc.wantErr)
			}
			if runtime.startCount != 0 {
				t.Fatalf("extension runtime start count = %d, want 0 after canceled boot context", runtime.startCount)
			}
			if len(cleanup.fns) != 1 {
				t.Fatalf("cleanup fns = %d, want 1", len(cleanup.fns))
			}
			if state.currentExtensionRuntime() != nil {
				t.Fatalf("state.extensions = %#v, want nil after canceled start", state.currentExtensionRuntime())
			}
			if state.deps.Extensions != nil {
				t.Fatalf("state.deps.Extensions = %#v, want nil after canceled start", state.deps.Extensions)
			}
			if state.bridges.extensions != nil {
				t.Fatalf("state.bridges.extensions = %#v, want nil after canceled start", state.bridges.extensions)
			}
		})
	}
}

func TestBootAutomationBuildsManagerDepsAndAttachesHookBoundary(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Automation.Enabled = true

	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
		}),
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	baseLifecycle := &recordingNotifier{}
	managerLifecycle := &recordingNotifier{}
	baseTelemetry := &recordingHookTelemetrySink{}
	managerTelemetry := &recordingHookTelemetrySink{}
	manager := &fakeAutomationManager{
		sessionObserver:   managerLifecycle,
		hookTelemetrySink: managerTelemetry,
		status:            automationpkg.ManagerStatus{Running: true, SchedulerRunning: true},
	}

	var captured automationManagerDeps
	d := newTestDaemon(t, homePaths, &cfg)
	d.newAutomationManager = func(deps automationManagerDeps) (automationRuntime, error) {
		captured = deps
		return manager, nil
	}

	state := &bootState{
		cfg:                cfg,
		logger:             discardLogger(),
		registry:           db,
		sessions:           &fakeSessionManager{},
		workspaceResolver:  resolver,
		lifecycleObservers: newSessionLifecycleFanout(baseLifecycle),
		hookTelemetrySinks: newHookTelemetryFanout(baseTelemetry),
	}
	cleanup := &bootCleanup{}

	if err := d.bootAutomation(testutil.Context(t), state, cleanup); err != nil {
		t.Fatalf("bootAutomation() error = %v", err)
	}

	if captured.Store == nil {
		t.Fatal("captured.Store = nil")
	}
	if captured.Sessions != state.sessions {
		t.Fatal("captured.Sessions dependency mismatch")
	}
	if captured.WorkspaceResolver != resolver {
		t.Fatal("captured.WorkspaceResolver dependency mismatch")
	}
	if got, want := captured.Config.Enabled, cfg.Automation.Enabled; got != want {
		t.Fatalf("captured.Config.Enabled = %v, want %v", got, want)
	}
	if got, want := captured.Config.Timezone, cfg.Automation.Timezone; got != want {
		t.Fatalf("captured.Config.Timezone = %q, want %q", got, want)
	}
	if got, want := captured.Config.MaxConcurrentJobs, cfg.Automation.MaxConcurrentJobs; got != want {
		t.Fatalf("captured.Config.MaxConcurrentJobs = %d, want %d", got, want)
	}
	if got, want := captured.Config.DefaultFireLimit, cfg.Automation.DefaultFireLimit; got != want {
		t.Fatalf("captured.Config.DefaultFireLimit = %#v, want %#v", got, want)
	}
	if got, want := captured.GlobalWorkspacePath, homePaths.HomeDir; got != want {
		t.Fatalf("captured.GlobalWorkspacePath = %q, want %q", got, want)
	}
	if manager.startCount != 1 {
		t.Fatalf("manager start count = %d, want 1", manager.startCount)
	}
	if state.automation != manager {
		t.Fatalf("state.automation = %#v, want manager", state.automation)
	}
	if state.deps.Automation != manager {
		t.Fatalf("state.deps.Automation = %#v, want manager", state.deps.Automation)
	}
	if len(cleanup.fns) != 1 {
		t.Fatalf("cleanup fns = %d, want 1", len(cleanup.fns))
	}

	state.lifecycleObservers.OnSessionCreated(testutil.Context(t), &session.Session{ID: "sess-automation"})
	if got, want := managerLifecycle.events, []string{"created"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("manager lifecycle events = %#v, want %#v", got, want)
	}
	if got, want := baseLifecycle.events, []string{"created"}; !testutil.EqualStringSlices(got, want) {
		t.Fatalf("base lifecycle events = %#v, want %#v", got, want)
	}

	if err := state.hookTelemetrySinks.WriteHookRecord(
		testutil.Context(t),
		"sess-automation",
		hookspkg.HookRunRecord{HookName: "post-stop"},
	); err != nil {
		t.Fatalf("WriteHookRecord() error = %v", err)
	}
	if got, want := baseTelemetry.count(), 1; got != want {
		t.Fatalf("base telemetry count = %d, want %d", got, want)
	}
	if got, want := managerTelemetry.count(), 1; got != want {
		t.Fatalf("manager telemetry count = %d, want %d", got, want)
	}
}

func TestHooksNotifierNoopDispatchesWithoutRuntime(t *testing.T) {
	t.Parallel()

	notifier := newHooksNotifier(discardLogger(), func() time.Time {
		return time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	})

	notifier.OnSessionCreated(testutil.Context(t), &session.Session{ID: "sess-created"})
	notifier.OnSessionStopped(testutil.Context(t), &session.Session{ID: "sess-stopped"})

	if _, err := notifier.DispatchSessionPreCreate(
		testutil.Context(t),
		hookspkg.SessionPreCreatePayload{},
	); err != nil {
		t.Fatalf("DispatchSessionPreCreate() error = %v", err)
	}
	if _, err := notifier.DispatchSessionPreResume(
		testutil.Context(t),
		hookspkg.SessionPreResumePayload{},
	); err != nil {
		t.Fatalf("DispatchSessionPreResume() error = %v", err)
	}
	if _, err := notifier.DispatchSessionPostResume(
		testutil.Context(t),
		hookspkg.SessionPostResumePayload{},
	); err != nil {
		t.Fatalf("DispatchSessionPostResume() error = %v", err)
	}
	if _, err := notifier.DispatchSessionPreStop(testutil.Context(t), hookspkg.SessionPreStopPayload{}); err != nil {
		t.Fatalf("DispatchSessionPreStop() error = %v", err)
	}
	if _, err := notifier.DispatchInputPreSubmit(testutil.Context(t), hookspkg.InputPreSubmitPayload{}); err != nil {
		t.Fatalf("DispatchInputPreSubmit() error = %v", err)
	}
	if _, err := notifier.DispatchPromptPostAssemble(testutil.Context(t), hookspkg.PromptPayload{}); err != nil {
		t.Fatalf("DispatchPromptPostAssemble() error = %v", err)
	}
	if _, err := notifier.DispatchEventPreRecord(testutil.Context(t), hookspkg.EventPreRecordPayload{}); err != nil {
		t.Fatalf("DispatchEventPreRecord() error = %v", err)
	}
	if _, err := notifier.DispatchEventPostRecord(testutil.Context(t), hookspkg.EventPostRecordPayload{}); err != nil {
		t.Fatalf("DispatchEventPostRecord() error = %v", err)
	}
	if _, err := notifier.DispatchAgentPreStart(testutil.Context(t), hookspkg.AgentPreStartPayload{}); err != nil {
		t.Fatalf("DispatchAgentPreStart() error = %v", err)
	}
	if _, err := notifier.DispatchAgentSpawned(testutil.Context(t), hookspkg.AgentSpawnedPayload{}); err != nil {
		t.Fatalf("DispatchAgentSpawned() error = %v", err)
	}
	if _, err := notifier.DispatchAgentCrashed(testutil.Context(t), hookspkg.AgentCrashedPayload{}); err != nil {
		t.Fatalf("DispatchAgentCrashed() error = %v", err)
	}
	if _, err := notifier.DispatchAgentStopped(testutil.Context(t), hookspkg.AgentStoppedPayload{}); err != nil {
		t.Fatalf("DispatchAgentStopped() error = %v", err)
	}
}

func TestDaemonExtensionServiceInstallStatusAndDisable(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	manager := extensionpkg.NewManager(registry, extensionpkg.WithLogger(discardLogger()))
	if err := manager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Stop(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Stop() error = %v", err)
		}
	})

	syncs := 0
	fixedNow := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	service := newDaemonExtensionService(
		registry,
		manager,
		fakeHookBindingPublisher(func(context.Context) error {
			syncs++
			return nil
		}),
		nil,
		nil,
		nil,
		nil,
		homePaths,
		discardLogger(),
		func() time.Time { return fixedNow },
		withDaemonExtensionMarketplace(
			aghconfig.ExtensionsMarketplaceConfig{AllowUnverified: true},
			nil,
		),
		withDaemonExtensionEventWriter(db),
	)
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh extension install")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}

	fixtureDir := filepath.Join(t.TempDir(), "service-ext")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", fixtureDir, err)
	}
	if err := os.WriteFile(
		filepath.Join(fixtureDir, "extension.toml"),
		[]byte(daemonTestExtensionManifest("service-ext", daemonTestExtensionOptions{
			runtimeCommand: daemonExtensionHelperCommand(t),
			runtimeArgs:    daemonExtensionHelperArgs(),
			runtimeEnv:     daemonExtensionHelperEnv(""),
		})),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(fixtureDir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
	}

	installed, err := service.Install(testutil.Context(t), contract.InstallExtensionRequest{
		Path:            fixtureDir,
		Checksum:        checksum,
		AllowUnverified: true,
	}, actor)
	if err != nil {
		t.Fatalf("service.Install() error = %v", err)
	}
	if installed.Name != "service-ext" || installed.State != "active" || !installed.DaemonRunning {
		t.Fatalf("installed extension = %#v, want active daemon-backed extension", installed)
	}

	info, err := registry.Get("service-ext")
	if err != nil {
		t.Fatalf("registry.Get(service-ext) error = %v", err)
	}
	wantManifestPath := filepath.Join(extensionpkg.ManagedInstallPath(homePaths, "service-ext"), "extension.toml")
	if info.ManifestPath != wantManifestPath {
		t.Fatalf("installed manifest path = %q, want %q", info.ManifestPath, wantManifestPath)
	}
	if _, err := os.Stat(filepath.Join(fixtureDir, "extension.toml")); err != nil {
		t.Fatalf("source fixture manifest stat error = %v", err)
	}

	status, err := service.Status(testutil.Context(t), "service-ext")
	if err != nil {
		t.Fatalf("service.Status() error = %v", err)
	}
	if status.Name != "service-ext" || status.State != "active" {
		t.Fatalf("status = %#v, want active extension", status)
	}

	disabled, err := service.Disable(testutil.Context(t), "service-ext", actor)
	if err != nil {
		t.Fatalf("service.Disable() error = %v", err)
	}
	if disabled.State != "disabled" || disabled.Enabled {
		t.Fatalf("disabled extension = %#v, want disabled extension", disabled)
	}

	enabled, err := service.Enable(testutil.Context(t), "service-ext", actor)
	if err != nil {
		t.Fatalf("service.Enable() error = %v", err)
	}
	if enabled.State != "active" || !enabled.Enabled {
		t.Fatalf("enabled extension = %#v, want active enabled extension", enabled)
	}

	disabled, err = service.Disable(testutil.Context(t), "service-ext", actor)
	if err != nil {
		t.Fatalf("service.Disable(second) error = %v", err)
	}
	if disabled.State != "disabled" || disabled.Enabled {
		t.Fatalf("disabled extension after second disable = %#v, want disabled extension", disabled)
	}

	listed, err := service.List(testutil.Context(t))
	if err != nil {
		t.Fatalf("service.List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].State != "disabled" {
		t.Fatalf("listed extensions = %#v, want one disabled extension", listed)
	}
	if syncs != 4 {
		t.Fatalf("hook binding sync count = %d, want 4", syncs)
	}
	summaries, err := db.ListEventSummaries(testutil.Context(t), store.EventSummaryQuery{
		Component: eventspkg.ComponentExtension,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(extension) error = %v", err)
	}
	wantCounts := map[string]int{
		eventspkg.ExtensionInstalled: 1,
		eventspkg.ExtensionEnabled:   1,
		eventspkg.ExtensionDisabled:  2,
	}
	gotCounts := make(map[string]int, len(wantCounts))
	var installContent []byte
	for i, summary := range summaries {
		gotCounts[summary.Type]++
		if summary.ActorKind != string(taskpkg.ActorKindHuman) || summary.ActorID != "user-1" {
			t.Fatalf("summary[%d] actor = %s/%s, want human/user-1", i, summary.ActorKind, summary.ActorID)
		}
		if summary.Type == eventspkg.ExtensionInstalled {
			installContent = summary.Content
		}
	}
	if len(summaries) != 4 || !maps.Equal(gotCounts, wantCounts) {
		t.Fatalf("extension event type counts = %#v from summaries=%#v, want %#v", gotCounts, summaries, wantCounts)
	}
	if !bytes.Contains(installContent, []byte("\"allow_unverified\":true")) {
		t.Fatalf("install event content = %s, want allow_unverified=true", string(installContent))
	}
}

func TestDaemonExtensionServiceRecordsCommittedBatchUpdatesBeforeReturningFailure(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	fixedNow := time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC)
	service := &daemonExtensionService{eventWriter: db, now: func() time.Time { return fixedNow }}
	actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh extension update --all")
	if err != nil {
		t.Fatalf("DeriveHumanActorContext() error = %v", err)
	}
	cause := errors.New("later update failed")
	items := []extensionpkg.MarketplaceUpdateResult{
		{
			Name: "a-good", Slug: "acme/a-good", Registry: "github",
			CurrentVersion: "1.0.0", LatestVersion: "2.0.0", Path: "/extensions/a-good",
			Status: extensionpkg.MarketplaceUpdateStatusUpdated,
			Warnings: []diagnosticcontract.DiagnosticItem{{
				Code: diagnosticcontract.CodeExtensionUpdateCleanupFailed,
			}},
		},
		{
			Name: "b-good", Slug: "acme/b-good", Registry: "github",
			CurrentVersion: "1.0.0", LatestVersion: "2.0.0", Path: "/extensions/b-good",
			Status: extensionpkg.MarketplaceUpdateStatusUpdated,
		},
	}

	payloads, err := service.finalizeMarketplaceUpdateBatch(t.Context(), actor, items, cause)
	if !errors.Is(err, cause) {
		t.Fatalf("finalizeMarketplaceUpdateBatch() error = %v, want original batch failure", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("finalizeMarketplaceUpdateBatch() payloads = %#v, want two committed updates", payloads)
	}
	if len(payloads[0].Warnings) != 1 ||
		payloads[0].Warnings[0].Code != diagnosticcontract.CodeExtensionUpdateCleanupFailed {
		t.Fatalf("finalizeMarketplaceUpdateBatch() warnings = %#v, want cleanup warning", payloads[0].Warnings)
	}
	summaries, err := db.ListEventSummaries(t.Context(), store.EventSummaryQuery{
		Type: eventspkg.ExtensionUpdated,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(extension.updated) error = %v", err)
	}
	if len(summaries) != 2 {
		t.Fatalf("extension.updated summaries = %#v, want two", summaries)
	}
	if !bytes.Contains(summaries[0].Content, []byte(diagnosticcontract.CodeExtensionUpdateCleanupFailed)) &&
		!bytes.Contains(summaries[1].Content, []byte(diagnosticcontract.CodeExtensionUpdateCleanupFailed)) {
		t.Fatalf("extension.updated summaries = %#v, want cleanup warning", summaries)
	}
	for _, name := range []string{"a-good", "b-good"} {
		found := false
		for _, summary := range summaries {
			if bytes.Contains(summary.Content, []byte(`"name":"`+name+`"`)) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("extension.updated summaries = %#v, want event for %s", summaries, name)
		}
	}

	writeErr := errors.New("event store failed")
	failingWriter := &daemonExtensionEventStoreStub{writeErr: writeErr}
	service.eventWriter = failingWriter
	_, err = service.finalizeMarketplaceUpdateBatch(t.Context(), actor, items, cause)
	if !errors.Is(err, cause) || !errors.Is(err, writeErr) {
		t.Fatalf("finalizeMarketplaceUpdateBatch(event failure) error = %v, want batch and event failures", err)
	}
	if failingWriter.writeCalls != len(items) {
		t.Fatalf("event writer calls = %d, want %d committed update attempts", failingWriter.writeCalls, len(items))
	}
}

type daemonExtensionEventStoreStub struct {
	writeErr   error
	writeCalls int
}

func (s *daemonExtensionEventStoreStub) WriteEventSummary(context.Context, store.EventSummary) error {
	s.writeCalls++
	return s.writeErr
}

func (*daemonExtensionEventStoreStub) ListEventSummaries(
	context.Context,
	store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return nil, nil
}

func TestDaemonExtensionServiceRollsBackFailedInstallReload(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRollBackManagedInstallWhenReloadFails", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		db := openDaemonTestGlobalDB(t)
		registry := extensionpkg.NewRegistry(db.DB())
		manager := extensionpkg.NewManager(registry, extensionpkg.WithLogger(discardLogger()))
		if err := manager.Start(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t)); err != nil {
				t.Fatalf("manager.Stop() error = %v", err)
			}
		})

		service := newDaemonExtensionService(
			registry,
			manager,
			fakeHookBindingPublisher(func(context.Context) error {
				return nil
			}),
			nil,
			nil,
			nil,
			nil,
			homePaths,
			discardLogger(),
			time.Now,
			withDaemonExtensionMarketplace(
				aghconfig.ExtensionsMarketplaceConfig{AllowUnverified: true},
				nil,
			),
		)
		actor, err := taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindCLI, "agh extension install")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext() error = %v", err)
		}

		fixtureDir := filepath.Join(t.TempDir(), "rollback-ext")
		agentDir := filepath.Join(fixtureDir, "agents", "broken")
		if err := os.MkdirAll(agentDir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", agentDir, err)
		}
		if err := os.WriteFile(
			filepath.Join(fixtureDir, "extension.toml"),
			[]byte(`[extension]
name = "rollback-ext"
version = "0.1.0"
description = "Invalid extension used to verify daemon install rollback."
min_agh_version = "0.0.1"

[resources]
agents = ["agents"]
`),
			0o644,
		); err != nil {
			t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(agentDir, "AGENT.md"),
			[]byte(`---
provider: codex
---

Broken agent missing required name.
`),
			0o644,
		); err != nil {
			t.Fatalf("os.WriteFile(AGENT.md) error = %v", err)
		}

		checksum, err := extensionpkg.ComputeDirectoryChecksum(fixtureDir)
		if err != nil {
			t.Fatalf("ComputeDirectoryChecksum() error = %v", err)
		}

		_, err = service.Install(testutil.Context(t), contract.InstallExtensionRequest{
			Path:            fixtureDir,
			Checksum:        checksum,
			AllowUnverified: true,
		}, actor)
		if err == nil {
			t.Fatal("service.Install(invalid extension) error = nil, want reload failure")
		}
		if !strings.Contains(err.Error(), "agent name is required") {
			t.Fatalf("service.Install(invalid extension) error = %v, want agent parse failure", err)
		}

		if _, err := registry.Get("rollback-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("registry.Get(rollback-ext) error = %v, want ErrExtensionNotFound", err)
		}
		managedPath := extensionpkg.ManagedInstallPath(homePaths, "rollback-ext")
		if _, err := os.Stat(managedPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want not exists", managedPath, err)
		}
		if _, err := manager.Get("rollback-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("manager.Get(rollback-ext) error = %v, want ErrExtensionNotFound", err)
		}
		if listed := manager.List(); len(listed) != 0 {
			t.Fatalf("manager.List() = %#v, want no extensions after rollback", listed)
		}
		if _, err := os.Stat(filepath.Join(fixtureDir, "extension.toml")); err != nil {
			t.Fatalf("source fixture manifest stat error = %v", err)
		}
	})
}

func TestDaemonExtensionServiceCheckReadyErrors(t *testing.T) {
	t.Parallel()

	var nilService *daemonExtensionService
	if err := nilService.checkReady(); err == nil {
		t.Fatal("nil service checkReady() error = nil, want error")
	}

	service := &daemonExtensionService{homePaths: testHomePaths(t), logger: discardLogger(), now: time.Now}
	if _, err := service.List(testutil.Context(t)); err == nil {
		t.Fatal("List() without registry error = nil, want error")
	}
}

func TestExtensionDeclarationProviderReturnsRuntimeDeclarations(t *testing.T) {
	t.Parallel()

	want := []hookspkg.HookDecl{
		{
			Name:         "ext-turn-start",
			Event:        hookspkg.HookTurnStart,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorSubprocess,
			Command:      "/bin/sh",
			Args:         []string{"-c", "printf '{}'"},
		},
	}
	runtime := &fakeExtensionRuntime{hookDecls: want}

	got, err := extensionDeclarationProvider(func() extensionRuntime { return runtime })(testutil.Context(t))
	if err != nil {
		t.Fatalf("extensionDeclarationProvider() error = %v", err)
	}
	if !testutil.EqualStringSlices([]string{got[0].Name}, []string{want[0].Name}) {
		t.Fatalf("extensionDeclarationProvider() = %#v, want %#v", got, want)
	}
}

func TestChainDeclarationProvidersWrapsProviderErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("provider boom")
	provider := chainDeclarationProviders(
		func(context.Context) ([]hookspkg.HookDecl, error) {
			return nil, wantErr
		},
	)

	_, err := provider(testutil.Context(t))
	if !errors.Is(err, wantErr) {
		t.Fatalf("provider error = %v, want wrapped %v", err, wantErr)
	}
	if err == nil || !strings.Contains(err.Error(), "provider 1") {
		t.Fatalf("provider error = %v, want provider context", err)
	}
}

func TestExtensionDeclarationProviderWrapsRuntimeErrors(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("runtime boom")
	runtime := &fakeExtensionRuntime{hookErr: wantErr}

	_, err := extensionDeclarationProvider(func() extensionRuntime { return runtime })(testutil.Context(t))
	if !errors.Is(err, wantErr) {
		t.Fatalf("extensionDeclarationProvider() error = %v, want wrapped %v", err, wantErr)
	}
	if err == nil || !strings.Contains(err.Error(), "extension runtime") {
		t.Fatalf("extensionDeclarationProvider() error = %v, want runtime context", err)
	}
}

func TestBootStateExtensionRuntimeAccessIsSynchronized(t *testing.T) {
	t.Parallel()

	state := &bootState{}
	runtime := &fakeExtensionRuntime{
		hookDecls: []hookspkg.HookDecl{{
			Name:         "ext-turn-start",
			Event:        hookspkg.HookTurnStart,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorSubprocess,
			Command:      "/bin/sh",
			Args:         []string{"-c", "printf '{}'"},
		}},
	}
	provider := extensionDeclarationProvider(state.currentExtensionRuntime)

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(2)

		go func(iteration int) {
			defer wg.Done()
			<-start
			for j := range 128 {
				if (iteration+j)%2 == 0 {
					state.setExtensionRuntime(runtime)
				} else {
					state.setExtensionRuntime(nil)
				}
			}
		}(i)

		go func() {
			defer wg.Done()
			<-start
			for range 128 {
				_, _ = provider(context.Background())
			}
		}()
	}

	close(start)
	wg.Wait()
}

func TestShutdownDrainsHooksBeforeClosingDatabase(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)

	asyncStarted := make(chan struct{}, 1)
	asyncRelease := make(chan struct{})
	dbClosed := make(chan struct{}, 1)

	hooks := hookspkg.NewHooks(
		hookspkg.WithLogger(discardLogger()),
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{
			{
				Name:         "async-stop",
				Event:        hookspkg.HookSessionPostStop,
				Mode:         hookspkg.HookModeAsync,
				ExecutorKind: hookspkg.HookExecutorNative,
			},
		}),
		hookspkg.WithExecutorResolver(testHookExecutorResolver(map[string]hookspkg.Executor{
			"async-stop": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.SessionLifecyclePayload) (hookspkg.SessionPostStopPatch, error) {
					asyncStarted <- struct{}{}
					<-asyncRelease
					return hookspkg.SessionPostStopPatch{}, nil
				},
			),
		})),
	)
	t.Cleanup(hooks.Close)
	if err := hooks.Rebuild(testutil.Context(t)); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}

	notifier := newHooksNotifier(
		discardLogger(),
		func() time.Time { return time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC) },
	)
	notifier.setRuntime(hooks, nil)

	d.sessions = &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}},
		onStop: func(string) {
			if _, err := notifier.DispatchSessionPostStop(
				context.Background(),
				hookSessionLifecyclePayload(&session.Session{
					ID:          "sess-a",
					AgentName:   "codex",
					WorkspaceID: "ws-1",
					Workspace:   "/tmp/ws-1",
					Type:        session.SessionTypeUser,
					State:       session.StateStopped,
					CreatedAt:   time.Date(2026, 4, 9, 11, 0, 0, 0, time.UTC),
					UpdatedAt:   time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
				}, hookspkg.HookSessionPostStop, time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC)),
			); err != nil {
				t.Fatalf("DispatchSessionPostStop() error = %v", err)
			}
		},
	}
	d.hooks = hooks
	d.registry = &recordingRegistry{
		path: homePaths.DatabaseFile,
		onClose: func() {
			dbClosed <- struct{}{}
		},
	}
	d.closeLogger = func() error { return nil }

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Shutdown(testutil.Context(t))
	}()

	select {
	case <-asyncStarted:
	case <-time.After(time.Second):
		t.Fatal("async hook did not start before shutdown blocked")
	}

	select {
	case <-dbClosed:
		t.Fatal("database closed before hooks drained")
	default:
	}

	close(asyncRelease)
	if err := <-errCh; err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	select {
	case <-dbClosed:
	case <-time.After(time.Second):
		t.Fatal("database was not closed after hooks drained")
	}
}

func TestBootFailureCleansUpStartedResourcesInReverseOrder(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	d := newTestDaemon(t, homePaths, &cfg)

	var events []string
	d.closeLogger = func() error {
		events = append(events, "logger")
		return nil
	}
	d.acquireLock = func(path string, _ int) (*Lock, error) {
		return &Lock{
			path: path,
			releaseFn: func() error {
				events = append(events, "lock")
				return nil
			},
		}, nil
	}
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{
			path: homePaths.DatabaseFile,
			onClose: func() {
				events = append(events, "db")
			},
		}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{
			name: "http",
			onShutdown: func() {
				events = append(events, "http")
			},
		}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return nil, errors.New("uds boom")
	}

	if err := d.boot(testutil.Context(t)); err == nil || !strings.Contains(err.Error(), "uds boom") {
		t.Fatalf("boot() error = %v, want uds boom", err)
	}

	want := []string{"http", "db", "lock", "logger"}
	if !testutil.EqualStringSlices(events, want) {
		t.Fatalf("boot() cleanup order = %#v, want %#v", events, want)
	}
}

func TestBootFailureWhenWritingDaemonInfoCleansUpAllServers(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	infoDir := filepath.Join(homePaths.HomeDir, "daemon-info-dir")
	if err := os.MkdirAll(infoDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(infoDir) error = %v", err)
	}

	d := newTestDaemon(t, homePaths, &cfg)
	d.homePaths.DaemonInfo = infoDir

	var events []string
	d.closeLogger = func() error {
		events = append(events, "logger")
		return nil
	}
	d.acquireLock = func(path string, _ int) (*Lock, error) {
		return &Lock{
			path: path,
			releaseFn: func() error {
				events = append(events, "lock")
				return nil
			},
		}, nil
	}
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{
			path: homePaths.DatabaseFile,
			onClose: func() {
				events = append(events, "db")
			},
		}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http", onShutdown: func() { events = append(events, "http") }}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds", onShutdown: func() { events = append(events, "uds") }}, nil
	}

	if err := d.boot(testutil.Context(t)); err == nil || !strings.Contains(err.Error(), "daemon info") {
		t.Fatalf("boot() error = %v, want daemon info failure", err)
	}

	want := []string{"uds", "http", "db", "lock", "logger"}
	if !testutil.EqualStringSlices(events, want) {
		t.Fatalf("boot() cleanup order = %#v, want %#v", events, want)
	}
}

func TestVerifyImportBoundariesReportsViolations(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module github.com/compozy/agh\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}

	sourceDir := filepath.Join(root, "internal", "worker")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(sourceDir) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "worker.go"),
		[]byte("package worker\n\nimport _ \"github.com/compozy/agh/internal/daemon\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(worker.go) error = %v", err)
	}

	violations, err := verifyImportBoundaries(root)
	if err != nil {
		t.Fatalf("verifyImportBoundaries() error = %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("verifyImportBoundaries() violations = %d, want 1", len(violations))
	}
}

func TestVerifyImportBoundariesAllowsDaemonSubpackages(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module github.com/compozy/agh\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}

	sourceDir := filepath.Join(root, "internal", "daemon", "subsystem")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(sourceDir) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "subsystem.go"),
		[]byte("package subsystem\n\nimport _ \"github.com/compozy/agh/internal/cli\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(subsystem.go) error = %v", err)
	}

	violations, err := verifyImportBoundaries(root)
	if err != nil {
		t.Fatalf("verifyImportBoundaries() error = %v", err)
	}
	if len(violations) != 0 {
		t.Fatalf("verifyImportBoundaries() violations = %d, want 0", len(violations))
	}
}

func TestVerifyImportBoundariesDoesNotExemptHTTPPackages(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module github.com/compozy/agh\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}

	sourceDir := filepath.Join(root, "internal", "api", "httpapi")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(sourceDir) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(sourceDir, "handler.go"),
		[]byte("package httpapi\n\nimport _ \"github.com/compozy/agh/internal/cli\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(handler.go) error = %v", err)
	}

	violations, err := verifyImportBoundaries(root)
	if err != nil {
		t.Fatalf("verifyImportBoundaries() error = %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("verifyImportBoundaries() violations = %d, want 1", len(violations))
	}
}

func TestStopSessionsIgnoresNotFoundAndHandlesNilManager(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.stopSessions(testutil.Context(t), nil); err != nil {
		t.Fatalf("stopSessions(nil) error = %v", err)
	}

	manager := &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}},
		stopErr: func(id string) error {
			return fmt.Errorf("%w: %s", session.ErrSessionNotFound, id)
		},
	}
	if err := d.stopSessions(testutil.Context(t), manager); err != nil {
		t.Fatalf("stopSessions(not found) error = %v", err)
	}
}

func TestStopSessionsUsesShutdownCauseWhenSupported(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	manager := &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}},
	}
	if err := d.stopSessions(testutil.Context(t), manager); err != nil {
		t.Fatalf("stopSessions() error = %v", err)
	}

	if got := len(manager.stopWithCauseCalls); got != 1 {
		t.Fatalf("StopWithCause() calls = %d, want 1", got)
	}
	call := manager.stopWithCauseCalls[0]
	if call.id != "sess-a" {
		t.Fatalf("StopWithCause() id = %q, want %q", call.id, "sess-a")
	}
	if call.cause != session.CauseShutdown {
		t.Fatalf("StopWithCause() cause = %v, want %v", call.cause, session.CauseShutdown)
	}
	if call.detail != "daemon shutdown" {
		t.Fatalf("StopWithCause() detail = %q, want %q", call.detail, "daemon shutdown")
	}
	if got := len(manager.stopCalls); got != 0 {
		t.Fatalf("Stop() calls = %d, want 0 when StopWithCause is available", got)
	}
}

func TestFakeSessionManagerDeleteTracksDeleteIndependently(t *testing.T) {
	t.Parallel()

	t.Run("ShouldTrackDeleteIndependentlyFromStop", func(t *testing.T) {
		t.Parallel()

		manager := &fakeSessionManager{
			infos: []*session.Info{{ID: "sess-a"}, {ID: "sess-b"}},
		}

		if err := manager.Delete(testutil.Context(t), "sess-a"); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		if got, want := len(manager.deleteCalls), 1; got != want {
			t.Fatalf("len(deleteCalls) = %d, want %d", got, want)
		}
		if got, want := manager.deleteCalls[0], "sess-a"; got != want {
			t.Fatalf("deleteCalls[0] = %q, want %q", got, want)
		}
		if got := len(manager.stopCalls); got != 0 {
			t.Fatalf("len(stopCalls) = %d, want 0", got)
		}
		if got, want := len(manager.infos), 1; got != want {
			t.Fatalf("len(infos) = %d, want %d", got, want)
		}
		if got, want := manager.infos[0].ID, "sess-b"; got != want {
			t.Fatalf("infos[0].ID = %q, want %q", got, want)
		}
	})
}

func TestStopSessionsWaitsForInFlightFinalizations(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	release := make(chan struct{})
	manager := &fakeSessionManager{
		infos:                    []*session.Info{{ID: "sess-a"}},
		waitFinalizationsRelease: release,
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- d.stopSessions(testutil.Context(t), manager)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("stopSessions() returned before finalizations completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	if err := <-stopDone; err != nil {
		t.Fatalf("stopSessions() error = %v", err)
	}
	if got := manager.waitFinalizationsCalls; got != 1 {
		t.Fatalf("WaitForFinalizations() calls = %d, want 1", got)
	}
}

func TestStopSessionsShutsDownManagerAfterFinalizations(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	release := make(chan struct{})
	manager := &fakeSessionManager{
		infos:                    []*session.Info{{ID: "sess-a"}},
		waitFinalizationsRelease: release,
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- d.stopSessions(testutil.Context(t), manager)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("stopSessions() returned before finalizations completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	if got := manager.shutdownCalls; got != 0 {
		t.Fatalf("Shutdown() calls before finalization release = %d, want 0", got)
	}

	close(release)

	if err := <-stopDone; err != nil {
		t.Fatalf("stopSessions() error = %v", err)
	}

	if got := manager.waitFinalizationsCalls; got != 1 {
		t.Fatalf("WaitForFinalizations() calls = %d, want 1", got)
	}
	if got := manager.shutdownCalls; got != 1 {
		t.Fatalf("Shutdown() calls = %d, want 1", got)
	}
}

func TestCleanupOrphansHandlesListAndSignalErrors(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	d.listProcesses = func(context.Context) ([]processInfo, error) {
		return nil, errors.New("ps failed")
	}
	if err := d.cleanupOrphans(testutil.Context(t), 1); err == nil || !strings.Contains(err.Error(), "ps failed") {
		t.Fatalf("cleanupOrphans(list failure) error = %v, want ps failed", err)
	}

	d.listProcesses = func(context.Context) ([]processInfo, error) {
		return []processInfo{{PID: 10, PPID: 5}}, nil
	}
	d.signalProcess = func(int, syscall.Signal) error {
		return errors.New("signal failed")
	}
	if err := d.cleanupOrphans(testutil.Context(t), 5); err == nil || !strings.Contains(err.Error(), "signal failed") {
		t.Fatalf("cleanupOrphans(signal failure) error = %v, want signal failed", err)
	}
	if err := d.cleanupOrphans(testutil.Context(t), 0); err != nil {
		t.Fatalf("cleanupOrphans(no stale pid) error = %v", err)
	}
}

func TestOptionsConfigureDaemon(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	signalCh := make(chan os.Signal, 1)
	httpFactory := func(context.Context, RuntimeDeps) (Server, error) { return &fakeServer{name: "http"}, nil }
	udsFactory := func(context.Context, RuntimeDeps) (Server, error) { return &fakeServer{name: "uds"}, nil }
	now := time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC)

	d, err := New(
		WithHomePaths(homePaths),
		WithConfigLoader(func() (aghconfig.Config, error) { return cfg, nil }),
		WithLogger(discardLogger()),
		WithNow(func() time.Time { return now }),
		WithHTTPServerFactory(httpFactory),
		WithUDSServerFactory(udsFactory),
		WithSignalBridge(signalCh),
		WithBoundaryVerification(true),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got, err := d.loadConfig(); err != nil {
		t.Fatalf("loadConfig() error = %v", err)
	} else if got.HTTP.Port != cfg.HTTP.Port {
		t.Fatalf("loadConfig() port = %d, want %d", got.HTTP.Port, cfg.HTTP.Port)
	}
	if got := d.now(); !got.Equal(now) {
		t.Fatalf("now() = %v, want %v", got, now)
	}
	if d.signalCh != signalCh {
		t.Fatal("WithSignalBridge() did not apply")
	}
	if !d.verifyBoundaries {
		t.Fatal("WithBoundaryVerification(true) did not apply")
	}
}

func TestBootConfigEnsuresManagedAgents(t *testing.T) {
	t.Parallel()

	t.Run("Should ensure the default managed agent definitions", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		d := newTestDaemon(t, homePaths, &cfg)
		state := &bootState{}
		cleanup := &bootCleanup{}

		if err := d.bootConfig(state, cleanup); err != nil {
			t.Fatalf("bootConfig() error = %v", err)
		}

		for _, name := range []string{aghconfig.DefaultAgentName} {
			agent, err := aghconfig.LoadAgentDef(name, homePaths)
			if err != nil {
				t.Fatalf("LoadAgentDef(%s) error = %v", name, err)
			}
			if agent.Name != name {
				t.Fatalf("LoadAgentDef(%s).Name = %q, want %q", name, agent.Name, name)
			}
		}
	})
}

func TestRunShutsDownOnInjectedSignal(t *testing.T) {
	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	signalCh := make(chan os.Signal, 1)
	const waitTimeout = 5 * time.Second

	d := newTestDaemon(t, homePaths, &cfg)
	d.signalCh = signalCh
	d.acquireLock = func(path string, _ int) (*Lock, error) {
		return &Lock{path: path}, nil
	}
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(context.Background())
	}()

	select {
	case <-d.readyCh:
	case err := <-errCh:
		t.Fatalf("Run() exited before signaling ready: %v", err)
	case <-time.After(waitTimeout):
		t.Fatal("Run() did not signal ready after boot")
	}
	signalCh <- syscall.SIGTERM

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run() did not shut down after injected signal")
	}
}

func TestRunAbortsCleanlyOnInjectedSignalDuringBoot(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	signalCh := make(chan os.Signal, 1)
	sessionFactoryStarted := make(chan struct{})
	const waitTimeout = 5 * time.Second

	d := newTestDaemon(t, homePaths, &cfg)
	d.signalCh = signalCh

	var (
		eventsMu sync.Mutex
		events   []string
	)
	recordEvent := func(event string) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		events = append(events, event)
	}
	d.closeLogger = func() error {
		recordEvent("logger")
		return nil
	}
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{
			path: homePaths.DatabaseFile,
			onClose: func() {
				recordEvent("db")
			},
		}, nil
	}
	d.newSessionManager = func(ctx context.Context, _ SessionManagerDeps) (SessionManager, error) {
		close(sessionFactoryStarted)
		<-ctx.Done()
		return nil, ctx.Err()
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(context.Background())
	}()

	select {
	case <-sessionFactoryStarted:
	case err := <-errCh:
		t.Fatalf("Run() exited before boot reached session manager: %v", err)
	case <-time.After(waitTimeout):
		t.Fatal("Run() did not reach cancellable boot step")
	}

	signalCh <- syscall.SIGTERM

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context.Canceled", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run() did not abort boot after injected signal")
	}

	lockBytes, err := os.ReadFile(homePaths.DaemonLock)
	if err != nil {
		t.Fatalf("os.ReadFile(daemon lock) error = %v", err)
	}
	t.Logf("daemon_lock=%s contents=%q", homePaths.DaemonLock, string(lockBytes))
	if got := strings.TrimSpace(string(lockBytes)); got != "0" {
		t.Fatalf("daemon lock = %q, want 0 after boot cleanup", got)
	}
	if _, err := ReadInfo(homePaths.DaemonInfo); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadInfo(after boot abort) error = %v, want os.ErrNotExist", err)
	}
	t.Logf("daemon_info=%s removed=true", homePaths.DaemonInfo)
	select {
	case <-d.readyCh:
		t.Fatal("readyCh closed even though boot was aborted")
	default:
	}

	eventsMu.Lock()
	gotEvents := append([]string(nil), events...)
	eventsMu.Unlock()
	wantEvents := []string{"db", "logger"}
	if !testutil.EqualStringSlices(gotEvents, wantEvents) {
		t.Fatalf("boot abort cleanup events = %#v, want %#v", gotEvents, wantEvents)
	}
	t.Logf("cleanup_events=%v", gotEvents)
}

const daemonGoleakHelperEnv = "AGH_DAEMON_GOLEAK_HELPER"

func TestDaemonGoleakHelperProcess(_ *testing.T) {
	if os.Getenv(daemonGoleakHelperEnv) != "1" {
		return
	}

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-signalCh:
		signal.Stop(signalCh)
		os.Exit(0)
	case <-time.After(30 * time.Second):
		signal.Stop(signalCh)
		os.Exit(2)
	}
}

func TestDaemonGoleakFullCycle(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	signalCh := make(chan os.Signal, 1)
	const waitTimeout = 5 * time.Second

	manager := &goleakSessionManager{
		fakeSessionManager: &fakeSessionManager{
			infos: []*session.Info{{
				ID:    "sess-goleak",
				State: session.StateActive,
			}},
		},
	}
	d := newTestDaemon(t, homePaths, &cfg)
	d.signalCh = signalCh
	d.openRegistry = func(context.Context, string) (Registry, error) {
		return &recordingRegistry{path: homePaths.DatabaseFile}, nil
	}
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return manager, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(context.Background())
	}()

	select {
	case <-d.readyCh:
	case err := <-errCh:
		t.Fatalf("Run() exited before signaling ready: %v", err)
	case <-time.After(waitTimeout):
		t.Fatal("Run() did not signal ready")
	}

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	process, err := subprocess.Launch(testutil.Context(t), subprocess.LaunchConfig{
		Command:          bin,
		Args:             []string{"-test.run=TestDaemonGoleakHelperProcess"},
		Env:              append(os.Environ(), daemonGoleakHelperEnv+"=1"),
		DisableTransport: true,
		ShutdownTimeout:  250 * time.Millisecond,
		PostSignalGrace:  50 * time.Millisecond,
		Logger:           discardLogger(),
	})
	if err != nil {
		t.Fatalf("subprocess.Launch(helper) error = %v", err)
	}
	manager.setProcess(process)
	t.Logf("managed_subprocess_pid=%d", process.PID())

	signalCh <- syscall.SIGTERM
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run() shutdown error = %v", err)
		}
	case <-time.After(waitTimeout):
		t.Fatal("Run() did not stop after signal")
	}
	if !manager.stopped() {
		t.Fatal("session manager did not stop the managed subprocess during daemon shutdown")
	}
	select {
	case <-process.Done():
	case <-time.After(waitTimeout):
		t.Fatal("managed subprocess did not exit after daemon shutdown")
	}
	if err := process.Wait(); err != nil {
		t.Fatalf("managed subprocess wait error = %v; stderr=%s", err, process.Stderr())
	}
}

func TestRunShutsDownWhenObserverRetentionStartFails(t *testing.T) {
	t.Parallel()

	t.Run("ShouldShutDownWhenObserverRetentionStartFails", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		retentionErr := errors.New("retention start failed")
		observer := &failingRetentionObserver{startErr: retentionErr}
		httpShutdown := false
		udsShutdown := false

		d := newTestDaemon(t, homePaths, &cfg)
		d.acquireLock = func(path string, _ int) (*Lock, error) {
			return &Lock{path: path}, nil
		}
		d.openRegistry = func(context.Context, string) (Registry, error) {
			return &recordingRegistry{path: homePaths.DatabaseFile}, nil
		}
		d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return observer, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http", onShutdown: func() { httpShutdown = true }}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds", onShutdown: func() { udsShutdown = true }}, nil
		}

		err := d.Run(context.Background())
		if !errors.Is(err, retentionErr) {
			t.Fatalf("Run() error = %v, want retention start failure", err)
		}
		if !observer.shutdownCalled {
			t.Fatal("observer.ShutdownRetention() was not called")
		}
		if !httpShutdown || !udsShutdown {
			t.Fatalf("server shutdown flags = http:%v uds:%v, want both true", httpShutdown, udsShutdown)
		}
	})
}

func TestBoundariesUsesConfiguredRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module github.com/compozy/agh\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(internal) error = %v", err)
	}

	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.boundaryRoot = root

	if err := d.Boundaries(testutil.Context(t)); err != nil {
		t.Fatalf("Boundaries() error = %v", err)
	}
}

func TestBoundariesReturnsViolations(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module github.com/compozy/agh\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}
	violatingDir := filepath.Join(root, "internal", "worker")
	if err := os.MkdirAll(violatingDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(violatingDir) error = %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(violatingDir, "worker.go"),
		[]byte("package worker\n\nimport _ \"github.com/compozy/agh/internal/cli\"\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(worker.go) error = %v", err)
	}

	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.boundaryRoot = root

	if err := d.Boundaries(testutil.Context(t)); err == nil {
		t.Fatal("Boundaries() error = nil, want violation")
	}
}

func TestBoundariesUsesWorkingDirectoryWhenRootUnset(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "go.mod"),
		[]byte("module github.com/compozy/agh\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(internal) error = %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(root) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := d.Boundaries(testutil.Context(t)); err != nil {
		t.Fatalf("Boundaries() error = %v", err)
	}
}

func TestLoadConfigFromHomeAppliesOverlayAndNormalizesSocket(t *testing.T) {
	homePaths := testHomePaths(t)
	if err := os.WriteFile(
		homePaths.ConfigFile,
		[]byte("[daemon]\nsocket = \"~/agh-test.sock\"\n[http]\nport = 4242\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(config) error = %v", err)
	}

	cfg, err := loadConfigFromHome(homePaths)
	if err != nil {
		t.Fatalf("loadConfigFromHome() error = %v", err)
	}
	if got, want := cfg.HTTP.Port, 4242; got != want {
		t.Fatalf("cfg.HTTP.Port = %d, want %d", got, want)
	}
	if !strings.Contains(cfg.Daemon.Socket, "agh-test.sock") || !filepath.IsAbs(cfg.Daemon.Socket) {
		t.Fatalf("cfg.Daemon.Socket = %q, want expanded absolute path", cfg.Daemon.Socket)
	}
}

func TestLoadConfigFromHomeValidationError(t *testing.T) {
	homePaths := testHomePaths(t)
	if err := os.WriteFile(
		homePaths.ConfigFile,
		[]byte("[http]\nport = 70000\n"),
		0o644,
	); err != nil {
		t.Fatalf("os.WriteFile(config) error = %v", err)
	}

	if _, err := loadConfigFromHome(homePaths); err == nil {
		t.Fatal("loadConfigFromHome(invalid config) error = nil, want non-nil")
	}
}

func TestShouldVerifyBoundariesFromEnv(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.getenv = func(string) string { return "yes" }
	if !d.shouldVerifyBoundaries() {
		t.Fatal("shouldVerifyBoundaries() = false, want true")
	}
	d.verifyBoundaries = false
	d.getenv = func(string) string { return "" }
	if d.shouldVerifyBoundaries() {
		t.Fatal("shouldVerifyBoundaries() = true, want false")
	}
	d.getenv = nil
	if d.shouldVerifyBoundaries() {
		t.Fatal("shouldVerifyBoundaries() with nil getenv = true, want false")
	}
	d.verifyBoundaries = true
	if !d.shouldVerifyBoundaries() {
		t.Fatal("shouldVerifyBoundaries() with explicit option = false, want true")
	}
}

func TestSignalSourceDefaultsToOSSignalRegistration(t *testing.T) {
	d, err := New(WithLogger(discardLogger()))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ch, stop := d.signalSource()
	if ch == nil {
		t.Fatal("signalSource() bridge = nil")
	}
	stop()
}

func TestBootInjectsComposedAssemblerForFeatureFlagCombinations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		memoryEnabled bool
		skillsEnabled bool
		wantMemory    bool
		wantSkills    bool
	}{
		{
			name:          "memory on and skills on",
			memoryEnabled: true,
			skillsEnabled: true,
			wantMemory:    true,
			wantSkills:    true,
		},
		{
			name:          "memory on and skills off",
			memoryEnabled: true,
			skillsEnabled: false,
			wantMemory:    true,
			wantSkills:    false,
		},
		{
			name:          "memory off and skills on",
			memoryEnabled: false,
			skillsEnabled: true,
			wantMemory:    false,
			wantSkills:    true,
		},
		{
			name:          "memory off and skills off",
			memoryEnabled: false,
			skillsEnabled: false,
			wantMemory:    false,
			wantSkills:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths := testHomePaths(t)
			cloneDaemonTestStoreSeed(t, homePaths.DatabaseFile)
			cfg := testConfig(t, homePaths)
			cfg.Memory.Enabled = tc.memoryEnabled
			cfg.Skills.Enabled = tc.skillsEnabled
			cfg.Memory.GlobalDir = filepath.Join(homePaths.HomeDir, "custom-memory")

			d := newTestDaemon(t, homePaths, &cfg)

			var capturedDeps SessionManagerDeps
			d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
				capturedDeps = deps
				return &fakeSessionManager{}, nil
			}
			d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
				return &fakeObserver{}, nil
			}
			d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
				return &fakeServer{name: "http"}, nil
			}
			d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
				return &fakeServer{name: "uds"}, nil
			}

			if err := d.boot(testutil.Context(t)); err != nil {
				t.Fatalf("boot() error = %v", err)
			}
			t.Cleanup(func() {
				if err := d.Shutdown(testutil.Context(t)); err != nil {
					t.Fatalf("Shutdown() error = %v", err)
				}
			})

			if capturedDeps.PromptAssembler == nil {
				t.Fatal("boot() did not inject the composed prompt assembler")
			}
			if _, ok := capturedDeps.PromptAssembler.(session.StartupPromptAssembler); !ok {
				t.Fatal("boot() did not inject a startup-aware prompt assembler")
			}
			if capturedDeps.StartupPromptOverlay == nil {
				t.Fatal("boot() did not inject the AGH runtime startup prompt overlay")
			}
			if capturedDeps.PromptInputAugmenter == nil {
				t.Fatal("boot() did not inject the prompt input augmenter")
			}
			if capturedDeps.WorkspaceResolver == nil {
				t.Fatal("boot() did not inject the workspace resolver")
			}
			if d.workspaceResolver == nil {
				t.Fatal("boot() did not retain the workspace resolver")
			}
			if got := d.memoryStore != nil; got != tc.wantMemory {
				t.Fatalf("memory store initialized = %t, want %t", got, tc.wantMemory)
			}
			if got := d.skillsRegistry != nil; got != tc.wantSkills {
				t.Fatalf("skills registry initialized = %t, want %t", got, tc.wantSkills)
			}

			workspace := filepath.Join(t.TempDir(), "workspace")
			writeDaemonMemoryIndex(t, cfg.Memory.GlobalDir, workspace)

			workspaceRef := workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{RootDir: workspace},
				Agents:    []aghconfig.AgentDef{testPromptAgent("Base prompt.")},
			}
			prompt, err := capturedDeps.PromptAssembler.Assemble(
				context.Background(),
				testPromptAgent("Base prompt."),
				&workspaceRef,
			)
			if err != nil {
				t.Fatalf("PromptAssembler.Assemble() error = %v", err)
			}

			assertPromptContainsInOrder(t, prompt, orderedFragments(tc.wantMemory, tc.wantSkills)...)
			assertPromptExcludes(t, prompt, excludedFragments(tc.wantMemory, tc.wantSkills)...)

			if tc.wantMemory {
				if info, err := os.Stat(cfg.Memory.GlobalDir); err != nil {
					t.Fatalf("stat memory.global_dir error = %v", err)
				} else if !info.IsDir() {
					t.Fatalf("memory.global_dir mode = %v, want directory", info.Mode())
				}
			}
			if tc.wantSkills {
				if skills := d.skillsRegistry.List(); len(skills) == 0 {
					t.Fatal("skills registry list = empty, want bundled skills")
				}
			}
		})
	}
}

func TestBootCreatesWorkspaceResolverAndInjectsSessionManager(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)

	var capturedDeps SessionManagerDeps
	var capturedUDSDeps RuntimeDeps
	sessions := &fakeSessionManager{}
	d := newTestDaemon(t, homePaths, &cfg)
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		capturedDeps = deps
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
		capturedUDSDeps = deps
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	if d.workspaceResolver == nil {
		t.Fatal("boot() did not create the daemon workspace resolver")
	}
	if capturedDeps.WorkspaceResolver == nil {
		t.Fatal("boot() did not inject the session manager workspace resolver")
	}
	if capturedDeps.SandboxRegistry == nil {
		t.Fatal("boot() did not inject the session manager sandbox registry")
	}
	if capturedDeps.SessionCompaction != cfg.Session.Compaction {
		t.Fatalf(
			"session compaction config = %#v, want %#v",
			capturedDeps.SessionCompaction,
			cfg.Session.Compaction,
		)
	}
	if sessions.compactionHandler == nil {
		t.Fatal("boot() did not bind the checkpoint compaction runtime")
	}
	if d.sandboxRegistry == nil {
		t.Fatal("boot() did not retain the daemon sandbox registry")
	}
	if capturedUDSDeps.WorkspaceService == nil {
		t.Fatal("boot() did not inject the uds workspace service")
	}
	if capturedUDSDeps.WorkspaceService != d.workspaceResolver {
		t.Fatal("boot() injected a different workspace service into uds")
	}

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	resolved := resolveDaemonWorkspace(t, capturedDeps.WorkspaceResolver, workspaceRoot)
	if got, want := resolved.RootDir, canonicalDaemonRoot(t, workspaceRoot); got != want {
		t.Fatalf("resolved workspace root = %q, want %q", got, want)
	}
}

func TestWorkspaceRegistrationRefreshesHookBindings(t *testing.T) {
	t.Parallel()

	t.Run("Should refresh config hooks after a workspace is registered", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		cloneDaemonTestStoreSeed(t, homePaths.DatabaseFile)
		cfg := testConfig(t, homePaths)
		cfg.Memory.Enabled = false
		cfg.Skills.Enabled = false
		cfg.Automation.Enabled = false

		d := newTestDaemon(t, homePaths, &cfg)
		d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
			return &fakeSessionManager{}, nil
		}
		d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
			return &fakeObserver{}, nil
		}
		d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "http"}, nil
		}
		d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
			return &fakeServer{name: "uds"}, nil
		}

		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		})

		hooksRuntime, ok := d.hooks.(*hookspkg.Hooks)
		if !ok {
			t.Fatalf("daemon hooks runtime = %T, want *hooks.Hooks", d.hooks)
		}

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		writeDaemonFile(t, filepath.Join(workspaceRoot, ".agh", "config.toml"), `
[[hooks.declarations]]
name = "workspace-register-hook"
event = "session.post_create"
mode = "sync"
command = "/bin/sh"
args = ["-c", "printf '{}'"]
`)

		resolved, err := d.workspaceResolver.ResolveOrRegister(testutil.Context(t), workspaceRoot)
		if err != nil {
			t.Fatalf("ResolveOrRegister() error = %v", err)
		}

		waitForCondition(t, "workspace hook binding refresh", func() bool {
			entries, catalogErr := hooksRuntime.Catalog(hookspkg.CatalogFilter{
				WorkspaceID: resolved.WorkspaceID,
				Event:       hookspkg.HookSessionPostCreate,
			})
			if catalogErr != nil {
				t.Fatalf("Catalog() error = %v", catalogErr)
			}
			for _, entry := range entries {
				if entry.Name == "workspace-register-hook" && entry.Source == hookspkg.HookSourceConfig {
					return true
				}
			}
			return false
		})
	})
}

func TestBootSkillsWatcherRefreshesOnGlobalChangesAndStopsOnShutdown(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Enabled = false
	cfg.Skills.Enabled = true
	cfg.Skills.PollInterval = 10 * time.Millisecond

	d := newTestDaemon(t, homePaths, &cfg)
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}

	registry := d.skillsRegistry
	if registry == nil {
		t.Fatal("boot() did not initialize the skills registry")
	}
	skillsDone := d.skillsDone
	if skillsDone == nil {
		t.Fatal("boot() did not start the skills watcher")
	}

	writeDaemonSkill(t, homePaths.SkillsDir, "watched-skill", "Global watched skill")
	waitForCondition(t, "watcher refresh after boot", func() bool {
		_, ok := registry.Get("watched-skill")
		return ok
	})

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	select {
	case <-skillsDone:
	default:
		t.Fatal("skills watcher was still running after shutdown")
	}
	versionAfterShutdown := registry.GlobalVersion()

	writeDaemonSkill(
		t,
		homePaths.SkillsDir,
		"after-shutdown",
		"Should not be observed",
	)

	if got := registry.GlobalVersion(); got != versionAfterShutdown {
		t.Fatalf("registry version after shutdown file write = %d, want %d", got, versionAfterShutdown)
	}
	if _, ok := registry.Get("after-shutdown"); ok {
		t.Fatal("skills watcher continued refreshing after shutdown")
	}
}

func TestShutdownStopsSkillsWatcherBeforeSessions(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Enabled = false
	cfg.Skills.Enabled = true
	cfg.Skills.PollInterval = 10 * time.Millisecond

	var skillsDone <-chan struct{}
	sessions := &fakeSessionManager{
		infos: []*session.Info{{ID: "sess-a"}},
		onStop: func(string) {
			select {
			case <-skillsDone:
			default:
				t.Error("skills watcher was still running when session shutdown started")
			}
		},
	}

	d := newTestDaemon(t, homePaths, &cfg)
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	if err := d.boot(testutil.Context(t)); err != nil {
		t.Fatalf("boot() error = %v", err)
	}
	skillsDone = d.skillsDone
	if skillsDone == nil {
		t.Fatal("boot() did not start the skills watcher")
	}

	if err := d.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestSkillsRegistryConfigUsesDaemonHomeAndDisabledSkills(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Skills.DisabledSkills = []string{"alpha", "beta"}

	d := newTestDaemon(t, homePaths, &cfg)

	registryCfg := d.skillsRegistryConfig(&cfg)
	if registryCfg.BundledFS == nil {
		t.Fatal("skillsRegistryConfig() BundledFS = nil")
	}
	if got, want := registryCfg.UserSkillsDir, homePaths.SkillsDir; got != want {
		t.Fatalf("skillsRegistryConfig() UserSkillsDir = %q, want %q", got, want)
	}
	if got, want := registryCfg.UserAgentsDir, homePaths.AgentsDir; got != want {
		t.Fatalf("skillsRegistryConfig() UserAgentsDir = %q, want %q", got, want)
	}
	if got := registryCfg.DisabledSkills; len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("skillsRegistryConfig() DisabledSkills = %#v, want [alpha beta]", got)
	}
}

func TestRunConfiguresDreamRuntimeForLiveRoleLifecycle(t *testing.T) {
	t.Parallel()
	runDreamRuntimeLifecycleCases(t)
}

func runDreamRuntimeLifecycleCases(t *testing.T) {
	t.Helper()

	testCases := []struct {
		name        string
		patch       func(*aghconfig.Config)
		wantRuntime bool
	}{
		{
			name: "Should keep the dream runtime absent when memory is disabled",
			patch: func(cfg *aghconfig.Config) {
				cfg.Memory.Enabled = false
			},
		},
		{
			name: "Should retain the dream runtime for live role enablement",
			patch: func(cfg *aghconfig.Config) {
				cfg.Roles.Dream.Enabled = false
			},
			wantRuntime: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			homePaths := testHomePaths(t)
			cfg := testConfig(t, homePaths)
			tc.patch(&cfg)

			d := newTestDaemon(t, homePaths, &cfg)
			d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
				return &fakeSessionManager{}, nil
			}
			d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
				return &fakeObserver{}, nil
			}
			d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
				return &fakeServer{name: "http"}, nil
			}
			d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
				return &fakeServer{name: "uds"}, nil
			}

			runCtx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error, 1)
			go func() {
				errCh <- d.Run(runCtx)
			}()

			<-d.readyCh
			waitForCondition(t, "dream runtime lifecycle applied", func() bool {
				d.mu.Lock()
				defer d.mu.Unlock()
				return (d.dreamRuntime != nil) == tc.wantRuntime
			})
			d.mu.Lock()
			dreamRuntime := d.dreamRuntime
			d.mu.Unlock()
			if tc.wantRuntime && !dreamRuntime.Enabled() {
				t.Fatal("dream runtime Enabled() = false, want memory-backed scheduling for workspace role resolution")
			}

			cancel()
			if err := <-errCh; err != nil {
				t.Fatalf("Run() error = %v", err)
			}
		})
	}
}

func TestDreamTickerRunsAndStopsOnCancellation(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Dream.CheckInterval = 10 * time.Millisecond

	dream := &fakeDreamService{shouldRun: true}
	d := newTestDaemon(t, homePaths, &cfg)
	d.newSessionManager = func(context.Context, SessionManagerDeps) (SessionManager, error) {
		return &fakeSessionManager{}, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.newDreamService = func(_ ...memory.Option) consolidation.Service {
		return dream
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(runCtx)
	}()

	<-d.readyCh
	waitForCondition(t, "dream loop started", func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.dreamRuntime != nil
	})
	waitForCondition(t, "dream ticker run", func() bool {
		return dream.runCount() > 0
	})

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	d.mu.Lock()
	dreamRuntime := d.dreamRuntime
	d.mu.Unlock()
	if dreamRuntime != nil {
		t.Fatal("dream runtime still attached after daemon shutdown")
	}
}

func TestSessionStopNotifierQueuesDreamCheck(t *testing.T) {
	t.Parallel()

	homePaths := testHomePaths(t)
	cfg := testConfig(t, homePaths)
	cfg.Memory.Dream.CheckInterval = time.Hour

	workspace := filepath.Join(t.TempDir(), "workspace")
	sessions := &fakeSessionManager{}
	dream := &fakeDreamService{
		shouldRun: true,
		runHook: func(ctx context.Context, spawn memory.SessionSpawner, workspace string) error {
			return spawn(ctx, "memory-consolidation", "session-stop prompt", workspace, time.Time{})
		},
	}
	var dispatcher session.HookSet

	d := newTestDaemon(t, homePaths, &cfg)
	d.newSessionManager = func(_ context.Context, deps SessionManagerDeps) (SessionManager, error) {
		dispatcher = deps.Hooks
		return sessions, nil
	}
	d.newObserver = func(context.Context, RuntimeDeps) (Observer, error) {
		return &fakeObserver{}, nil
	}
	d.newDreamService = func(_ ...memory.Option) consolidation.Service {
		return dream
	}
	d.httpFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "http"}, nil
	}
	d.udsFactory = func(context.Context, RuntimeDeps) (Server, error) {
		return &fakeServer{name: "uds"}, nil
	}

	runCtx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(runCtx)
	}()

	<-d.readyCh
	waitForCondition(t, "dream loop started", func() bool {
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.dreamRuntime != nil
	})
	if dispatcher.Session == nil {
		t.Fatal("session manager hook set = nil")
	}

	resolved := resolveDaemonWorkspace(t, d.workspaceResolver, workspace)
	if _, err := dispatcher.Session.DispatchSessionPostStop(context.Background(), hookspkg.SessionPostStopPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookSessionPostStop,
			Timestamp: time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID:   "sess-user",
			WorkspaceID: resolved.ID,
			SessionType: string(session.SessionTypeUser),
			State:       string(session.StateStopped),
		},
	}); err != nil {
		t.Fatalf("DispatchSessionPostStop() error = %v", err)
	}
	waitForCondition(t, "dream run from session stop", func() bool {
		return dream.runCount() == 1
	})
	waitForCondition(t, "dream session workspace propagated", func() bool {
		return sessions.createCount() == 1
	})
	if got := sessions.createCall(0).Workspace; got != resolved.ID {
		t.Fatalf("Create() workspace = %q, want %q", got, resolved.ID)
	}
	if got := sessions.createCall(0).WorkspacePath; got != "" {
		t.Fatalf("Create() workspace_path = %q, want empty", got)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRemoveStaleSocketBehaviors(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "daemon.sock")
	if err := removeStaleSocket(socketPath); err != nil {
		t.Fatalf("removeStaleSocket(missing) error = %v", err)
	}

	if err := os.WriteFile(socketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(socket) error = %v", err)
	}
	if err := removeStaleSocket(socketPath); err != nil {
		t.Fatalf("removeStaleSocket(file) error = %v", err)
	}

	dirPath := filepath.Join(t.TempDir(), "dir")
	if err := os.MkdirAll(filepath.Join(dirPath, "child"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(dirPath) error = %v", err)
	}
	if err := removeStaleSocket(dirPath); err == nil {
		t.Fatal("removeStaleSocket(dir) error = nil, want non-nil")
	}
}

func TestResolveDaemonPortUsesReporterWhenAvailable(t *testing.T) {
	if got, want := resolveDaemonPort(2123, portReportingServer{port: 9090}), 9090; got != want {
		t.Fatalf("resolveDaemonPort() = %d, want %d", got, want)
	}
	if got, want := resolveDaemonPort(2123, &fakeServer{name: "default"}), 2123; got != want {
		t.Fatalf("resolveDaemonPort(default) = %d, want %d", got, want)
	}
}

func TestListProcessesAndSignalProcess(t *testing.T) {
	processes, err := listProcesses(testutil.Context(t))
	if err != nil {
		t.Fatalf("listProcesses() error = %v", err)
	}
	if len(processes) == 0 {
		t.Fatal("listProcesses() returned no processes")
	}

	if err := procutil.Signal(os.Getpid(), syscall.Signal(0)); err != nil {
		t.Fatalf("procutil.Signal(self, 0) error = %v", err)
	}
	if err := procutil.Signal(0, syscall.SIGTERM); err == nil {
		t.Fatal("procutil.Signal(invalid pid) error = nil, want non-nil")
	}
}

func TestProcessAliveAndRuntimeLoggerHelpers(t *testing.T) {
	if procutil.Alive(0) {
		t.Fatal("procutil.Alive(0) = true, want false")
	}
	if !procutil.Alive(os.Getpid()) {
		t.Fatal("procutil.Alive(self) = false, want true")
	}
	if procutil.Alive(999999) && runtime.GOOS != "windows" {
		t.Fatal("procutil.Alive(999999) = true, want false")
	}

	d, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got := d.runtimeLogger(); got == nil {
		t.Fatal("runtimeLogger() = nil")
	}
}

func TestInfoValidationAndReadFailures(t *testing.T) {
	if err := (Info{}).Validate(); err == nil {
		t.Fatal("Info.Validate() error = nil, want non-nil")
	}
	if err := (Info{PID: 1, Port: -1, StartedAt: time.Now().UTC()}).Validate(); err == nil {
		t.Fatal("Info.Validate(invalid port) error = nil, want non-nil")
	}
	if err := (Info{PID: 1, Port: 1, StartedAt: time.Now().UTC()}).Validate(); err != nil {
		t.Fatalf("Info.Validate(valid) error = %v", err)
	}
	if err := WriteInfo("", Info{}); err == nil {
		t.Fatal("WriteInfo(blank path) error = nil, want non-nil")
	}
	if err := RemoveInfo(""); err != nil {
		t.Fatalf("RemoveInfo(blank path) error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{bad"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(bad.json) error = %v", err)
	}
	if _, err := ReadInfo(path); err == nil {
		t.Fatal("ReadInfo(invalid JSON) error = nil, want non-nil")
	}
	if _, err := ReadInfo(""); err == nil {
		t.Fatal("ReadInfo(blank path) error = nil, want non-nil")
	}

	validPath := filepath.Join(t.TempDir(), "nested", "daemon.json")
	validInfo := Info{PID: 10, Port: 20, StartedAt: time.Now().UTC()}
	if err := WriteInfo(validPath, validInfo); err != nil {
		t.Fatalf("WriteInfo(valid path) error = %v", err)
	}
	if err := syncDir(filepath.Dir(validPath)); err != nil {
		t.Fatalf("syncDir(valid dir) error = %v", err)
	}
	if err := RemoveInfo(filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatalf("RemoveInfo(missing file) error = %v", err)
	}
}

func TestDaemonNetworkInfoHelpersValidateAndRedactRuntimeStatus(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)

	if err := (NetworkInfo{}).Validate(); err == nil {
		t.Fatal("NetworkInfo.Validate() error = nil, want non-nil")
	}
	if err := (NetworkInfo{Enabled: true, Status: network.StatusActive}).Validate(); err != nil {
		t.Fatalf("NetworkInfo.Validate(valid) error = %v", err)
	}
	for _, invalid := range []NetworkInfo{
		{Status: network.StatusActive},
		{Status: network.StatusReady},
		{Enabled: true, Status: network.StatusDisabled},
		{Enabled: true, Status: "degraded"},
	} {
		t.Run("Should reject inconsistent or unsupported network status "+invalid.Status, func(t *testing.T) {
			t.Parallel()
			if err := invalid.Validate(); err == nil {
				t.Fatalf("NetworkInfo.Validate(%#v) error = nil, want non-nil", invalid)
			}
		})
	}

	disabledInfo, err := daemonNetworkInfo(ctx, aghconfig.NetworkConfig{}, nil)
	if err != nil {
		t.Fatalf("daemonNetworkInfo(disabled) error = %v", err)
	}
	if disabledInfo == nil || disabledInfo.Enabled || disabledInfo.Status != network.StatusDisabled {
		t.Fatalf("daemonNetworkInfo(disabled) = %#v, want disabled snapshot", disabledInfo)
	}

	if _, err := daemonNetworkInfo(ctx, aghconfig.NetworkConfig{Enabled: true}, nil); err == nil {
		t.Fatal("daemonNetworkInfo(enabled nil service) error = nil, want non-nil")
	}
	if _, err := daemonNetworkInfo(ctx, aghconfig.NetworkConfig{Enabled: true}, &fakeNetworkRuntime{}); err == nil {
		t.Fatal("daemonNetworkInfo(nil status) error = nil, want non-nil")
	}

	info, err := daemonNetworkInfo(ctx, aghconfig.NetworkConfig{Enabled: true}, &fakeNetworkRuntime{
		status: &network.Status{
			Enabled: true,
			Status:  " active ",
		},
	})
	if err != nil {
		t.Fatalf("daemonNetworkInfo(runtime status) error = %v", err)
	}
	if info == nil {
		t.Fatal("daemonNetworkInfo(runtime status) = nil, want populated diagnostics")
		return
	}
	if !info.Enabled || info.Status != network.StatusActive {
		t.Fatalf("daemonNetworkInfo(runtime status) = %#v, want trimmed active diagnostics", info)
	}
}

func TestLockHelpersAndErrors(t *testing.T) {
	lock := &Lock{path: "/tmp/daemon.lock"}
	if got := lock.Path(); got != "/tmp/daemon.lock" {
		t.Fatalf("lock.Path() = %q, want %q", got, "/tmp/daemon.lock")
	}

	err := errAlreadyRunning{pid: 42}
	if !strings.Contains(err.Error(), "42") {
		t.Fatalf("errAlreadyRunning.Error() = %q, want pid in message", err.Error())
	}
	if got := (errAlreadyRunning{}).Error(); got != ErrAlreadyRunning.Error() {
		t.Fatalf("errAlreadyRunning{}.Error() = %q, want %q", got, ErrAlreadyRunning.Error())
	}
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("errors.Is(errAlreadyRunning, ErrAlreadyRunning) = false, want true")
	}

	if _, err := AcquireLock("", 1); err == nil {
		t.Fatal("AcquireLock(blank path) error = nil, want non-nil")
	}
	if _, err := AcquireLock(filepath.Join(t.TempDir(), "daemon.lock"), 0); err == nil {
		t.Fatal("AcquireLock(invalid pid) error = nil, want non-nil")
	}

	released := false
	if err := (&Lock{releaseFn: func() error { released = true; return nil }}).Release(); err != nil {
		t.Fatalf("Lock.Release(releaseFn) error = %v", err)
	}
	if !released {
		t.Fatal("Lock.Release() did not use injected release function")
	}
	if got := ((*Lock)(nil)).Path(); got != "" {
		t.Fatalf("nil lock Path() = %q, want empty", got)
	}
	if got := ((*Lock)(nil)).StalePID(); got != 0 {
		t.Fatalf("nil lock StalePID() = %d, want 0", got)
	}
	if err := ((*Lock)(nil)).Release(); err != nil {
		t.Fatalf("nil lock Release() error = %v, want nil", err)
	}

	path := filepath.Join(t.TempDir(), "pid.lock")
	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(pid.lock) error = %v", err)
	}
	if got, err := readLockPID(path); err != nil {
		t.Fatalf("readLockPID(invalid contents) error = %v", err)
	} else if got != 0 {
		t.Fatalf("readLockPID(invalid contents) = %d, want 0", got)
	}
	if err := writeLockPID(path, 0); err != nil {
		t.Fatalf("writeLockPID(0) error = %v", err)
	}
	if data, err := os.ReadFile(path); err != nil {
		t.Fatalf("os.ReadFile(pid.lock) error = %v", err)
	} else if string(data) != "0\n" {
		t.Fatalf("writeLockPID(0) contents = %q, want 0 newline", string(data))
	}
}

func waitForCondition(t *testing.T, label string, fn func() bool) {
	t.Helper()
	waitForConditionWithin(t, label, 2*time.Second, fn)
}

func waitForConditionWithin(t *testing.T, label string, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", label)
}

func testHomePaths(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	homePaths.DaemonSocket = shortSocketPath(t)
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func testConfig(t *testing.T, homePaths aghconfig.HomePaths) aghconfig.Config {
	t.Helper()

	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)
	cfg.Daemon.Socket = homePaths.DaemonSocket
	cfg.Automation.Enabled = false
	return cfg
}

func testConfigPtr(t *testing.T, homePaths aghconfig.HomePaths) *aghconfig.Config {
	t.Helper()

	cfg := testConfig(t, homePaths)
	return &cfg
}

func writeDaemonMemoryIndex(t *testing.T, globalDir string, workspace string) {
	t.Helper()

	writeDaemonFile(
		t,
		filepath.Join(globalDir, "global.md"),
		memoryDocument("Global", "global note", memcontract.TypeUser, "global note"),
	)
	writeDaemonFile(t, filepath.Join(globalDir, "MEMORY.md"), "- [Global](global.md) - global note")
	writeDaemonFile(
		t,
		filepath.Join(workspace, aghconfig.DirName, "memory", "workspace.md"),
		memoryDocument("Workspace", "workspace note", memcontract.TypeProject, "workspace note"),
	)
	writeDaemonFile(
		t,
		filepath.Join(workspace, aghconfig.DirName, "memory", "MEMORY.md"),
		"- [Workspace](workspace.md) - workspace note",
	)
}

func memoryDocument(name string, description string, memoryType memcontract.Type, body string) string {
	return strings.TrimSpace(strings.Join([]string{
		"---",
		"name: " + name,
		"description: " + description,
		"type: " + string(memoryType),
		"---",
		"",
		body,
	}, "\n")) + "\n"
}

func writeDaemonSkill(t *testing.T, root string, name string, description string) {
	t.Helper()

	content := fmt.Sprintf(`---
name: %s
description: %s
---

# %s
`, name, description, name)
	writeDaemonFile(t, filepath.Join(root, name, "SKILL.md"), content)
}

func writeDaemonFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func resolveDaemonWorkspace(
	t *testing.T,
	resolver workspacepkg.RuntimeResolver,
	root string,
) workspacepkg.ResolvedWorkspace {
	t.Helper()

	if resolver == nil {
		t.Fatal("workspace resolver = nil")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", root, err)
	}

	resolved, err := resolver.ResolveOrRegister(testutil.Context(t), root)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", root, err)
	}
	return resolved
}

func canonicalDaemonRoot(t *testing.T, root string) string {
	t.Helper()

	evaluated, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", root, err)
	}
	canonical, err := filepath.Abs(evaluated)
	if err != nil {
		t.Fatalf("filepath.Abs(%q) error = %v", evaluated, err)
	}
	return canonical
}

func orderedFragments(wantMemory bool, wantSkills bool) []string {
	fragments := make([]string, 0, 6)
	fragments = append(fragments, aghRuntimeEnvelopeStart, "# AGH Runtime")
	if wantMemory {
		fragments = append(fragments, "# Persistent Memory")
	}
	fragments = append(fragments, "Base prompt.")
	if wantSkills {
		fragments = append(fragments, "<available-skills>", "agh")
	}
	fragments = append(fragments, "# Tools And Skills", "# Native Tools")
	return fragments
}

func excludedFragments(wantMemory bool, wantSkills bool) []string {
	fragments := make([]string, 0, 2)
	if !wantMemory {
		fragments = append(fragments, "# Persistent Memory")
	}
	if !wantSkills {
		fragments = append(fragments, "<available-skills>")
	}
	return fragments
}

func assertPromptContainsInOrder(t *testing.T, prompt string, fragments ...string) {
	t.Helper()

	searchFrom := 0
	for _, fragment := range fragments {
		if fragment == "" {
			continue
		}

		offset := strings.Index(prompt[searchFrom:], fragment)
		if offset < 0 {
			t.Fatalf("prompt %q does not contain %q", prompt, fragment)
		}
		searchFrom += offset + len(fragment)
	}
}

func assertPromptExcludes(t *testing.T, prompt string, fragments ...string) {
	t.Helper()

	for _, fragment := range fragments {
		if fragment == "" {
			continue
		}
		if strings.Contains(prompt, fragment) {
			t.Fatalf("prompt %q contains unexpected fragment %q", prompt, fragment)
		}
	}
}

func newTestDaemon(t *testing.T, homePaths aghconfig.HomePaths, cfg *aghconfig.Config) *Daemon {
	t.Helper()

	if _, err := os.Stat(homePaths.DatabaseFile); errors.Is(err, os.ErrNotExist) {
		if err := daemonTestStoreSeed.Clone(homePaths.DatabaseFile); err != nil {
			t.Fatalf("daemon store seed Clone() error = %v", err)
		}
	} else if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", homePaths.DatabaseFile, err)
	}
	d, err := New(
		WithHomePaths(homePaths),
		WithConfig(cfg),
		WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	d.getenv = func(key string) string {
		if key == "HOME" {
			return homePaths.HomeDir
		}
		return os.Getenv(key)
	}
	return d
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func strconvString(v int) string {
	return fmt.Sprintf("%d", v)
}

func shortSocketPath(t *testing.T) string {
	t.Helper()

	path := filepath.Join(os.TempDir(), fmt.Sprintf("agh-%d.sock", time.Now().UTC().UnixNano()))
	t.Cleanup(func() {
		_ = os.Remove(path)
	})
	return path
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	return testutil.FreeTCPPort(t)
}

func TestDetachedHarnessDaemonScenarios(t *testing.T) {
	testCases := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "ShouldAllowProcessedReentryMetadataForDuplicateDetachedHarnessSubmission",
			run:  testTaskRuntimeDetachedHarnessSubmissionAllowsProcessedReentryMetadata,
		},
		{
			name: "ShouldScheduleRescanWhenHarnessReentryQueueIsFull",
			run:  testHarnessReentryBridgeOnTaskEventSchedulesRescanWhenQueueIsFull,
		},
		{
			name: "ShouldRecoverEqualTimestampRunsByTerminalSequence",
			run:  testHarnessReentryBridgeRecoverOrdersEqualTimestampsByTerminalSequence,
		},
		{
			name: "ShouldCancelBlockedStatusLookupOnBridgeShutdown",
			run:  testHarnessReentryBridgeShutdownCancelsBlockedStatusLookup,
		},
		{
			name: "ShouldFinalizeHungSyntheticWakeOnBridgeShutdown",
			run:  testHarnessReentryBridgeShutdownFinalizesHungSyntheticWake,
		},
		{
			name: "ShouldFilterFallbackSectionsWithoutDuplicatesOrDisabledProviders",
			run:  testSectionSelectorFallbackStillFiltersProvidersAndDuplicates,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tt.run(t)
		})
	}
}

func testTaskRuntimeDetachedHarnessSubmissionAllowsProcessedReentryMetadata(t *testing.T) {
	sessions := &fakeSessionManager{}
	runtime, resolver, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	workspace := resolveDaemonWorkspace(t, resolver, filepath.Join(t.TempDir(), "workspace"))
	sessions.infos = []*session.Info{
		{
			ID:                   "sess-owner",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
		{
			ID:                   "sess-wake",
			Type:                 session.SessionTypeSystem,
			State:                session.StateActive,
			WorkspaceID:          workspace.ID,
			Workspace:            workspace.RootDir,
			NetworkParticipation: daemonTestLiveParticipation(workspace.ID, "builders"),
		},
	}

	req := detachedHarnessSubmitRequest{
		SubmissionKey:  "detached-reentry-match",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    workspace.ID,
		Summary:        "Processed detached work",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	}

	first, err := runtime.submitDetachedHarnessWork(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(first) error = %v", err)
	}

	completeDetachedHarnessRunForTest(t, runtime, first.Run.ID, "sess-owner")
	waitForDetachedHarnessReentryState(t, runtime, first.Run.ID, harnessReentryOutcomeEmitted)

	second, err := runtime.submitDetachedHarnessWork(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("submitDetachedHarnessWork(duplicate after reentry) error = %v", err)
	}
	if got := second.ExistingTask; !got {
		t.Fatal("duplicate submission ExistingTask = false, want true")
	}
	if got := second.ExistingRun; !got {
		t.Fatal("duplicate submission ExistingRun = false, want true")
	}
	if got, want := second.Run.ID, first.Run.ID; got != want {
		t.Fatalf("duplicate submission run id = %q, want %q", got, want)
	}
}

func testHarnessReentryBridgeOnTaskEventSchedulesRescanWhenQueueIsFull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	record := taskpkg.EventRecord{
		Sequence: 1,
		Event: taskpkg.Event{
			TaskID:    "task-1",
			RunID:     "run-1",
			EventType: harnessTaskEventRunCompleted,
		},
	}
	bridge := &harnessReentryBridge{
		ctx:        ctx,
		cancel:     cancel,
		logger:     discardLogger(),
		events:     make(chan taskpkg.EventRecord, 1),
		rescan:     make(chan struct{}, 1),
		processing: make(map[string]struct{}),
		queues:     make(map[string]*harnessWakeQueue),
	}
	bridge.events <- record

	done := make(chan struct{})
	go func() {
		bridge.OnTaskEvent(context.Background(), record)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("OnTaskEvent() blocked on a full queue, want immediate return")
	}

	select {
	case <-bridge.rescan:
	default:
		t.Fatal("overflow did not schedule a recovery rescan")
	}
}

func testHarnessReentryBridgeRecoverOrdersEqualTimestampsByTerminalSequence(t *testing.T) {
	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})
	db := openDaemonTestGlobalDB(t)
	if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID: "ws-1", RootDir: t.TempDir(), Name: "detached-recovery",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("InsertWorkspace(detached recovery) error = %v", err)
	}
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{
				ID:                   "sess-owner",
				AgentName:            "coder",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          "ws-1",
				NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
			},
			{
				ID:                   "sess-wake",
				AgentName:            "coder",
				Type:                 session.SessionTypeSystem,
				State:                session.StateActive,
				WorkspaceID:          "ws-1",
				NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
			},
		},
	}
	actor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}

	base := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	seedDetachedHarnessRecoveryRunForTest(
		t,
		db,
		actor,
		"task-b",
		"run-b",
		"first completed run",
		base,
	)
	seedDetachedHarnessRecoveryRunForTest(
		t,
		db,
		actor,
		"task-a",
		"run-a",
		"second completed run",
		base,
	)

	bridge, err := newHarnessReentryBridge(context.Background(), resolver, nil, db, sessions, discardLogger())
	if err != nil {
		t.Fatalf("newHarnessReentryBridge() error = %v", err)
	}
	t.Cleanup(bridge.shutdown)

	if err := bridge.recover(testutil.Context(t)); err != nil {
		t.Fatalf("recover() error = %v", err)
	}
	waitForTaskRuntimeCondition(t, 2*time.Second, func() bool {
		return sessions.syntheticPromptCount() == 2
	})

	sessions.mu.Lock()
	calls := append([]fakeSyntheticPromptCall(nil), sessions.syntheticPromptCalls...)
	sessions.mu.Unlock()

	if got, want := len(calls), 2; got != want {
		t.Fatalf("len(syntheticPromptCalls) = %d, want %d", got, want)
	}
	if got, want := calls[0].opts.Metadata.TaskRunID, "run-b"; got != want {
		t.Fatalf("first recovered synthetic wake run id = %q, want %q", got, want)
	}
	if got, want := calls[1].opts.Metadata.TaskRunID, "run-a"; got != want {
		t.Fatalf("second recovered synthetic wake run id = %q, want %q", got, want)
	}
}

func testHarnessReentryBridgeShutdownFinalizesHungSyntheticWake(t *testing.T) {
	started := make(chan struct{})
	blocked := make(chan acp.AgentEvent)
	sessions := &fakeSessionManager{
		infos: []*session.Info{
			{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
		},
	}
	runtime, _, _ := newDetachedHarnessTaskRuntimeForTest(t, sessions)
	sessions.syntheticPromptHook = func(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error) {
		select {
		case <-started:
		default:
			close(started)
		}
		return blocked, nil
	}

	submission := submitDetachedHarnessWorkForTest(t, runtime, detachedHarnessSubmitRequest{
		SubmissionKey:  "reentry-shutdown-hung-stream",
		OwnerSessionID: "sess-owner",
		Scope:          taskpkg.ScopeGlobal,
		Summary:        "Bridge shutdown during synthetic wake",
		WakeTarget: detachedHarnessWakeTargetInput{
			SessionID: "sess-wake",
		},
	})

	completeDetachedHarnessRunForTest(t, runtime, submission.Run.ID, "sess-owner")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("synthetic wake did not start before shutdown")
	}

	runtime.reentry.shutdown()
	metadata := waitForDetachedHarnessReentryState(t, runtime, submission.Run.ID, harnessReentryOutcomeDropped)
	if got, want := metadata.Reentry.Reason, harnessReentryReasonDispatchFailed; got != want {
		t.Fatalf("metadata.Reentry.Reason = %q, want %q", got, want)
	}
}

func testHarnessReentryBridgeShutdownCancelsBlockedStatusLookup(t *testing.T) {
	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		SyntheticTurnsEnabled:      true,
		DetachedTaskRuntimeEnabled: true,
	})
	db := openDaemonTestGlobalDB(t)
	if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID: "ws-1", RootDir: t.TempDir(), Name: "detached-blocked-status",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("InsertWorkspace(detached blocked status) error = %v", err)
	}
	actor, err := detachedHarnessActorContext("sess-owner")
	if err != nil {
		t.Fatalf("detachedHarnessActorContext() error = %v", err)
	}

	completedAt := time.Date(2026, 4, 19, 4, 0, 0, 0, time.UTC)
	seedDetachedHarnessRecoveryRunForTest(
		t,
		db,
		actor,
		"task-blocked",
		"run-blocked",
		"blocked status lookup",
		completedAt,
	)

	statusStarted := make(chan struct{})
	sessions := &blockingStatusSessionManager{
		fakeSessionManager: &fakeSessionManager{
			infos: []*session.Info{
				{ID: "sess-owner", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
				{ID: "sess-wake", AgentName: "coder", Type: session.SessionTypeSystem, State: session.StateActive},
			},
		},
		blockSessionID: "sess-wake",
		statusStarted:  statusStarted,
	}

	bridge, err := newHarnessReentryBridge(context.Background(), resolver, nil, db, sessions, discardLogger())
	if err != nil {
		t.Fatalf("newHarnessReentryBridge() error = %v", err)
	}
	t.Cleanup(bridge.shutdown)

	bridge.OnTaskEvent(context.Background(), taskpkg.EventRecord{
		Sequence: 1,
		Event: taskpkg.Event{
			TaskID:    "task-blocked",
			RunID:     "run-blocked",
			EventType: harnessTaskEventRunCompleted,
			Timestamp: completedAt,
		},
	})

	select {
	case <-statusStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("bridge worker did not reach the blocked status lookup")
	}

	shutdownDone := make(chan struct{})
	go func() {
		bridge.shutdown()
		close(shutdownDone)
	}()

	select {
	case <-shutdownDone:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown() blocked while a session status lookup ignored cancellation")
	}
}

func testSectionSelectorFallbackStillFiltersProvidersAndDuplicates(t *testing.T) {
	var selector *SectionSelector
	selected, resolved, err := selector.Select(
		session.StartupPromptContext{SessionType: session.SessionTypeUser},
		[]PromptSectionDescriptor{
			{
				Name:     string(HarnessPromptSectionMemory),
				Position: PromptSectionPositionPrepend,
				Order:    10,
				Provider: nil,
			},
			{
				Name:     string(HarnessPromptSectionMemory),
				Position: PromptSectionPositionPrepend,
				Order:    20,
				Provider: staticPromptProvider("memory block"),
			},
			{
				Name:     string(HarnessPromptSectionNetwork),
				Position: PromptSectionPositionAppend,
				Order:    10,
				Provider: staticPromptProvider("network block"),
				Predicate: func(ResolvedHarnessPolicy) bool {
					return false
				},
			},
			{
				Name:     string(HarnessPromptSectionNetwork),
				Position: PromptSectionPositionAppend,
				Order:    20,
				Provider: staticPromptProvider("duplicate network block"),
			},
		},
	)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	gotNames := make([]string, 0, len(selected))
	for _, descriptor := range selected {
		gotNames = append(gotNames, descriptor.Name)
	}
	if got, want := gotNames, []string{
		string(HarnessPromptSectionMemory),
		string(HarnessPromptSectionNetwork),
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("selected descriptor names = %#v, want %#v", got, want)
	}
	if resolved.Surface != "" ||
		resolved.Session.SessionClass != "" ||
		resolved.Turn.Origin != "" ||
		len(resolved.Policy.IncludeSections) != 0 ||
		len(resolved.Policy.EnableAugmenters) != 0 ||
		resolved.Policy.DiagnosticLabel != "" ||
		len(resolved.Policy.ObservabilityTags) != 0 {
		t.Fatalf("resolved fallback context = %#v, want zero-value policy context", resolved)
	}
}

func TestHarnessContextResolverDetachedRunModeRequiresDetachedMetadata(t *testing.T) {
	testCases := []struct {
		name  string
		input HarnessResolutionInput
		want  DetachedRunMode
	}{
		{
			name: "ShouldNotEnableDetachedRunModeForSystemStartupWithoutDetachedMetadata",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceStartup,
				Session: HarnessSessionInput{
					Type: session.SessionTypeSystem,
				},
			},
			want: DetachedRunModeNone,
		},
		{
			name: "ShouldNotEnableDetachedRunModeForSyntheticTurnsWithoutDetachedMetadata",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type: session.SessionTypeSystem,
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceSynthetic,
					Synthetic: &SyntheticTurnMetadata{
						Reason:  "task_complete",
						Trigger: "task.run_completed",
					},
				},
			},
			want: DetachedRunModeNone,
		},
		{
			name: "ShouldEnableDetachedRunModeOnlyWhenDetachedMetadataIsPresent",
			input: HarnessResolutionInput{
				Surface: ResolutionSurfaceTurn,
				Session: HarnessSessionInput{
					Type: session.SessionTypeSystem,
				},
				Turn: HarnessTurnRequest{
					Source: session.TurnSourceSynthetic,
					Synthetic: &SyntheticTurnMetadata{
						Reason:  "task_complete",
						Trigger: "task.run_completed",
					},
					Detached: &DetachedRunMetadata{
						TaskRunID: "run-123",
					},
				},
			},
			want: DetachedRunModeTaskRuntime,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
				SyntheticTurnsEnabled:      true,
				DetachedTaskRuntimeEnabled: true,
			})
			resolved, err := resolver.Resolve(tt.input)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got, want := resolved.Policy.DetachedRunMode, tt.want; got != want {
				t.Fatalf("DetachedRunMode = %q, want %q", got, want)
			}
		})
	}
}

func TestFakeSessionManagerEventsReturnAscendingSequenceOrder(t *testing.T) {
	t.Run("ShouldSortBySequenceBeforeApplyingLimit", func(t *testing.T) {
		t.Parallel()

		sessions := &fakeSessionManager{
			sessionEvents: map[string][]store.SessionEvent{
				"sess-wake": {
					{ID: "evt-3", SessionID: "sess-wake", Sequence: 3, Type: acp.EventTypeSyntheticReentry},
					{ID: "evt-1", SessionID: "sess-wake", Sequence: 1, Type: acp.EventTypeSyntheticReentry},
					{ID: "evt-2", SessionID: "sess-wake", Sequence: 2, Type: acp.EventTypeSyntheticReentry},
				},
			},
		}

		events, err := sessions.Events(testutil.Context(t), "sess-wake", store.EventQuery{Limit: 2})
		if err != nil {
			t.Fatalf("Events() error = %v", err)
		}
		if got, want := len(events), 2; got != want {
			t.Fatalf("len(events) = %d, want %d", got, want)
		}
		if got, want := events[0].Sequence, int64(2); got != want {
			t.Fatalf("events[0].Sequence = %d, want %d", got, want)
		}
		if got, want := events[1].Sequence, int64(3); got != want {
			t.Fatalf("events[1].Sequence = %d, want %d", got, want)
		}
	})
}

func TestPromptInputCompositeEnforcesPerDescriptorBudgets(t *testing.T) {
	t.Run("ShouldCapEachDescriptorBeforeConsumingAggregateBudget", func(t *testing.T) {
		t.Parallel()

		resolver := &staticPromptInputAugmenterResolver{
			resolved: ResolvedHarnessContext{
				Policy: ResolvedHarnessPolicy{
					EnableAugmenters: []HarnessAugmenter{"prefix", "suffix"},
				},
			},
		}

		augmenter, err := newPromptInputCompositeAugmenter(
			discardLogger(),
			resolver,
			nil,
			promptInputAugmenterDescriptor{
				Name:   "prefix",
				Order:  100,
				Budget: 1,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					return "AAAA" + message, nil
				},
			},
			promptInputAugmenterDescriptor{
				Name:   "suffix",
				Order:  200,
				Budget: 3,
				Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
					return message + "BBBB", nil
				},
			},
		)
		if err != nil {
			t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
		}

		got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
		if err != nil {
			t.Fatalf("Augment() error = %v", err)
		}
		if got != "AbaseBBB" {
			t.Fatalf("Augment() = %q, want %q", got, "AbaseBBB")
		}
	})
}

func seedDetachedHarnessRecoveryRunForTest(
	t *testing.T,
	db harnessReentryStore,
	actor taskpkg.ActorContext,
	taskID string,
	runID string,
	summary string,
	completedAt time.Time,
) {
	t.Helper()

	taskMetadata, err := marshalDetachedHarnessMetadata(detachedHarnessTaskMetadata{
		Schema:         harnessDetachedMetadataSchema,
		Kind:           harnessDetachedTaskMetadataKey,
		SubmissionKey:  "submission-" + runID,
		Summary:        summary,
		OwnerSessionID: "sess-owner",
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   "sess-wake",
			SessionType: string(session.SessionTypeSystem),
		},
	})
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata(task) error = %v", err)
	}
	runMetadata, err := marshalDetachedHarnessMetadata(detachedHarnessRunMetadata{
		Schema:         harnessDetachedMetadataSchema,
		Kind:           harnessDetachedRunMetadataKey,
		SubmissionKey:  "submission-" + runID,
		Summary:        summary,
		OwnerSessionID: "sess-owner",
		WakeTarget: detachedHarnessWakeTarget{
			SessionID:   "sess-wake",
			SessionType: string(session.SessionTypeSystem),
		},
	})
	if err != nil {
		t.Fatalf("marshalDetachedHarnessMetadata(run) error = %v", err)
	}

	if err := db.CreateTask(testutil.Context(t), taskpkg.Task{
		ID:        taskID,
		Scope:     taskpkg.ScopeGlobal,
		Title:     summary,
		Status:    taskpkg.TaskStatusInProgress,
		CreatedBy: actor.Actor,
		Origin:    actor.Origin,
		CreatedAt: completedAt.Add(-time.Minute),
		UpdatedAt: completedAt.Add(-time.Minute),
		Metadata:  taskMetadata,
	}); err != nil {
		t.Fatalf("CreateTask(%q) error = %v", taskID, err)
	}
	if err := db.CreateTaskRun(testutil.Context(t), taskpkg.Run{
		ID:              runID,
		TaskID:          taskID,
		Status:          taskpkg.TaskRunStatusCompleted,
		Attempt:         1,
		Origin:          actor.Origin,
		IdempotencyKey:  "idem-" + runID,
		RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: daemonTestLiveParticipation("ws-1", "builders")},
		Metadata:        runMetadata,
		QueuedAt:        completedAt.Add(-2 * time.Minute),
		ClaimedAt:       completedAt.Add(-90 * time.Second),
		StartedAt:       completedAt.Add(-time.Minute),
		EndedAt:         completedAt,
		Result:          json.RawMessage(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", runID, err)
	}
	if err := db.CreateTaskEvent(testutil.Context(t), taskpkg.Event{
		ID:        "evt-" + runID,
		TaskID:    taskID,
		RunID:     runID,
		EventType: harnessTaskEventRunCompleted,
		Actor:     actor.Actor,
		Origin:    actor.Origin,
		Timestamp: completedAt,
	}); err != nil {
		t.Fatalf("CreateTaskEvent(%q) error = %v", runID, err)
	}
}

type fakeSessionManager struct {
	mu                sync.Mutex
	infos             []*session.Info
	sessionEvents     map[string][]store.SessionEvent
	nextEventSequence int64
	onStop            func(string)
	stopErr           func(string) error
	stopWithCauseErr  func(string, session.StopCause, string) error
	requestStopErr    func(string, session.StopCause, string) error
	createCalls       []session.CreateOpts
	promptCalls       []struct {
		id  string
		msg string
	}
	syntheticPromptCalls     []fakeSyntheticPromptCall
	syntheticPromptHook      func(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	promptHook               func(context.Context, string, string) (<-chan acp.AgentEvent, error)
	healthRows               map[string]heartbeat.SessionHealth
	promptStarted            chan struct{}
	promptRelease            <-chan struct{}
	promptCtxCancelled       chan struct{}
	stopCalls                []string
	deleteCalls              []string
	repairCalls              []session.RepairOpts
	stopWithCauseCalls       []fakeStopWithCauseCall
	requestStopCalls         []fakeStopWithCauseCall
	waitFinalizationsRelease <-chan struct{}
	waitFinalizationsCalls   int
	shutdownCalls            int
	shutdownErr              error
	compactionHandler        session.CompactionHandler
}

var _ SessionManager = (*fakeSessionManager)(nil)
var _ memoryExtractorSessionManager = (*fakeSessionManager)(nil)
var _ autoTitleSessionManager = (*fakeSessionManager)(nil)
var _ clarifyEventPublisher = (*fakeSessionManager)(nil)

func (f *fakeSessionManager) SetCompactionHandler(handler session.CompactionHandler) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.compactionHandler = handler
}

func (f *fakeSessionManager) PrepareWorkspaceRemoval(
	context.Context,
	string,
) (workspacepkg.UnregisterPreparation, error) {
	return fakeWorkspaceRemovalPreparation{}, nil
}

type fakeWorkspaceRemovalPreparation struct{}

func (fakeWorkspaceRemovalPreparation) BeforeDelete(context.Context) error { return nil }

func (fakeWorkspaceRemovalPreparation) Commit(context.Context) error { return nil }

func (fakeWorkspaceRemovalPreparation) Rollback(context.Context) error { return nil }

type blockingStatusSessionManager struct {
	*fakeSessionManager
	blockSessionID string
	statusStarted  chan struct{}
	statusOnce     sync.Once
}

type fakeSyntheticPromptCall struct {
	id   string
	opts session.SyntheticPromptOpts
}

type fakeStopWithCauseCall struct {
	id     string
	cause  session.StopCause
	detail string
}

type goleakSessionManager struct {
	*fakeSessionManager
	mu      sync.Mutex
	process *subprocess.Process
	wasStop bool
}

func (m *goleakSessionManager) setProcess(process *subprocess.Process) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.process = process
}

func (m *goleakSessionManager) stopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.wasStop
}

func (m *goleakSessionManager) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	m.mu.Lock()
	m.wasStop = true
	process := m.process
	m.mu.Unlock()
	if m.fakeSessionManager != nil {
		m.fakeSessionManager.mu.Lock()
		m.stopWithCauseCalls = append(
			m.stopWithCauseCalls,
			fakeStopWithCauseCall{id: id, cause: cause, detail: detail},
		)
		m.fakeSessionManager.mu.Unlock()
	}
	if process == nil {
		return nil
	}
	return process.Shutdown(ctx)
}

func (f *fakeSessionManager) Create(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls = append(f.createCalls, opts)
	sessionID := fmt.Sprintf("dream-%d", len(f.createCalls))
	workspaceID := strings.TrimSpace(opts.Workspace)
	workspace := strings.TrimSpace(opts.WorkspacePath)
	if workspace == "" {
		workspace = workspaceID
	}
	if workspaceID == "" {
		workspaceID = workspace
	}
	return &session.Session{
		ID:          sessionID,
		AgentName:   opts.AgentName,
		WorkspaceID: workspaceID,
		Workspace:   workspace,
		Type:        opts.Type,
		State:       session.StateActive,
	}, nil
}

func (f *fakeSessionManager) Spawn(ctx context.Context, opts session.SpawnOpts) (*session.Session, error) {
	child, err := f.Create(ctx, session.CreateOpts{
		AgentName:            opts.AgentName,
		Provider:             opts.Provider,
		Name:                 opts.Name,
		Workspace:            opts.Workspace,
		WorkspacePath:        opts.WorkspacePath,
		NetworkParticipation: opts.NetworkParticipation,
		PromptOverlay:        opts.PromptOverlay,
		Type:                 session.SessionTypeSpawned,
		Lineage: &store.SessionLineage{
			ParentSessionID:  opts.ParentSessionID,
			SpawnRole:        opts.SpawnRole,
			AutoStopOnParent: opts.AutoStopOnParent,
			PermissionPolicy: opts.PermissionPolicy,
		},
	})
	if err != nil {
		return nil, err
	}
	f.mu.Lock()
	f.infos = append(f.infos, child.Info())
	f.mu.Unlock()
	return child, nil
}

func (f *fakeSessionManager) ApplyAutomaticSessionTitle(
	_ context.Context,
	id string,
	title string,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, info := range f.infos {
		if info == nil || info.ID != id || strings.TrimSpace(info.Name) != "" {
			continue
		}
		info.Name = strings.TrimSpace(title)
		return true, nil
	}
	return false, nil
}

func (f *fakeSessionManager) List() []*session.Info {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*session.Info(nil), f.infos...)
}

func (f *fakeSessionManager) ListAll(context.Context) ([]*session.Info, error) {
	return f.List(), nil
}

func (f *fakeSessionManager) Status(_ context.Context, id string) (*session.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, info := range f.infos {
		if info != nil && info.ID == id {
			return info, nil
		}
	}
	return nil, session.ErrSessionNotFound
}

func (f *blockingStatusSessionManager) Status(ctx context.Context, id string) (*session.Info, error) {
	if strings.TrimSpace(id) == strings.TrimSpace(f.blockSessionID) {
		if f.statusStarted != nil {
			f.statusOnce.Do(func() {
				close(f.statusStarted)
			})
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.fakeSessionManager.Status(ctx, id)
}

func (f *fakeSessionManager) Events(
	_ context.Context,
	id string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	events := append([]store.SessionEvent(nil), f.sessionEvents[id]...)
	filtered := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		if query.AfterSequence > 0 && event.Sequence <= query.AfterSequence {
			continue
		}
		if query.Type != "" && event.Type != query.Type {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Sequence < filtered[j].Sequence
	})
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[len(filtered)-query.Limit:]
	}
	return filtered, nil
}

func (f *fakeSessionManager) LatestSessionEventByType(
	ctx context.Context,
	id string,
	eventType string,
) (*store.SessionEvent, error) {
	events, err := f.Events(ctx, id, store.EventQuery{Type: eventType, Limit: 1})
	if err != nil || len(events) == 0 {
		return nil, err
	}
	event := events[0]
	return &event, nil
}

func (f *fakeSessionManager) AppendSessionEventIfAbsent(
	_ context.Context,
	event session.GoalEvent,
) error {
	if err := event.Validate(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessionEvents == nil {
		f.sessionEvents = make(map[string][]store.SessionEvent)
	}
	for _, existing := range f.sessionEvents[event.SessionID] {
		if existing.ID != event.EventID {
			continue
		}
		if existing.TurnID == event.SyntheticTurnID &&
			existing.Type == event.Type &&
			existing.AgentName == event.AgentName &&
			existing.Content == string(event.Content) &&
			existing.Timestamp.Equal(event.CreatedAt) {
			return nil
		}
		return errors.New("fake session manager: event identity collision")
	}
	f.nextEventSequence++
	f.sessionEvents[event.SessionID] = append(f.sessionEvents[event.SessionID], store.SessionEvent{
		ID: event.EventID, SessionID: event.SessionID, Sequence: f.nextEventSequence,
		TurnID: event.SyntheticTurnID, Type: event.Type, AgentName: event.AgentName,
		Content: string(event.Content), Timestamp: event.CreatedAt,
	})
	return nil
}

func (f *fakeSessionManager) PublishClarifyEvent(
	_ context.Context,
	event toolspkg.ClarifyEvent,
) error {
	if err := event.Validate(); err != nil {
		return err
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("fake session manager: marshal clarification event: %w", err)
	}
	turnID := "clarify:" + event.Request.RequestID
	content, err := transcript.MarshalAgentEvent(acp.AgentEvent{
		Type:      acp.EventTypeClarify,
		SessionID: event.Request.SessionID,
		TurnID:    turnID,
		RequestID: event.Request.RequestID,
		Timestamp: event.At.UTC(),
		Raw:       payload,
	})
	if err != nil {
		return fmt.Errorf("fake session manager: encode clarification event: %w", err)
	}

	persisted := store.SessionEvent{
		ID:        event.Request.RequestID + "-" + string(event.Status),
		SessionID: event.Request.SessionID,
		TurnID:    turnID,
		Type:      acp.EventTypeClarify,
		AgentName: event.Request.AgentName,
		Content:   content,
		Timestamp: event.At.UTC(),
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessionEvents == nil {
		f.sessionEvents = make(map[string][]store.SessionEvent)
	}
	for _, existing := range f.sessionEvents[persisted.SessionID] {
		if existing.ID != persisted.ID {
			continue
		}
		if existing.TurnID == persisted.TurnID && existing.Type == persisted.Type &&
			existing.AgentName == persisted.AgentName && existing.Content == persisted.Content &&
			existing.Timestamp.Equal(persisted.Timestamp) {
			return nil
		}
		return errors.New("fake session manager: clarification event identity collision")
	}
	f.nextEventSequence++
	persisted.Sequence = f.nextEventSequence
	f.sessionEvents[persisted.SessionID] = append(f.sessionEvents[persisted.SessionID], persisted)
	return nil
}

func (f *fakeSessionManager) History(context.Context, string, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (f *fakeSessionManager) TranscriptPage(
	context.Context,
	string,
	transcript.PageQuery,
) (transcript.Page, error) {
	return transcript.Page{}, nil
}

func (f *fakeSessionManager) TranscriptChanges(
	context.Context,
	string,
	transcript.ChangeQuery,
) (transcript.ChangePage, error) {
	return transcript.ChangePage{}, nil
}

func (f *fakeSessionManager) InputQueueSummary(
	context.Context,
	string,
) (session.InputQueueSummary, error) {
	return session.InputQueueSummary{}, nil
}

func (f *fakeSessionManager) RepairSession(
	_ context.Context,
	opts session.RepairOpts,
) (*session.RepairResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.repairCalls = append(f.repairCalls, opts)
	return &session.RepairResult{SessionID: opts.SessionID}, nil
}

func (f *fakeSessionManager) Stop(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls = append(f.stopCalls, id)
	if f.onStop != nil && len(f.infos) > 0 {
		f.onStop(f.infos[0].ID)
		f.infos = f.infos[1:]
	}
	if f.stopErr != nil {
		return f.stopErr(id)
	}
	return nil
}

func (f *fakeSessionManager) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.deleteCalls = append(f.deleteCalls, id)

	removed := false
	filtered := f.infos[:0]
	for _, info := range f.infos {
		if info != nil && info.ID == id {
			removed = true
			continue
		}
		filtered = append(filtered, info)
	}
	f.infos = filtered
	if !removed {
		return session.ErrSessionNotFound
	}
	return nil
}

func (f *fakeSessionManager) StopWithCause(_ context.Context, id string, cause session.StopCause, detail string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopWithCauseCalls = append(f.stopWithCauseCalls, fakeStopWithCauseCall{
		id:     id,
		cause:  cause,
		detail: detail,
	})
	if f.onStop != nil && len(f.infos) > 0 {
		f.onStop(f.infos[0].ID)
		f.infos = f.infos[1:]
	}
	if f.stopWithCauseErr != nil {
		return f.stopWithCauseErr(id, cause, detail)
	}
	if f.stopErr != nil {
		return f.stopErr(id)
	}
	return nil
}

func (f *fakeSessionManager) RequestStopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requestStopCalls = append(f.requestStopCalls, fakeStopWithCauseCall{
		id:     id,
		cause:  cause,
		detail: detail,
	})
	if f.requestStopErr != nil {
		return f.requestStopErr(id, cause, detail)
	}
	return nil
}

func (f *fakeSessionManager) WaitForFinalizations(ctx context.Context) error {
	f.mu.Lock()
	f.waitFinalizationsCalls++
	release := f.waitFinalizationsRelease
	f.mu.Unlock()

	if release == nil {
		return nil
	}

	select {
	case <-release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeSessionManager) Shutdown(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdownCalls++
	return f.shutdownErr
}

func (f *fakeSessionManager) Resume(context.Context, string) (*session.Session, error) {
	return nil, nil
}

func (f *fakeSessionManager) ClearConversation(
	ctx context.Context,
	id string,
) (*session.Session, error) {
	info, err := f.Status(ctx, id)
	if err != nil && !errors.Is(err, session.ErrSessionNotFound) {
		return nil, err
	}
	if info == nil {
		return &session.Session{ID: id, State: session.StateActive}, nil
	}

	return &session.Session{
		ID:                   info.ID,
		Name:                 info.Name,
		AgentName:            info.AgentName,
		WorkspaceID:          info.WorkspaceID,
		Workspace:            info.Workspace,
		NetworkParticipation: info.NetworkParticipation,
		Type:                 info.Type,
		State:                session.StateActive,
		CreatedAt:            info.CreatedAt,
		UpdatedAt:            info.UpdatedAt,
	}, nil
}

func TestFakeSessionManagerClearConversationTreatsMissingSessionAsFreshConversation(t *testing.T) {
	t.Parallel()

	t.Run("ShouldTreatAMissingSessionAsAFreshConversation", func(t *testing.T) {
		manager := &fakeSessionManager{}
		cleared, err := manager.ClearConversation(context.Background(), "sess-missing")
		if err != nil {
			t.Fatalf("ClearConversation(missing) error = %v", err)
		}
		if cleared == nil {
			t.Fatal("ClearConversation(missing) = nil, want session")
		}
		if got, want := cleared.ID, "sess-missing"; got != want {
			t.Fatalf("cleared.ID = %q, want %q", got, want)
		}
		if got, want := cleared.State, session.StateActive; got != want {
			t.Fatalf("cleared.State = %q, want %q", got, want)
		}
	})
}

func TestBootSessionRepair(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRepairOnlyStoppedCrashOrErrorSessions", func(t *testing.T) {
		t.Parallel()

		manager := &fakeSessionManager{
			infos: []*session.Info{
				{ID: "sess-crash", State: session.StateStopped, StopReason: store.StopAgentCrashed},
				{ID: "sess-error", State: session.StateStopped, StopReason: store.StopError},
				{ID: "sess-complete", State: session.StateStopped, StopReason: store.StopCompleted},
				{ID: "sess-active", State: session.StateActive, StopReason: store.StopAgentCrashed},
			},
		}
		state := &bootState{
			logger:   discardLogger(),
			sessions: manager,
		}
		daemon := &Daemon{}

		if err := daemon.bootSessionRepair(testutil.Context(t), state); err != nil {
			t.Fatalf("bootSessionRepair() error = %v", err)
		}

		manager.mu.Lock()
		defer manager.mu.Unlock()
		if got, want := len(manager.repairCalls), 2; got != want {
			t.Fatalf("repair calls = %d, want %d", got, want)
		}
		if manager.repairCalls[0].SessionID != "sess-crash" || manager.repairCalls[1].SessionID != "sess-error" {
			t.Fatalf("repair calls = %#v, want crash then error sessions", manager.repairCalls)
		}
	})
}

func (f *fakeSessionManager) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	f.mu.Lock()
	f.promptCalls = append(f.promptCalls, struct {
		id  string
		msg string
	}{id: id, msg: msg})
	promptHook := f.promptHook
	promptStarted := f.promptStarted
	promptRelease := f.promptRelease
	promptCtxCancelled := f.promptCtxCancelled
	f.mu.Unlock()

	if promptHook != nil {
		return promptHook(ctx, id, msg)
	}

	if promptStarted != nil {
		select {
		case promptStarted <- struct{}{}:
		default:
		}
	}

	if promptRelease != nil || promptCtxCancelled != nil {
		ch := make(chan acp.AgentEvent)
		go func() {
			defer close(ch)
			if promptRelease == nil {
				if ctx != nil {
					<-ctx.Done()
				}
			} else {
				select {
				case <-promptRelease:
					return
				case <-ctx.Done():
				}
			}
			if promptCtxCancelled != nil {
				select {
				case promptCtxCancelled <- struct{}{}:
				default:
				}
			}
		}()
		return ch, nil
	}

	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (f *fakeSessionManager) PromptWithOpts(
	ctx context.Context,
	id string,
	opts session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	return f.Prompt(ctx, id, opts.Message)
}

func (f *fakeSessionManager) SendPrompt(
	ctx context.Context,
	id string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	events, err := f.Prompt(ctx, id, opts.Message)
	if err != nil {
		return session.SendPromptResult{}, err
	}
	return session.SendPromptResult{
		Status: "accepted",
		Mode:   opts.Mode,
		Events: events,
	}, nil
}

func (f *fakeSessionManager) InterruptPrompt(context.Context, string) (session.SendPromptResult, error) {
	return session.SendPromptResult{
		Status:      "interrupted",
		Mode:        session.BusyInputModeInterrupt,
		Interrupted: true,
	}, nil
}

func (f *fakeSessionManager) SteerPrompt(
	_ context.Context,
	_ string,
	_ string,
) (session.SendPromptResult, error) {
	return session.SendPromptResult{
		Status: "staged",
		Mode:   session.BusyInputModeSteer,
		Staged: true,
	}, nil
}

func (f *fakeSessionManager) CancelQueuedPrompt(
	_ context.Context,
	_ string,
	queueEntryID string,
) (session.SendPromptResult, error) {
	return session.SendPromptResult{
		Status:       "canceled",
		Mode:         session.BusyInputModeQueue,
		QueueEntryID: queueEntryID,
	}, nil
}

func (f *fakeSessionManager) CancelPrompt(context.Context, string) error {
	return nil
}

func (f *fakeSessionManager) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	info, err := f.Status(ctx, id)
	if err != nil {
		return nil, err
	}
	if info == nil || info.State != session.StateActive {
		return nil, session.ErrSessionNotActive
	}

	f.mu.Lock()
	f.syntheticPromptCalls = append(f.syntheticPromptCalls, fakeSyntheticPromptCall{id: id, opts: opts})
	hook := f.syntheticPromptHook
	f.mu.Unlock()

	if hook != nil {
		return hook(ctx, id, opts)
	}

	f.recordSyntheticEvent(id, info, opts)
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (f *fakeSessionManager) ApprovePermission(context.Context, string, acp.ApproveRequest) error {
	return nil
}

func (f *fakeSessionManager) SetNetworkPeerLifecycle(session.NetworkPeerLifecycle) {}

func (f *fakeSessionManager) SetTurnEndNotifier(session.TurnEndNotifier) {}

func (f *fakeSessionManager) PromptNetwork(
	context.Context,
	string,
	string,
	...acp.PromptNetworkMeta,
) (<-chan acp.AgentEvent, error) {
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (f *fakeSessionManager) IsPrompting(string) bool {
	return false
}

func (f *fakeSessionManager) recordSyntheticEvent(
	sessionID string,
	info *session.Info,
	opts session.SyntheticPromptOpts,
) {
	if info == nil {
		return
	}

	timestamp := time.Now().UTC()

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sessionEvents == nil {
		f.sessionEvents = make(map[string][]store.SessionEvent)
	}
	f.nextEventSequence++
	sequence := f.nextEventSequence
	turnID := fmt.Sprintf("turn-synthetic-%d", sequence)

	payload, err := json.Marshal(acp.AgentEvent{
		Type:      acp.EventTypeSyntheticReentry,
		TurnID:    turnID,
		Timestamp: timestamp,
		Text:      strings.TrimSpace(opts.Message),
		Synthetic: &opts.Metadata,
	})
	if err != nil {
		return
	}

	f.sessionEvents[sessionID] = append(f.sessionEvents[sessionID], store.SessionEvent{
		ID:        fmt.Sprintf("evt-%d", sequence),
		SessionID: sessionID,
		Sequence:  sequence,
		TurnID:    turnID,
		Type:      acp.EventTypeSyntheticReentry,
		AgentName: info.AgentName,
		Content:   string(payload),
		Timestamp: timestamp,
	})
}

func (f *fakeSessionManager) syntheticPromptCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.syntheticPromptCalls)
}

func (f *fakeSessionManager) createCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.createCalls)
}

func (f *fakeSessionManager) createCall(index int) session.CreateOpts {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.createCalls[index]
}

type fakeNetworkBindableSessionManager struct {
	*fakeSessionManager
	networkPeers        session.NetworkPeerLifecycle
	turnEndNotifier     session.TurnEndNotifier
	promptNetworkFn     func(context.Context, string, string) (<-chan acp.AgentEvent, error)
	promptWithOptsCalls int
	prompting           map[string]bool
}

func newFakeNetworkBindableSessionManager() *fakeNetworkBindableSessionManager {
	return &fakeNetworkBindableSessionManager{
		fakeSessionManager: &fakeSessionManager{},
		prompting:          make(map[string]bool),
	}
}

func (f *fakeNetworkBindableSessionManager) PromptWithOpts(
	_ context.Context,
	_ string,
	_ session.PromptOpts,
) (<-chan acp.AgentEvent, error) {
	f.mu.Lock()
	f.promptWithOptsCalls++
	f.mu.Unlock()
	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (f *fakeNetworkBindableSessionManager) SetNetworkPeerLifecycle(lifecycle session.NetworkPeerLifecycle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.networkPeers = lifecycle
}

func (f *fakeNetworkBindableSessionManager) currentNetworkPeerLifecycle() session.NetworkPeerLifecycle {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.networkPeers
}

func (f *fakeNetworkBindableSessionManager) SetTurnEndNotifier(fn session.TurnEndNotifier) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.turnEndNotifier = fn
}

func (f *fakeNetworkBindableSessionManager) currentTurnEndNotifier() session.TurnEndNotifier {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.turnEndNotifier
}

func (f *fakeNetworkBindableSessionManager) PromptNetwork(
	ctx context.Context,
	id string,
	msg string,
	_ ...acp.PromptNetworkMeta,
) (<-chan acp.AgentEvent, error) {
	f.mu.Lock()
	fn := f.promptNetworkFn
	f.mu.Unlock()
	if fn != nil {
		return fn(ctx, id, msg)
	}

	ch := make(chan acp.AgentEvent)
	close(ch)
	return ch, nil
}

func (f *fakeNetworkBindableSessionManager) IsPrompting(sessionID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.prompting[sessionID]
}

type syntheticPrompter interface {
	PromptSynthetic(context.Context, string, session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
}

type spawnSurface interface {
	Spawn(context.Context, session.SpawnOpts) (*session.Session, error)
}

type autoTitleApplySurface interface {
	ApplyAutomaticSessionTitle(context.Context, string, string) (bool, error)
}

type nonBindableHarnessSessionManager struct {
	SessionManager
	syntheticPrompter syntheticPrompter
}

func (m nonBindableHarnessSessionManager) PublishClarifyEvent(
	ctx context.Context,
	event toolspkg.ClarifyEvent,
) error {
	publisher, ok := m.SessionManager.(clarifyEventPublisher)
	if !ok {
		return errors.New("non-bindable session manager: clarification publisher is required")
	}
	return publisher.PublishClarifyEvent(ctx, event)
}

type sessionManagerWithoutWorkspaceRemoval struct {
	SessionManager
}

func (m sessionManagerWithoutWorkspaceRemoval) PublishClarifyEvent(
	ctx context.Context,
	event toolspkg.ClarifyEvent,
) error {
	publisher, ok := m.SessionManager.(clarifyEventPublisher)
	if !ok {
		return errors.New("session manager without workspace removal: clarification publisher is required")
	}
	return publisher.PublishClarifyEvent(ctx, event)
}

var (
	_ SessionManager                = (*fakeSessionManager)(nil)
	_ SessionManager                = (*fakeNetworkBindableSessionManager)(nil)
	_ SessionManager                = nonBindableHarnessSessionManager{}
	_ networkBindableSessionManager = (*fakeNetworkBindableSessionManager)(nil)
	_ syntheticPrompter             = (*fakeSessionManager)(nil)
	_ syntheticPrompter             = nonBindableHarnessSessionManager{}
	_ spawnSurface                  = nonBindableHarnessSessionManager{}
	_ autoTitleApplySurface         = nonBindableHarnessSessionManager{}
)

func (m nonBindableHarnessSessionManager) Spawn(
	ctx context.Context,
	opts session.SpawnOpts,
) (*session.Session, error) {
	spawner, ok := m.SessionManager.(spawnSurface)
	if !ok {
		return nil, errors.New("daemon test: session manager spawn surface is required")
	}
	return spawner.Spawn(ctx, opts)
}

func (m nonBindableHarnessSessionManager) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	return m.syntheticPrompter.PromptSynthetic(ctx, id, opts)
}

func (m nonBindableHarnessSessionManager) ApplyAutomaticSessionTitle(
	ctx context.Context,
	id string,
	title string,
) (bool, error) {
	applier, ok := m.SessionManager.(autoTitleApplySurface)
	if !ok {
		return false, errors.New("daemon test: session manager automatic title surface is required")
	}
	return applier.ApplyAutomaticSessionTitle(ctx, id, title)
}

func (m nonBindableHarnessSessionManager) PrepareWorkspaceRemoval(
	ctx context.Context,
	workspaceID string,
) (workspacepkg.UnregisterPreparation, error) {
	preparer, ok := m.SessionManager.(workspaceRemovalPreparer)
	if !ok {
		return nil, errMissingWorkspaceRemovalPreparation
	}
	return preparer.PrepareWorkspaceRemoval(ctx, workspaceID)
}

type fakeNetworkRuntime struct {
	mu               sync.Mutex
	status           *network.Status
	statusErr        error
	sendID           string
	sendErr          error
	sendCalls        []network.SendRequest
	runtimeSendCalls []network.RuntimeSendRequest
	joinCalls        []fakeNetworkJoinCall
	leaveCalls       []string
	turnEnds         []string
	inboxes          map[string][]network.Envelope
	shutdownErr      error
	onShutdown       func()
}

type fakeNetworkJoinCall struct {
	sessionID    string
	peerID       string
	channel      string
	capabilities []session.NetworkPeerCapability
}

func cloneFakeNetworkPeerCapabilities(capabilities []session.NetworkPeerCapability) []session.NetworkPeerCapability {
	if capabilities == nil {
		return nil
	}

	cloned := make([]session.NetworkPeerCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		cloned = append(cloned, session.NetworkPeerCapability{
			ID:                capability.ID,
			Summary:           capability.Summary,
			Outcome:           capability.Outcome,
			ContextNeeded:     append([]string(nil), capability.ContextNeeded...),
			ArtifactsExpected: append([]string(nil), capability.ArtifactsExpected...),
			ExecutionOutline:  append([]string(nil), capability.ExecutionOutline...),
			Constraints:       append([]string(nil), capability.Constraints...),
			Examples:          append([]string(nil), capability.Examples...),
		})
	}

	return cloned
}

func (f *fakeNetworkRuntime) Send(_ context.Context, req network.SendRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendCalls = append(f.sendCalls, req)
	if f.sendErr != nil {
		return "", f.sendErr
	}
	if strings.TrimSpace(f.sendID) != "" {
		return f.sendID, nil
	}
	return "msg-test", nil
}

func (f *fakeNetworkRuntime) SendFromRuntimePeer(
	_ context.Context,
	req network.RuntimeSendRequest,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runtimeSendCalls = append(f.runtimeSendCalls, req)
	if f.sendErr != nil {
		return "", f.sendErr
	}
	if strings.TrimSpace(f.sendID) != "" {
		return f.sendID, nil
	}
	return "msg-runtime-test", nil
}

func (f *fakeNetworkRuntime) ListPeers(context.Context, string, string) ([]network.PeerInfo, error) {
	return nil, nil
}

func (f *fakeNetworkRuntime) ListChannels(context.Context, string) ([]network.ChannelInfo, error) {
	return nil, nil
}

func (f *fakeNetworkRuntime) Status(context.Context) (*network.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.statusErr != nil {
		return nil, f.statusErr
	}
	if f.status == nil {
		return nil, nil
	}
	status := *f.status
	return &status, nil
}

func (f *fakeNetworkRuntime) Inbox(_ context.Context, sessionID string) ([]network.Envelope, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.inboxes) == 0 {
		return nil, nil
	}
	return append([]network.Envelope(nil), f.inboxes[sessionID]...), nil
}

func (f *fakeNetworkRuntime) WaitInbox(ctx context.Context, sessionID string, _ string) ([]network.Envelope, error) {
	return f.Inbox(ctx, sessionID)
}

func (f *fakeNetworkRuntime) JoinChannel(_ context.Context, join session.NetworkPeerJoin) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.joinCalls = append(f.joinCalls, fakeNetworkJoinCall{
		sessionID:    join.SessionID,
		peerID:       join.PeerID,
		channel:      join.Channel,
		capabilities: cloneFakeNetworkPeerCapabilities(join.Capabilities),
	})
	return nil
}

func (f *fakeNetworkRuntime) LeaveChannel(_ context.Context, sessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.leaveCalls = append(f.leaveCalls, sessionID)
	return nil
}

func (f *fakeNetworkRuntime) OnTurnEnd(sessionID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.turnEnds = append(f.turnEnds, sessionID)
}

func (f *fakeNetworkRuntime) Shutdown(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.onShutdown != nil {
		f.onShutdown()
	}
	return f.shutdownErr
}

func TestFakeNetworkRuntimeJoinChannelDeepClonesCapabilities(t *testing.T) {
	t.Parallel()

	runtime := &fakeNetworkRuntime{}
	join := session.NetworkPeerJoin{
		SessionID: "sess-1",
		PeerID:    "peer-1",
		Channel:   "channel-1",
		Capabilities: []session.NetworkPeerCapability{{
			ID:                "review-pr",
			Summary:           "Review pull requests",
			Outcome:           "Review feedback",
			ContextNeeded:     []string{"repo", "diff"},
			ArtifactsExpected: []string{"comments"},
			ExecutionOutline:  []string{"inspect", "comment"},
			Constraints:       []string{"stay scoped"},
			Examples:          []string{"review PR #49"},
		}},
	}

	if err := runtime.JoinChannel(testutil.Context(t), join); err != nil {
		t.Fatalf("JoinChannel() error = %v", err)
	}

	join.Capabilities[0].ContextNeeded[0] = "mutated"
	join.Capabilities[0].ArtifactsExpected[0] = "mutated"
	join.Capabilities[0].ExecutionOutline[0] = "mutated"
	join.Capabilities[0].Constraints[0] = "mutated"
	join.Capabilities[0].Examples[0] = "mutated"

	runtime.mu.Lock()
	recorded := runtime.joinCalls[0]
	runtime.mu.Unlock()

	if got, want := recorded.capabilities[0].ContextNeeded, []string{"repo", "diff"}; !slices.Equal(got, want) {
		t.Fatalf("recorded ContextNeeded = %#v, want %#v", got, want)
	}
	if got, want := recorded.capabilities[0].ArtifactsExpected, []string{"comments"}; !slices.Equal(got, want) {
		t.Fatalf("recorded ArtifactsExpected = %#v, want %#v", got, want)
	}
	if got, want := recorded.capabilities[0].ExecutionOutline, []string{
		"inspect",
		"comment",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("recorded ExecutionOutline = %#v, want %#v", got, want)
	}
	if got, want := recorded.capabilities[0].Constraints, []string{"stay scoped"}; !slices.Equal(got, want) {
		t.Fatalf("recorded Constraints = %#v, want %#v", got, want)
	}
	if got, want := recorded.capabilities[0].Examples, []string{"review PR #49"}; !slices.Equal(got, want) {
		t.Fatalf("recorded Examples = %#v, want %#v", got, want)
	}
}

type fakeObserver struct {
	reconciled  bool
	result      store.ReconcileResult
	err         error
	onReconcile func()
}

func (f *fakeObserver) QueryEvents(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
	return nil, nil
}

func (f *fakeObserver) QueryHookCatalog(context.Context, hookspkg.CatalogFilter) ([]hookspkg.CatalogEntry, error) {
	return nil, nil
}

func (f *fakeObserver) QueryHookRuns(context.Context, store.HookRunQuery) ([]hookspkg.HookRunRecord, error) {
	return nil, nil
}

func (f *fakeObserver) QueryHookEvents(context.Context, hookspkg.EventFilter) ([]hookspkg.EventDescriptor, error) {
	return nil, nil
}

func (f *fakeObserver) QueryTokenStats(context.Context, store.TokenStatsQuery) ([]store.TokenStats, error) {
	return nil, nil
}

func (f *fakeObserver) QueryBridgeHealth(context.Context) ([]observe.BridgeInstanceHealth, error) {
	return nil, nil
}

func (f *fakeObserver) QueryTaskDashboard(
	context.Context,
	observe.TaskDashboardQuery,
) (observe.TaskDashboardView, error) {
	return observe.TaskDashboardView{}, nil
}

func (f *fakeObserver) QueryTaskInbox(
	context.Context,
	observe.TaskInboxQuery,
	taskpkg.ActorIdentity,
) (observe.TaskInboxView, error) {
	return observe.TaskInboxView{}, nil
}

func (f *fakeObserver) QueryObserveOverview(
	context.Context,
	observe.OverviewQuery,
) (observe.OverviewView, error) {
	return observe.OverviewView{}, nil
}

func (f *fakeObserver) Health(context.Context) (observe.Health, error) {
	return observe.Health{Status: "ok"}, nil
}

func (f *fakeObserver) Reconcile(context.Context) (store.ReconcileResult, error) {
	if f.onReconcile != nil {
		f.onReconcile()
	}
	f.reconciled = true
	return f.result, f.err
}

func (f *fakeObserver) OnSessionCreated(context.Context, *session.Session) {}

func (f *fakeObserver) OnSessionStopped(context.Context, *session.Session) {}

func (f *fakeObserver) OnAgentEvent(context.Context, string, any) {}

type failingRetentionObserver struct {
	fakeObserver
	startErr       error
	shutdownCalled bool
}

func (f *failingRetentionObserver) StartRetention(context.Context) error {
	return f.startErr
}

func (f *failingRetentionObserver) ShutdownRetention(context.Context) error {
	f.shutdownCalled = true
	return nil
}

type hookAwareTestObserver struct {
	fakeObserver
	attached observe.HookCatalogSource
}

func (o *hookAwareTestObserver) AttachHooks(source observe.HookCatalogSource) {
	o.attached = source
}

type fakeServer struct {
	name       string
	onShutdown func()
}

func (f *fakeServer) Start(context.Context) error {
	return nil
}

func (f *fakeServer) Shutdown(context.Context) error {
	if f.onShutdown != nil {
		f.onShutdown()
	}
	return nil
}

type fakeResourceReconcileDriver struct {
	runBootCalls int
	closeCalls   int
	triggerCalls int
	lastKind     resources.ResourceKind
	lastReason   resources.ReconcileReason
	triggerErr   error
	onRunBoot    func()
	onClose      func()
}

func (f *fakeResourceReconcileDriver) Trigger(
	_ context.Context,
	kind resources.ResourceKind,
	reason resources.ReconcileReason,
) error {
	f.triggerCalls++
	f.lastKind = kind
	f.lastReason = reason
	return f.triggerErr
}

func (f *fakeResourceReconcileDriver) RunBoot(context.Context) error {
	f.runBootCalls++
	if f.onRunBoot != nil {
		f.onRunBoot()
	}
	return nil
}

func (f *fakeResourceReconcileDriver) Close(context.Context) error {
	f.closeCalls++
	if f.onClose != nil {
		f.onClose()
	}
	return nil
}

type recordingRegistry struct {
	path                      string
	onClose                   func()
	onListTaskRunsByStatus    func([]taskpkg.RunStatus)
	mu                        sync.Mutex
	workspaces                map[string]workspacepkg.Workspace
	deadEntities              map[store.DeadEntityKey]store.DeadEntity
	networkAvailability       store.NetworkAvailability
	networkAvailabilityWrites []bool
	coordinationSettings      map[string]workspacepkg.CoordinationSetting
	approvalGrants            map[toolspkg.ApprovalGrantKey]toolspkg.ApprovalGrant
}

func (r *recordingRegistry) LookupApprovalGrant(
	_ context.Context,
	key toolspkg.ApprovalGrantKey,
) (toolspkg.ApprovalGrant, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	grant, ok := r.approvalGrants[key.Normalize()]
	return grant, ok, nil
}

func (r *recordingRegistry) PutApprovalGrant(
	_ context.Context,
	grant toolspkg.ApprovalGrant,
) (toolspkg.ApprovalGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.approvalGrants == nil {
		r.approvalGrants = make(map[toolspkg.ApprovalGrantKey]toolspkg.ApprovalGrant)
	}
	grant = grant.Normalize()
	if grant.ID == "" {
		grant.ID = fmt.Sprintf("grant-%d", len(r.approvalGrants)+1)
	}
	now := time.Now().UTC()
	if grant.CreatedAt.IsZero() {
		grant.CreatedAt = now
	}
	if grant.LastUsedAt.IsZero() {
		grant.LastUsedAt = now
	}
	r.approvalGrants[grant.ApprovalGrantKey] = grant
	return grant, nil
}

func (r *recordingRegistry) ListApprovalGrants(
	_ context.Context,
	workspaceID string,
) ([]toolspkg.ApprovalGrant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	grants := make([]toolspkg.ApprovalGrant, 0)
	for key, grant := range r.approvalGrants {
		if key.WorkspaceID == workspaceID {
			grants = append(grants, grant)
		}
	}
	return grants, nil
}

func (r *recordingRegistry) RevokeApprovalGrant(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, grant := range r.approvalGrants {
		if key.WorkspaceID == workspaceID && grant.ID == id {
			delete(r.approvalGrants, key)
			return nil
		}
	}
	return toolspkg.ErrApprovalGrantNotFound
}

func (r *recordingRegistry) MarkDeadEntity(_ context.Context, entity store.DeadEntity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deadEntities == nil {
		r.deadEntities = make(map[store.DeadEntityKey]store.DeadEntity)
	}
	r.deadEntities[entity.DeadEntityKey] = entity
	return nil
}

func (r *recordingRegistry) ClearDeadEntity(
	_ context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.deadEntities, store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID})
	return nil
}

func (r *recordingRegistry) FindDeadEntity(
	_ context.Context,
	workspaceID string,
	kind store.DeadEntityKind,
	entityID string,
) (store.DeadEntity, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entity, ok := r.deadEntities[store.DeadEntityKey{WorkspaceID: workspaceID, Kind: kind, EntityID: entityID}]
	return entity, ok, nil
}

func (r *recordingRegistry) ListDeadEntities(
	_ context.Context,
	workspaceID string,
) ([]store.DeadEntity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entities := make([]store.DeadEntity, 0)
	for key, entity := range r.deadEntities {
		if key.WorkspaceID == workspaceID {
			entities = append(entities, entity)
		}
	}
	return entities, nil
}

var (
	_ Registry  = (*recordingRegistry)(nil)
	_ taskStore = (*recordingRegistry)(nil)
)

func (r *recordingRegistry) Path() string {
	return r.path
}

func (r *recordingRegistry) InsertWorkspace(_ context.Context, ws workspacepkg.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	for _, existing := range r.workspaces {
		if existing.RootDir == ws.RootDir {
			return workspacepkg.ErrWorkspacePathTaken
		}
		if existing.Name == ws.Name {
			return workspacepkg.ErrWorkspaceNameTaken
		}
	}
	r.workspaces[ws.ID] = cloneRecordingWorkspace(ws)
	return nil
}

func (r *recordingRegistry) UpdateWorkspace(_ context.Context, ws workspacepkg.Workspace) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	if _, ok := r.workspaces[ws.ID]; !ok {
		return workspacepkg.ErrWorkspaceNotFound
	}
	for id, existing := range r.workspaces {
		if id == ws.ID {
			continue
		}
		if existing.RootDir == ws.RootDir {
			return workspacepkg.ErrWorkspacePathTaken
		}
		if existing.Name == ws.Name {
			return workspacepkg.ErrWorkspaceNameTaken
		}
	}
	r.workspaces[ws.ID] = cloneRecordingWorkspace(ws)
	return nil
}

func (r *recordingRegistry) DeleteWorkspace(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	if _, ok := r.workspaces[id]; !ok {
		return workspacepkg.ErrWorkspaceNotFound
	}
	delete(r.workspaces, id)
	return nil
}

func (r *recordingRegistry) GetWorkspace(_ context.Context, id string) (workspacepkg.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	ws, ok := r.workspaces[id]
	if ok {
		return cloneRecordingWorkspace(ws), nil
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *recordingRegistry) GetWorkspaceByPath(
	_ context.Context,
	rootDir string,
) (workspacepkg.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	for _, ws := range r.workspaces {
		if ws.RootDir == rootDir {
			return cloneRecordingWorkspace(ws), nil
		}
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *recordingRegistry) GetWorkspaceByName(
	_ context.Context,
	name string,
) (workspacepkg.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	for _, ws := range r.workspaces {
		if ws.Name == name {
			return cloneRecordingWorkspace(ws), nil
		}
	}
	return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *recordingRegistry) ListWorkspaces(context.Context) ([]workspacepkg.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureWorkspaceMapLocked()
	workspaces := make([]workspacepkg.Workspace, 0, len(r.workspaces))
	for _, ws := range r.workspaces {
		workspaces = append(workspaces, cloneRecordingWorkspace(ws))
	}
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].Name == workspaces[j].Name {
			return workspaces[i].ID < workspaces[j].ID
		}
		return workspaces[i].Name < workspaces[j].Name
	})
	return workspaces, nil
}

func (r *recordingRegistry) ensureWorkspaceMapLocked() {
	if r.workspaces == nil {
		r.workspaces = make(map[string]workspacepkg.Workspace)
	}
}

func cloneRecordingWorkspace(ws workspacepkg.Workspace) workspacepkg.Workspace {
	ws.AdditionalDirs = append([]string(nil), ws.AdditionalDirs...)
	return ws
}

func (r *recordingRegistry) RegisterSession(context.Context, store.SessionInfo) error {
	return nil
}

func (r *recordingRegistry) UpdateSessionState(context.Context, store.SessionStateUpdate) error {
	return nil
}

func (r *recordingRegistry) DeleteSession(context.Context, string) error {
	return nil
}

func (r *recordingRegistry) ListSessions(context.Context, store.SessionListQuery) ([]store.SessionInfo, error) {
	return nil, nil
}

func (r *recordingRegistry) AttachSession(context.Context, store.SessionAttachRequest) (store.SessionAttach, error) {
	return store.SessionAttach{}, store.ErrSessionNotFound
}

func (r *recordingRegistry) ReconcileSessions(context.Context, []store.SessionInfo) (store.ReconcileResult, error) {
	return store.ReconcileResult{}, nil
}

func (r *recordingRegistry) WriteEventSummary(context.Context, store.EventSummary) error {
	return nil
}

func (r *recordingRegistry) ListEventSummaries(context.Context, store.EventSummaryQuery) ([]store.EventSummary, error) {
	return nil, nil
}

func (r *recordingRegistry) RecordTokenUsage(
	context.Context,
	store.TokenStatsUpdate,
	store.TokenUsageDailyUpdate,
) error {
	return nil
}

func (r *recordingRegistry) ListTokenStats(context.Context, store.TokenStatsQuery) ([]store.TokenStats, error) {
	return nil, nil
}

func (r *recordingRegistry) WritePermissionLog(context.Context, store.PermissionLogEntry) error {
	return nil
}

func (r *recordingRegistry) ListPermissionLog(
	context.Context,
	store.PermissionLogQuery,
) ([]store.PermissionLogEntry, error) {
	return nil, nil
}

func (r *recordingRegistry) WriteNetworkAudit(context.Context, store.NetworkAuditEntry) error {
	return nil
}

func (r *recordingRegistry) ListNetworkAudit(
	context.Context,
	store.NetworkAuditQuery,
) ([]store.NetworkAuditEntry, error) {
	return nil, nil
}

func (r *recordingRegistry) GetNetworkAvailability(context.Context) (store.NetworkAvailability, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.networkAvailability, nil
}

func (r *recordingRegistry) SetNetworkAvailability(
	_ context.Context,
	enabled bool,
	updatedBy string,
) (store.NetworkAvailability, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.networkAvailability.Enabled != enabled {
		r.networkAvailability.Epoch++
	}
	r.networkAvailability.Enabled = enabled
	r.networkAvailability.UpdatedAt = time.Now().UTC()
	r.networkAvailability.UpdatedBy = strings.TrimSpace(updatedBy)
	r.networkAvailabilityWrites = append(r.networkAvailabilityWrites, enabled)
	return r.networkAvailability, nil
}

func (*recordingRegistry) AcceptNetworkMessage(
	context.Context,
	store.AcceptNetworkMessageRequest,
) (store.AcceptNetworkMessageResult, error) {
	return store.AcceptNetworkMessageResult{}, nil
}

func (*recordingRegistry) SettleNetworkWake(
	context.Context,
	taskpkg.NetworkWakeSettlement,
) (taskpkg.NetworkWakeSettlementResult, error) {
	return taskpkg.NetworkWakeSettlementResult{}, nil
}

func (*recordingRegistry) ListQueuedNetworkWakes(
	context.Context,
	int,
) ([]store.CommittedNetworkNotification, error) {
	return nil, nil
}

func (*recordingRegistry) LoadNetworkWake(
	context.Context,
	string,
	string,
) (store.WakeReservation, []store.NetworkMessageEntry, error) {
	return store.WakeReservation{}, nil, nil
}

func (r *recordingRegistry) Get(
	_ context.Context,
	workspaceID string,
) (workspacepkg.CoordinationSetting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := strings.TrimSpace(workspaceID)
	return r.coordinationSettings[id], nil
}

func (r *recordingRegistry) Set(
	_ context.Context,
	workspaceID string,
	enabled bool,
	actor string,
) (workspacepkg.CoordinationSetting, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.coordinationSettings == nil {
		r.coordinationSettings = make(map[string]workspacepkg.CoordinationSetting)
	}
	id := strings.TrimSpace(workspaceID)
	setting := r.coordinationSettings[id]
	setting.WorkspaceID = id
	setting.Enabled = enabled
	setting.Revision++
	setting.UpdatedAt = time.Now().UTC()
	setting.UpdatedBy = strings.TrimSpace(actor)
	r.coordinationSettings[id] = setting
	return setting, nil
}

func (*recordingRegistry) GetInvitation(
	_ context.Context,
	workspaceID string,
	scopeKind string,
	scopeID string,
) (workspacepkg.CoordinationInvitation, error) {
	return workspacepkg.CoordinationInvitation{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ScopeKind:   strings.TrimSpace(scopeKind),
		ScopeID:     strings.TrimSpace(scopeID),
	}, nil
}

func (*recordingRegistry) DismissInvitation(
	_ context.Context,
	workspaceID string,
	scopeKind string,
	scopeID string,
	actor string,
) (workspacepkg.CoordinationInvitation, error) {
	now := time.Now().UTC()
	return workspacepkg.CoordinationInvitation{
		WorkspaceID: strings.TrimSpace(workspaceID),
		ScopeKind:   strings.TrimSpace(scopeKind),
		ScopeID:     strings.TrimSpace(scopeID),
		Dismissed:   true,
		DismissedAt: now,
		DismissedBy: strings.TrimSpace(actor),
	}, nil
}

func (*recordingRegistry) ResetInvitation(context.Context, string, string, string) error {
	return nil
}

func (*recordingRegistry) GetCoordination(
	context.Context,
	workspacepkg.CoordinationRef,
) (workspacepkg.CoordinationView, error) {
	return workspacepkg.CoordinationView{}, nil
}

func (*recordingRegistry) SetCoordination(
	context.Context,
	workspacepkg.SetCoordination,
	taskpkg.ActorContext,
) (workspacepkg.CoordinationView, error) {
	return workspacepkg.CoordinationView{}, nil
}

func (*recordingRegistry) SetCoordinationInvitation(
	context.Context,
	workspacepkg.SetInvitation,
	taskpkg.ActorContext,
) (workspacepkg.CoordinationView, error) {
	return workspacepkg.CoordinationView{}, nil
}

func (*recordingRegistry) GetNetworkUsage(
	context.Context,
	store.NetworkUsageQuery,
) (store.NetworkUsageReport, error) {
	return store.NetworkUsageReport{}, nil
}

func (r *recordingRegistry) WriteNetworkChannel(context.Context, store.NetworkChannelEntry) error {
	return nil
}

func (r *recordingRegistry) CreateNetworkChannel(context.Context, store.NetworkChannelEntry) error {
	return nil
}

func (r *recordingRegistry) PatchNetworkChannel(
	context.Context,
	store.NetworkChannelRef,
	store.NetworkChannelPatch,
) error {
	return nil
}

func (r *recordingRegistry) GetNetworkChannel(
	context.Context,
	store.NetworkChannelRef,
) (store.NetworkChannelEntry, error) {
	return store.NetworkChannelEntry{}, sql.ErrNoRows
}

func (r *recordingRegistry) ListNetworkChannels(
	context.Context,
	store.NetworkChannelQuery,
) ([]store.NetworkChannelEntry, error) {
	return nil, nil
}

func (r *recordingRegistry) DeleteNetworkChannel(context.Context, store.NetworkChannelRef) error {
	return nil
}

func (r *recordingRegistry) GetOnboardingStatus(context.Context) (store.OnboardingStatus, error) {
	return store.OnboardingStatus{}, nil
}

func (r *recordingRegistry) CompleteOnboarding(context.Context, string) (store.OnboardingStatus, error) {
	return store.OnboardingStatus{Completed: true}, nil
}

func (r *recordingRegistry) ResetOnboarding(context.Context) (store.OnboardingStatus, error) {
	return store.OnboardingStatus{}, nil
}

func (r *recordingRegistry) GetAppMetadata(context.Context, string) (string, bool, error) {
	return "", false, nil
}

func (r *recordingRegistry) SetAppMetadata(context.Context, string, string) error {
	return nil
}

func (r *recordingRegistry) DeleteAppMetadata(context.Context, string) error {
	return nil
}

func (r *recordingRegistry) WriteNetworkMessage(context.Context, store.NetworkMessageEntry) error {
	return nil
}

func (r *recordingRegistry) ListNetworkMessages(
	context.Context,
	store.NetworkMessageQuery,
) ([]store.NetworkMessageEntry, error) {
	return nil, nil
}

func (r *recordingRegistry) ResolveDirectRoom(
	context.Context,
	store.NetworkDirectRoomEntry,
) (store.NetworkDirectRoomSummary, error) {
	return store.NetworkDirectRoomSummary{}, nil
}

func (r *recordingRegistry) WriteConversationMessage(
	context.Context,
	store.NetworkConversationMessage,
) (store.NetworkConversationWriteResult, error) {
	return store.NetworkConversationWriteResult{}, nil
}

func (r *recordingRegistry) ListThreads(
	context.Context,
	store.NetworkChannelRef,
	store.NetworkThreadQuery,
) (store.NetworkThreadPage, error) {
	return store.NetworkThreadPage{}, nil
}

func (r *recordingRegistry) GetThread(
	context.Context,
	store.NetworkChannelRef,
	string,
) (store.NetworkThreadSummary, error) {
	return store.NetworkThreadSummary{}, store.ErrNetworkConversationNotFound
}

func (r *recordingRegistry) ListDirectRooms(
	context.Context,
	store.NetworkChannelRef,
	store.NetworkDirectRoomQuery,
) (store.NetworkDirectRoomPage, error) {
	return store.NetworkDirectRoomPage{}, nil
}

func (r *recordingRegistry) GetDirectRoom(
	context.Context,
	store.NetworkChannelRef,
	string,
) (store.NetworkDirectRoomSummary, error) {
	return store.NetworkDirectRoomSummary{}, store.ErrNetworkConversationNotFound
}

func (r *recordingRegistry) ListConversationMessages(
	context.Context,
	store.NetworkConversationRef,
	store.NetworkConversationMessageQuery,
) ([]store.NetworkConversationMessage, error) {
	return nil, nil
}

func (r *recordingRegistry) GetWork(context.Context, string, string) (store.NetworkWorkEntry, error) {
	return store.NetworkWorkEntry{}, store.ErrNetworkConversationNotFound
}

func (r *recordingRegistry) PutNetworkSubscription(context.Context, store.NetworkSubscriptionEntry) error {
	return nil
}

func (r *recordingRegistry) PutNetworkSubscriptionWithChannel(
	context.Context,
	store.NetworkChannelEntry,
	store.NetworkSubscriptionEntry,
) error {
	return nil
}

func (r *recordingRegistry) ListNetworkSubscriptions(
	context.Context,
	store.NetworkSubscriptionQuery,
) ([]store.NetworkSubscriptionEntry, error) {
	return nil, nil
}

func (r *recordingRegistry) DeleteNetworkSubscription(context.Context, store.NetworkSubscriptionRef) error {
	return nil
}

func (r *recordingRegistry) PutNetworkTaskThreadOrigin(context.Context, store.NetworkTaskThreadOrigin) error {
	return nil
}

func (r *recordingRegistry) ListNetworkTaskThreadOrigins(
	context.Context,
	store.NetworkTaskThreadOriginQuery,
) ([]store.NetworkTaskThreadOrigin, error) {
	return nil, nil
}

func (r *recordingRegistry) PutTaskDesignationRollup(context.Context, store.TaskDesignationRollup) error {
	return nil
}

func (r *recordingRegistry) ListTaskDesignationRollups(
	context.Context,
	store.TaskDesignationRollupQuery,
) ([]store.TaskDesignationRollup, error) {
	return nil, nil
}

func (r *recordingRegistry) CreateTask(context.Context, taskpkg.Task) error {
	return nil
}

func (r *recordingRegistry) CreateTaskDefinition(
	context.Context,
	taskpkg.CreateTaskDefinitionMutation,
) error {
	return nil
}

func (r *recordingRegistry) UpdateTaskDefinition(
	context.Context,
	*taskpkg.UpdateTaskDefinitionMutation,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, nil
}

func (r *recordingRegistry) SetTaskExecutionProfile(
	context.Context,
	*taskpkg.ExecutionProfileMutation,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, nil
}

func (r *recordingRegistry) DeleteTaskExecutionProfile(
	context.Context,
	taskpkg.ExecutionProfileDeleteMutation,
) error {
	return nil
}

func (r *recordingRegistry) UpdateTask(context.Context, taskpkg.Task, taskpkg.ActorContext) error {
	return nil
}

func (r *recordingRegistry) DeleteTask(context.Context, string) error {
	return nil
}

func (r *recordingRegistry) GetTask(context.Context, string) (taskpkg.Task, error) {
	return taskpkg.Task{}, taskpkg.ErrTaskNotFound
}
func (r *recordingRegistry) ListTasks(context.Context, taskpkg.Query) ([]taskpkg.Summary, error) {
	return nil, nil
}

func (r *recordingRegistry) CountDirectChildren(context.Context, string) (int, error) {
	return 0, nil
}

func (r *recordingRegistry) CreateDependency(context.Context, taskpkg.Dependency) error {
	return nil
}

func (r *recordingRegistry) DeleteDependency(context.Context, string, string) error {
	return nil
}

func (r *recordingRegistry) ListDependencies(context.Context, string) ([]taskpkg.Dependency, error) {
	return nil, nil
}

func (r *recordingRegistry) ListTaskBlocks(context.Context, string, bool) ([]taskpkg.TaskBlock, error) {
	return nil, nil
}

func (r *recordingRegistry) HasOpenTaskBlocks(context.Context, string) (bool, error) {
	return false, nil
}

func (r *recordingRegistry) CreateTaskBlock(
	context.Context,
	taskpkg.CreateTaskBlockMutation,
) (taskpkg.BlockMutationResult, error) {
	return taskpkg.BlockMutationResult{}, nil
}

func (r *recordingRegistry) ClearTaskBlock(
	context.Context,
	taskpkg.ClearTaskBlockMutation,
) (taskpkg.TaskBlock, error) {
	return taskpkg.TaskBlock{}, nil
}

func (r *recordingRegistry) ClearTaskNeedsAttention(
	context.Context,
	taskpkg.NeedsAttentionClearMutation,
) (taskpkg.NeedsAttentionClearResult, error) {
	return taskpkg.NeedsAttentionClearResult{}, nil
}

func (r *recordingRegistry) ExpireTaskBlocks(
	context.Context,
	taskpkg.ExpireTaskBlocksMutation,
) (taskpkg.ExpireTaskBlocksResult, error) {
	return taskpkg.ExpireTaskBlocksResult{}, nil
}

func (r *recordingRegistry) BlockTaskAndReleaseRun(
	context.Context,
	taskpkg.BlockTaskAndReleaseRunMutation,
) (taskpkg.BlockTaskAndReleaseRunResult, error) {
	return taskpkg.BlockTaskAndReleaseRunResult{}, nil
}

func (r *recordingRegistry) ListDependents(context.Context, string) ([]taskpkg.Dependency, error) {
	return nil, nil
}

func (r *recordingRegistry) CountDependencies(context.Context, string) (int, error) {
	return 0, nil
}

func (r *recordingRegistry) HasDependencyPath(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *recordingRegistry) CreateTaskRun(context.Context, taskpkg.Run) error {
	return nil
}

func (r *recordingRegistry) UpdateTaskRun(context.Context, taskpkg.Run) error {
	return nil
}

func (r *recordingRegistry) CompleteRunSettlement(
	context.Context,
	taskpkg.Run,
	taskpkg.ActorContext,
) (taskpkg.CompletedRunSettlement, error) {
	return taskpkg.CompletedRunSettlement{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) GetTaskRun(context.Context, string) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) ListTaskRuns(context.Context, taskpkg.RunQuery) ([]taskpkg.Run, error) {
	return nil, nil
}

func (r *recordingRegistry) ListTaskRunsByStatus(
	_ context.Context,
	statuses []taskpkg.RunStatus,
) ([]taskpkg.Run, error) {
	if r.onListTaskRunsByStatus != nil {
		r.onListTaskRunsByStatus(append([]taskpkg.RunStatus(nil), statuses...))
	}
	return nil, nil
}

func (r *recordingRegistry) GetTaskTriageState(
	context.Context,
	string,
	taskpkg.ActorIdentity,
) (taskpkg.TriageState, error) {
	return taskpkg.TriageState{}, taskpkg.ErrTaskTriageStateNotFound
}

func (r *recordingRegistry) UpsertTaskTriageState(context.Context, taskpkg.TriageState) error {
	return nil
}

func (r *recordingRegistry) GetExecutionProfile(
	context.Context,
	string,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
}

func (r *recordingRegistry) UpsertExecutionProfile(
	context.Context,
	*taskpkg.ExecutionProfile,
) (taskpkg.ExecutionProfile, error) {
	return taskpkg.ExecutionProfile{}, nil
}

func (r *recordingRegistry) DeleteExecutionProfile(context.Context, string) error {
	return taskpkg.ErrExecutionProfileNotFound
}

func (r *recordingRegistry) RequestRunReview(
	context.Context,
	*taskpkg.RunReview,
) (taskpkg.RunReview, bool, error) {
	return taskpkg.RunReview{}, false, taskpkg.ErrRunReviewNotFound
}

func (r *recordingRegistry) GetRunReview(context.Context, string) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (r *recordingRegistry) RecordRunReview(
	context.Context,
	taskpkg.RecordRunReviewRequest,
	taskpkg.ActorContext,
	time.Time,
	string,
) (taskpkg.RunReviewResult, error) {
	return taskpkg.RunReviewResult{}, taskpkg.ErrRunReviewNotFound
}

func (r *recordingRegistry) BindRunReviewSession(
	context.Context,
	taskpkg.BindRunReviewSessionRequest,
	time.Time,
) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (r *recordingRegistry) LookupRunReviewBySession(context.Context, string) (taskpkg.RunReview, error) {
	return taskpkg.RunReview{}, taskpkg.ErrRunReviewNotFound
}

func (r *recordingRegistry) ListRunReviews(context.Context, taskpkg.RunReviewQuery) ([]taskpkg.RunReview, error) {
	return nil, nil
}

func (r *recordingRegistry) ListTaskTriageStates(
	context.Context,
	taskpkg.ActorIdentity,
) ([]taskpkg.TriageState, error) {
	return nil, nil
}

func (r *recordingRegistry) CountActiveSessionBindings(context.Context, string) (int, error) {
	return 0, nil
}

func (r *recordingRegistry) ClaimNextRun(
	context.Context,
	taskpkg.ClaimCriteria,
) (taskpkg.ClaimResult, error) {
	return taskpkg.ClaimResult{}, taskpkg.ErrNoClaimableRun
}

func (r *recordingRegistry) HeartbeatRunLease(
	context.Context,
	taskpkg.LeaseHeartbeat,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) ReleaseRunLease(
	context.Context,
	taskpkg.LeaseRelease,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) ForceReleaseTaskRun(
	context.Context,
	taskpkg.ForceReleaseRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	return taskpkg.ForceRunMutationResult{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) ForceFailTaskRun(
	context.Context,
	taskpkg.ForceFailRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	return taskpkg.ForceRunMutationResult{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) RetryTaskRun(
	context.Context,
	taskpkg.RetryRunMutation,
) (taskpkg.RetryRunResult, error) {
	return taskpkg.RetryRunResult{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) RecoverTaskRun(
	context.Context,
	taskpkg.RecoverRunMutation,
) (taskpkg.RetryRunResult, error) {
	return taskpkg.RetryRunResult{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) MarkTaskRunNeedsAttention(
	context.Context,
	string,
	string,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) CompleteRunLease(
	context.Context,
	taskpkg.LeaseCompletion,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) CompleteRunLeaseSettlement(
	context.Context,
	taskpkg.LeaseCompletion,
) (taskpkg.CompletedRunSettlement, error) {
	return taskpkg.CompletedRunSettlement{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) FailRunLease(
	context.Context,
	taskpkg.LeaseFailure,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) RecoverExpiredRunLeases(
	context.Context,
	taskpkg.ExpiredLeaseRecovery,
) ([]taskpkg.ExpiredLeaseRecoveryResult, error) {
	return nil, nil
}

func (r *recordingRegistry) ReserveQueuedRun(
	context.Context,
	taskpkg.QueueRunReservation,
) (taskpkg.Task, taskpkg.Run, bool, error) {
	return taskpkg.Task{}, taskpkg.Run{}, false, taskpkg.ErrTaskNotFound
}

func (r *recordingRegistry) CompleteCoordinatorAndEnqueueNext(
	context.Context,
	taskpkg.CoordinatorCompletion,
	taskpkg.GenerationStateFinalizer,
) (taskpkg.CoordinatorCompletionResult, error) {
	return taskpkg.CoordinatorCompletionResult{}, taskpkg.ErrTaskRunNotFound
}

func (r *recordingRegistry) CreateTaskEvent(context.Context, taskpkg.Event) error {
	return nil
}

func (r *recordingRegistry) ListTaskEvents(context.Context, taskpkg.EventQuery) ([]taskpkg.Event, error) {
	return nil, nil
}

func (r *recordingRegistry) TaskWakeEventExists(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *recordingRegistry) GetTaskEventRecord(context.Context, string) (taskpkg.EventRecord, error) {
	return taskpkg.EventRecord{}, taskpkg.ErrTaskEventNotFound
}

func (r *recordingRegistry) ListTaskEventRecords(
	context.Context,
	taskpkg.EventRecordQuery,
) ([]taskpkg.EventRecord, error) {
	return nil, nil
}

func (r *recordingRegistry) GetTaskRunByIdempotencyKey(
	context.Context,
	string,
	taskpkg.Origin,
) (taskpkg.Run, error) {
	return taskpkg.Run{}, taskpkg.ErrTaskRunIdempotencyNotFound
}

func (r *recordingRegistry) SaveTaskRunIdempotency(context.Context, taskpkg.RunIdempotency) error {
	return nil
}
func (r *recordingRegistry) Close(context.Context) error {
	if r.onClose != nil {
		r.onClose()
	}
	return nil
}

type recordingNotifier struct {
	events []string
}

func (n *recordingNotifier) OnSessionCreated(context.Context, *session.Session) {
	n.events = append(n.events, "created")
}

func (n *recordingNotifier) OnSessionStopped(context.Context, *session.Session) {
	n.events = append(n.events, "stopped")
}

func (n *recordingNotifier) OnAgentEvent(context.Context, string, any) {
	n.events = append(n.events, "agent")
}

type recordingHookTelemetrySink struct {
	calls []struct {
		sessionID string
		record    hookspkg.HookRunRecord
	}
}

func (s *recordingHookTelemetrySink) WriteHookRecord(
	_ context.Context,
	sessionID string,
	record hookspkg.HookRunRecord,
) error {
	s.calls = append(s.calls, struct {
		sessionID string
		record    hookspkg.HookRunRecord
	}{
		sessionID: sessionID,
		record:    record,
	})
	return nil
}

func (s *recordingHookTelemetrySink) count() int {
	if s == nil {
		return 0
	}
	return len(s.calls)
}

type noopMemoryObserver struct{}

func (noopMemoryObserver) OnMemoryConsolidated(context.Context, automationpkg.MemoryConsolidatedEvent) error {
	return nil
}

type fakeAutomationManager struct {
	jobs              []automationpkg.Job
	triggers          []automationpkg.Trigger
	runs              []automationpkg.Run
	status            automationpkg.ManagerStatus
	startCount        int
	shutdownCount     int
	startErr          error
	shutdownErr       error
	onStart           func()
	onShutdown        func()
	sessionObserver   session.Notifier
	hookTelemetrySink hookspkg.TelemetrySink
}

func (f *fakeAutomationManager) Start(context.Context) error {
	f.startCount++
	if f.onStart != nil {
		f.onStart()
	}
	f.status.Running = true
	return f.startErr
}

func (f *fakeAutomationManager) Shutdown(context.Context) error {
	f.shutdownCount++
	if f.onShutdown != nil {
		f.onShutdown()
	}
	f.status.Running = false
	f.status.SchedulerRunning = false
	return f.shutdownErr
}

func (f *fakeAutomationManager) ListSuggestions(
	context.Context,
	string,
	automationpkg.SuggestionStatus,
) ([]automationpkg.Suggestion, error) {
	return nil, nil
}

func (f *fakeAutomationManager) AcceptSuggestion(
	context.Context,
	string,
	string,
) (automationpkg.SuggestionAcceptance, error) {
	return automationpkg.SuggestionAcceptance{}, automationpkg.ErrSuggestionNotFound
}

func (f *fakeAutomationManager) DismissSuggestion(
	context.Context,
	string,
	string,
) (automationpkg.Suggestion, error) {
	return automationpkg.Suggestion{}, automationpkg.ErrSuggestionNotFound
}

func (f *fakeAutomationManager) Jobs(context.Context) ([]automationpkg.Job, error) {
	return append([]automationpkg.Job(nil), f.jobs...), nil
}

func (f *fakeAutomationManager) ListJobs(
	_ context.Context,
	query automationpkg.JobListQuery,
) (automationpkg.JobListPage, error) {
	return automationpkg.BuildJobListPage(f.jobs, query)
}

func (f *fakeAutomationManager) GetJob(_ context.Context, id string) (automationpkg.Job, error) {
	for _, job := range f.jobs {
		if job.ID == strings.TrimSpace(id) {
			return job, nil
		}
	}
	return automationpkg.Job{}, automationpkg.ErrJobNotFound
}

func (f *fakeAutomationManager) CreateJob(_ context.Context, job automationpkg.Job) (automationpkg.Job, error) {
	f.jobs = append(f.jobs, job)
	return job, nil
}

func (f *fakeAutomationManager) UpdateJob(_ context.Context, job automationpkg.Job) (automationpkg.Job, error) {
	for i := range f.jobs {
		if f.jobs[i].ID == strings.TrimSpace(job.ID) {
			f.jobs[i] = job
			return job, nil
		}
	}
	return automationpkg.Job{}, automationpkg.ErrJobNotFound
}

func (f *fakeAutomationManager) DeleteJob(_ context.Context, id string) error {
	for i := range f.jobs {
		if f.jobs[i].ID == strings.TrimSpace(id) {
			f.jobs = append(f.jobs[:i], f.jobs[i+1:]...)
			return nil
		}
	}
	return automationpkg.ErrJobNotFound
}

func (f *fakeAutomationManager) TriggerJob(ctx context.Context, id string) (automationpkg.Run, error) {
	return f.TriggerJobWithPayload(ctx, id, nil)
}

func (f *fakeAutomationManager) TriggerJobWithPayload(
	_ context.Context,
	id string,
	_ map[string]any,
) (automationpkg.Run, error) {
	run := automationpkg.Run{
		ID:      "run-" + strings.TrimSpace(id),
		JobID:   strings.TrimSpace(id),
		Status:  automationpkg.RunCompleted,
		Attempt: 1,
	}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *fakeAutomationManager) Triggers(context.Context) ([]automationpkg.Trigger, error) {
	return append([]automationpkg.Trigger(nil), f.triggers...), nil
}

func (f *fakeAutomationManager) ListTriggers(
	_ context.Context,
	query automationpkg.TriggerListQuery,
) (automationpkg.TriggerListPage, error) {
	return automationpkg.BuildTriggerListPage(f.triggers, query)
}

func (f *fakeAutomationManager) GetTrigger(_ context.Context, id string) (automationpkg.Trigger, error) {
	for _, trigger := range f.triggers {
		if trigger.ID == strings.TrimSpace(id) {
			return trigger, nil
		}
	}
	return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
}

func (f *fakeAutomationManager) CreateTrigger(
	_ context.Context,
	trigger automationpkg.Trigger,
	_ automationpkg.WebhookSecretWrite,
) (automationpkg.Trigger, error) {
	f.triggers = append(f.triggers, trigger)
	return trigger, nil
}

func (f *fakeAutomationManager) UpdateTrigger(
	_ context.Context,
	trigger automationpkg.Trigger,
	_ *automationpkg.WebhookSecretWrite,
) (automationpkg.Trigger, error) {
	for i := range f.triggers {
		if f.triggers[i].ID == strings.TrimSpace(trigger.ID) {
			f.triggers[i] = trigger
			return trigger, nil
		}
	}
	return automationpkg.Trigger{}, automationpkg.ErrTriggerNotFound
}

func (f *fakeAutomationManager) DeleteTrigger(_ context.Context, id string) error {
	for i := range f.triggers {
		if f.triggers[i].ID == strings.TrimSpace(id) {
			f.triggers = append(f.triggers[:i], f.triggers[i+1:]...)
			return nil
		}
	}
	return automationpkg.ErrTriggerNotFound
}

func (f *fakeAutomationManager) Runs(context.Context, automationpkg.RunQuery) ([]automationpkg.Run, error) {
	return append([]automationpkg.Run(nil), f.runs...), nil
}

func (f *fakeAutomationManager) ListRuns(_ context.Context, query automationpkg.RunQuery) ([]automationpkg.Run, error) {
	runs := make([]automationpkg.Run, 0, len(f.runs))
	for _, run := range f.runs {
		if query.JobID != "" && run.JobID != query.JobID {
			continue
		}
		if query.TriggerID != "" && run.TriggerID != query.TriggerID {
			continue
		}
		if query.Status != "" && run.Status != query.Status {
			continue
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (f *fakeAutomationManager) GetRun(_ context.Context, id string) (automationpkg.Run, error) {
	for _, run := range f.runs {
		if run.ID == strings.TrimSpace(id) {
			return run, nil
		}
	}
	return automationpkg.Run{}, automationpkg.ErrRunNotFound
}

func (f *fakeAutomationManager) Status(context.Context) (automationpkg.ManagerStatus, error) {
	return f.status, nil
}

func (f *fakeAutomationManager) SetJobEnabled(context.Context, string, bool) (automationpkg.Job, error) {
	return automationpkg.Job{}, nil
}

func (f *fakeAutomationManager) SetTriggerEnabled(context.Context, string, bool) (automationpkg.Trigger, error) {
	return automationpkg.Trigger{}, nil
}

func (f *fakeAutomationManager) HandleWebhook(
	context.Context,
	automationpkg.WebhookRequest,
) (automationpkg.TriggerResult, error) {
	return automationpkg.TriggerResult{}, nil
}

func (f *fakeAutomationManager) SyncManagedDefinitions(
	_ context.Context,
	source automationpkg.JobSource,
	desiredJobs []automationpkg.Job,
	desiredTriggers []automationpkg.Trigger,
) (automationpkg.SyncStats, error) {
	f.jobs = slices.DeleteFunc(append([]automationpkg.Job(nil), f.jobs...), func(job automationpkg.Job) bool {
		return job.Source == source
	})
	f.jobs = append(f.jobs, desiredJobs...)

	f.triggers = slices.DeleteFunc(
		append([]automationpkg.Trigger(nil), f.triggers...),
		func(trigger automationpkg.Trigger) bool {
			return trigger.Source == source
		},
	)
	f.triggers = append(f.triggers, desiredTriggers...)

	return automationpkg.SyncStats{
		JobsSynced:     len(desiredJobs),
		TriggersSynced: len(desiredTriggers),
		SyncedAt:       time.Now().UTC(),
	}, nil
}

func (f *fakeAutomationManager) FireExtensionTrigger(
	_ context.Context,
	request automationpkg.ExtensionTriggerRequest,
) (automationpkg.TriggerResult, error) {
	return automationpkg.TriggerResult{
		Matched: 0,
		Runs:    append([]automationpkg.Run(nil), f.runs...),
	}, request.Validate("extension_trigger")
}

func (f *fakeAutomationManager) SessionObserver() session.Notifier {
	if f.sessionObserver != nil {
		return f.sessionObserver
	}
	return &recordingNotifier{}
}

func (f *fakeAutomationManager) HookTelemetrySink() hookspkg.TelemetrySink {
	if f.hookTelemetrySink != nil {
		return f.hookTelemetrySink
	}
	return &recordingHookTelemetrySink{}
}

func (*fakeAutomationManager) MemoryObserver() automationpkg.MemoryConsolidationObserver {
	return noopMemoryObserver{}
}

type fakeHookRuntime struct {
	version            int64
	onRebuild          func(context.Context) error
	onClose            func()
	onDispatchCreate   func(context.Context, hookspkg.SessionPostCreatePayload) error
	onDispatchStop     func(context.Context, hookspkg.SessionPostStopPayload) error
	onTurnStart        func(context.Context, hookspkg.TurnStartPayload) error
	onTurnEnd          func(context.Context, hookspkg.TurnEndPayload) error
	onMessageStart     func(context.Context, hookspkg.MessageStartPayload) error
	onMessageDelta     func(context.Context, hookspkg.MessageDeltaPayload) error
	onMessageEnd       func(context.Context, hookspkg.MessageEndPayload) error
	onMessagePersisted func(context.Context, hookspkg.SessionMessagePersistedPayload) error
	onToolPreCall      func(context.Context, hookspkg.ToolPreCallPayload) error
	onToolPostCall     func(context.Context, hookspkg.ToolPostCallPayload) error
	onToolPostError    func(context.Context, hookspkg.ToolPostErrorPayload) error
	onPermRequest      func(context.Context, hookspkg.PermissionRequestPayload) error
	onPermResolved     func(context.Context, hookspkg.PermissionResolvedPayload) error
	onPermDenied       func(context.Context, hookspkg.PermissionDeniedPayload) error
	onPreCompact       func(context.Context, hookspkg.ContextPreCompactPayload) error
	onPostCompact      func(context.Context, hookspkg.ContextPostCompactPayload) error
	onTaskRunEnqueued  func(context.Context, hookspkg.TaskRunEnqueuedPayload) error
	onTaskRunPreClaim  func(context.Context, hookspkg.TaskRunPreClaimPayload) error
	onTaskRunPostClaim func(context.Context, hookspkg.TaskRunPostClaimPayload) error
	onTaskRunRecovered func(context.Context, hookspkg.TaskRunLeaseRecoveredPayload) error
	onLoopNodeTerminal func(context.Context, hookspkg.LoopNodeTerminalPayload) error
	onLoopTerminal     func(context.Context, hookspkg.LoopTerminalPayload) error
	onSpawnPreCreate   func(context.Context, hookspkg.SpawnPreCreatePayload) error
}

func (f *fakeHookRuntime) Rebuild(ctx context.Context) error {
	if f.onRebuild != nil {
		return f.onRebuild(ctx)
	}
	return nil
}

func (f *fakeHookRuntime) Close() {
	if f.onClose != nil {
		f.onClose()
	}
}

func (f *fakeHookRuntime) Version() int64 {
	return f.version
}

func (f *fakeHookRuntime) DispatchSessionPreCreate(
	_ context.Context,
	payload hookspkg.SessionPreCreatePayload,
) (hookspkg.SessionPreCreatePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionPostCreate(
	ctx context.Context,
	payload hookspkg.SessionPostCreatePayload,
) (hookspkg.SessionPostCreatePayload, error) {
	if f.onDispatchCreate != nil {
		return payload, f.onDispatchCreate(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionPreResume(
	_ context.Context,
	payload hookspkg.SessionPreResumePayload,
) (hookspkg.SessionPreResumePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionPostResume(
	_ context.Context,
	payload hookspkg.SessionPostResumePayload,
) (hookspkg.SessionPostResumePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionPreStop(
	_ context.Context,
	payload hookspkg.SessionPreStopPayload,
) (hookspkg.SessionPreStopPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionPostStop(
	ctx context.Context,
	payload hookspkg.SessionPostStopPayload,
) (hookspkg.SessionPostStopPayload, error) {
	if f.onDispatchStop != nil {
		return payload, f.onDispatchStop(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchInputPreSubmit(
	_ context.Context,
	payload hookspkg.InputPreSubmitPayload,
) (hookspkg.InputPreSubmitPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchPromptPostAssemble(
	_ context.Context,
	payload hookspkg.PromptPayload,
) (hookspkg.PromptPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchEventPreRecord(
	_ context.Context,
	payload hookspkg.EventPreRecordPayload,
) (hookspkg.EventPreRecordPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchEventPostRecord(
	_ context.Context,
	payload hookspkg.EventPostRecordPayload,
) (hookspkg.EventPostRecordPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAutomationJobPreFire(
	_ context.Context,
	payload hookspkg.AutomationJobPreFirePayload,
) (hookspkg.AutomationJobPreFirePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAutomationJobPostFire(
	_ context.Context,
	payload hookspkg.AutomationJobPostFirePayload,
) (hookspkg.AutomationJobPostFirePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAutomationTriggerPreFire(
	_ context.Context,
	payload hookspkg.AutomationTriggerPreFirePayload,
) (hookspkg.AutomationTriggerPreFirePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAutomationTriggerPostFire(
	_ context.Context,
	payload hookspkg.AutomationTriggerPostFirePayload,
) (hookspkg.AutomationTriggerPostFirePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAutomationRunCompleted(
	_ context.Context,
	payload hookspkg.AutomationRunCompletedPayload,
) (hookspkg.AutomationRunCompletedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAutomationRunFailed(
	_ context.Context,
	payload hookspkg.AutomationRunFailedPayload,
) (hookspkg.AutomationRunFailedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentPreStart(
	_ context.Context,
	payload hookspkg.AgentPreStartPayload,
) (hookspkg.AgentPreStartPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentSpawned(
	_ context.Context,
	payload hookspkg.AgentSpawnedPayload,
) (hookspkg.AgentSpawnedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentCrashed(
	_ context.Context,
	payload hookspkg.AgentCrashedPayload,
) (hookspkg.AgentCrashedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentStopped(
	_ context.Context,
	payload hookspkg.AgentStoppedPayload,
) (hookspkg.AgentStoppedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTurnStart(
	ctx context.Context,
	payload hookspkg.TurnStartPayload,
) (hookspkg.TurnStartPayload, error) {
	if f.onTurnStart != nil {
		return payload, f.onTurnStart(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTurnEnd(
	ctx context.Context,
	payload hookspkg.TurnEndPayload,
) (hookspkg.TurnEndPayload, error) {
	if f.onTurnEnd != nil {
		return payload, f.onTurnEnd(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchMessageStart(
	ctx context.Context,
	payload hookspkg.MessageStartPayload,
) (hookspkg.MessageStartPayload, error) {
	if f.onMessageStart != nil {
		return payload, f.onMessageStart(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchMessageDelta(
	ctx context.Context,
	payload hookspkg.MessageDeltaPayload,
) (hookspkg.MessageDeltaPayload, error) {
	if f.onMessageDelta != nil {
		return payload, f.onMessageDelta(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchMessageEnd(
	ctx context.Context,
	payload hookspkg.MessageEndPayload,
) (hookspkg.MessageEndPayload, error) {
	if f.onMessageEnd != nil {
		return payload, f.onMessageEnd(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionMessagePersisted(
	ctx context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) (hookspkg.SessionMessagePersistedPayload, error) {
	if f.onMessagePersisted != nil {
		return payload, f.onMessagePersisted(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchToolPreCall(
	ctx context.Context,
	payload hookspkg.ToolPreCallPayload,
) (hookspkg.ToolPreCallPayload, error) {
	if f.onToolPreCall != nil {
		return payload, f.onToolPreCall(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchToolPostCall(
	ctx context.Context,
	payload hookspkg.ToolPostCallPayload,
) (hookspkg.ToolPostCallPayload, error) {
	if f.onToolPostCall != nil {
		return payload, f.onToolPostCall(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchToolPostError(
	ctx context.Context,
	payload hookspkg.ToolPostErrorPayload,
) (hookspkg.ToolPostErrorPayload, error) {
	if f.onToolPostError != nil {
		return payload, f.onToolPostError(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchPermissionRequest(
	ctx context.Context,
	payload hookspkg.PermissionRequestPayload,
) (hookspkg.PermissionRequestPayload, error) {
	if f.onPermRequest != nil {
		return payload, f.onPermRequest(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchPermissionResolved(
	ctx context.Context,
	payload hookspkg.PermissionResolvedPayload,
) (hookspkg.PermissionResolvedPayload, error) {
	if f.onPermResolved != nil {
		return payload, f.onPermResolved(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchPermissionDenied(
	ctx context.Context,
	payload hookspkg.PermissionDeniedPayload,
) (hookspkg.PermissionDeniedPayload, error) {
	if f.onPermDenied != nil {
		return payload, f.onPermDenied(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchContextPreCompact(
	ctx context.Context,
	payload hookspkg.ContextPreCompactPayload,
) (hookspkg.ContextPreCompactPayload, error) {
	if f.onPreCompact != nil {
		return payload, f.onPreCompact(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchContextPostCompact(
	ctx context.Context,
	payload hookspkg.ContextPostCompactPayload,
) (hookspkg.ContextPostCompactPayload, error) {
	if f.onPostCompact != nil {
		return payload, f.onPostCompact(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSandboxPrepare(
	_ context.Context,
	payload hookspkg.SandboxPreparePayload,
) (hookspkg.SandboxPreparePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSandboxReady(
	_ context.Context,
	payload hookspkg.SandboxReadyPayload,
) (hookspkg.SandboxReadyPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSandboxSyncBefore(
	_ context.Context,
	payload hookspkg.SandboxSyncBeforePayload,
) (hookspkg.SandboxSyncBeforePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSandboxSyncAfter(
	_ context.Context,
	payload hookspkg.SandboxSyncAfterPayload,
) (hookspkg.SandboxSyncAfterPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSandboxStop(
	_ context.Context,
	payload hookspkg.SandboxStopPayload,
) (hookspkg.SandboxStopPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchCoordinatorPreSpawn(
	_ context.Context,
	payload hookspkg.CoordinatorPreSpawnPayload,
) (hookspkg.CoordinatorPreSpawnPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchCoordinatorSpawned(
	_ context.Context,
	payload hookspkg.CoordinatorSpawnedPayload,
) (hookspkg.CoordinatorSpawnedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchCoordinatorDecision(
	_ context.Context,
	payload hookspkg.CoordinatorDecisionPayload,
) (hookspkg.CoordinatorDecisionPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchCoordinatorStopped(
	_ context.Context,
	payload hookspkg.CoordinatorStoppedPayload,
) (hookspkg.CoordinatorStoppedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchCoordinatorFailed(
	_ context.Context,
	payload hookspkg.CoordinatorFailedPayload,
) (hookspkg.CoordinatorFailedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskBlocked(
	_ context.Context,
	payload hookspkg.TaskBlockedPayload,
) (hookspkg.TaskBlockedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskUnblocked(
	_ context.Context,
	payload hookspkg.TaskUnblockedPayload,
) (hookspkg.TaskUnblockedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskNeedsAttention(
	_ context.Context,
	payload hookspkg.TaskNeedsAttentionPayload,
) (hookspkg.TaskNeedsAttentionPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRecovered(
	_ context.Context,
	payload hookspkg.TaskRecoveredPayload,
) (hookspkg.TaskRecoveredPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskStatusChanged(
	_ context.Context,
	payload hookspkg.TaskStatusChangedPayload,
) (hookspkg.TaskStatusChangedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunEnqueued(
	ctx context.Context,
	payload hookspkg.TaskRunEnqueuedPayload,
) (hookspkg.TaskRunEnqueuedPayload, error) {
	if f.onTaskRunEnqueued != nil {
		return payload, f.onTaskRunEnqueued(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunPreClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPreClaimPayload,
) (hookspkg.TaskRunPreClaimPayload, error) {
	if f.onTaskRunPreClaim != nil {
		return payload, f.onTaskRunPreClaim(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunPostClaim(
	ctx context.Context,
	payload hookspkg.TaskRunPostClaimPayload,
) (hookspkg.TaskRunPostClaimPayload, error) {
	if f.onTaskRunPostClaim != nil {
		return payload, f.onTaskRunPostClaim(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunLeaseExtended(
	_ context.Context,
	payload hookspkg.TaskRunLeaseExtendedPayload,
) (hookspkg.TaskRunLeaseExtendedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunLeaseExpired(
	_ context.Context,
	payload hookspkg.TaskRunLeaseExpiredPayload,
) (hookspkg.TaskRunLeaseExpiredPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunLeaseRecovered(
	ctx context.Context,
	payload hookspkg.TaskRunLeaseRecoveredPayload,
) (hookspkg.TaskRunLeaseRecoveredPayload, error) {
	if f.onTaskRunRecovered != nil {
		return payload, f.onTaskRunRecovered(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunReleased(
	_ context.Context,
	payload hookspkg.TaskRunReleasedPayload,
) (hookspkg.TaskRunReleasedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunCompleted(
	_ context.Context,
	payload hookspkg.TaskRunCompletedPayload,
) (hookspkg.TaskRunCompletedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchTaskRunFailed(
	_ context.Context,
	payload hookspkg.TaskRunFailedPayload,
) (hookspkg.TaskRunFailedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopStarted(
	_ context.Context,
	payload hookspkg.LoopStartedPayload,
) (hookspkg.LoopStartedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopGenerationPre(
	_ context.Context,
	payload hookspkg.LoopGenerationPrePayload,
) (hookspkg.LoopGenerationPrePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopGenerationPost(
	_ context.Context,
	payload hookspkg.LoopGenerationPostPayload,
) (hookspkg.LoopGenerationPostPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopGatePre(
	_ context.Context,
	payload hookspkg.LoopGatePrePayload,
) (hookspkg.LoopGatePrePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopGatePost(
	_ context.Context,
	payload hookspkg.LoopGatePostPayload,
) (hookspkg.LoopGatePostPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopNodeTerminal(
	ctx context.Context,
	payload hookspkg.LoopNodeTerminalPayload,
) (hookspkg.LoopNodeTerminalPayload, error) {
	if f.onLoopNodeTerminal != nil {
		return payload, f.onLoopNodeTerminal(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchLoopTerminal(
	ctx context.Context,
	payload hookspkg.LoopTerminalPayload,
) (hookspkg.LoopTerminalPayload, error) {
	if f.onLoopTerminal != nil {
		return payload, f.onLoopTerminal(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSpawnPreCreate(
	ctx context.Context,
	payload hookspkg.SpawnPreCreatePayload,
) (hookspkg.SpawnPreCreatePayload, error) {
	if f.onSpawnPreCreate != nil {
		return payload, f.onSpawnPreCreate(ctx, payload)
	}
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSpawnCreated(
	_ context.Context,
	payload hookspkg.SpawnCreatedPayload,
) (hookspkg.SpawnCreatedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSpawnParentStopped(
	_ context.Context,
	payload hookspkg.SpawnParentStoppedPayload,
) (hookspkg.SpawnParentStoppedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSpawnTTLExpired(
	_ context.Context,
	payload hookspkg.SpawnTTLExpiredPayload,
) (hookspkg.SpawnTTLExpiredPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSpawnReaped(
	_ context.Context,
	payload hookspkg.SpawnReapedPayload,
) (hookspkg.SpawnReapedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentSoulSnapshotResolved(
	_ context.Context,
	payload hookspkg.AgentSoulSnapshotResolvedPayload,
) (hookspkg.AgentSoulSnapshotResolvedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentSoulMutationAfter(
	_ context.Context,
	payload hookspkg.AgentSoulMutationAfterPayload,
) (hookspkg.AgentSoulMutationAfterPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentHeartbeatPolicyResolved(
	_ context.Context,
	payload hookspkg.AgentHeartbeatPolicyResolvedPayload,
) (hookspkg.AgentHeartbeatPolicyResolvedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentHeartbeatWakeBefore(
	_ context.Context,
	payload hookspkg.AgentHeartbeatWakeBeforePayload,
) (hookspkg.AgentHeartbeatWakeBeforePayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchAgentHeartbeatWakeAfter(
	_ context.Context,
	payload hookspkg.AgentHeartbeatWakeAfterPayload,
) (hookspkg.AgentHeartbeatWakeAfterPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchSessionHealthUpdateAfter(
	_ context.Context,
	payload hookspkg.SessionHealthUpdateAfterPayload,
) (hookspkg.SessionHealthUpdateAfterPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkPeerJoined(
	_ context.Context,
	payload hookspkg.NetworkPeerJoinedPayload,
) (hookspkg.NetworkPeerJoinedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkPeerLeft(
	_ context.Context,
	payload hookspkg.NetworkPeerLeftPayload,
) (hookspkg.NetworkPeerLeftPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkThreadOpened(
	_ context.Context,
	payload hookspkg.NetworkThreadOpenedPayload,
) (hookspkg.NetworkThreadOpenedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkDirectRoomOpened(
	_ context.Context,
	payload hookspkg.NetworkDirectRoomOpenedPayload,
) (hookspkg.NetworkDirectRoomOpenedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkMessagePersisted(
	_ context.Context,
	payload hookspkg.NetworkMessagePersistedPayload,
) (hookspkg.NetworkMessagePersistedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkWorkOpened(
	_ context.Context,
	payload hookspkg.NetworkWorkOpenedPayload,
) (hookspkg.NetworkWorkOpenedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkWorkTransitioned(
	_ context.Context,
	payload hookspkg.NetworkWorkTransitionedPayload,
) (hookspkg.NetworkWorkTransitionedPayload, error) {
	return payload, nil
}

func (f *fakeHookRuntime) DispatchNetworkWorkClosed(
	_ context.Context,
	payload hookspkg.NetworkWorkClosedPayload,
) (hookspkg.NetworkWorkClosedPayload, error) {
	return payload, nil
}

func testHookExecutorResolver(native map[string]hookspkg.Executor) hookspkg.ExecutorResolver {
	return func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
		if decl.ExecutorKind == hookspkg.HookExecutorNative {
			executor := native[strings.TrimSpace(decl.Name)]
			if executor == nil {
				return nil, errors.New("missing native executor")
			}
			return executor, nil
		}
		return defaultDaemonExecutorResolver(decl)
	}
}

type fakeDreamService struct {
	mu             sync.Mutex
	shouldRun      bool
	shouldRunErr   error
	runErr         error
	shouldRunCalls int
	runCalls       int
	runHook        func(context.Context, memory.SessionSpawner, string) error
}

type fakeHookBindingPublisher func(context.Context) error

func (f fakeHookBindingPublisher) Sync(ctx context.Context) error {
	if f == nil {
		return nil
	}
	return f(ctx)
}

func (f *fakeDreamService) ShouldRun() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shouldRunCalls++
	return f.shouldRun, f.shouldRunErr
}

func (f *fakeDreamService) Run(ctx context.Context, spawn memory.SessionSpawner, workspace string) error {
	f.mu.Lock()
	f.runCalls++
	runHook := f.runHook
	runErr := f.runErr
	f.mu.Unlock()

	if runHook != nil {
		return runHook(ctx, spawn, workspace)
	}
	return runErr
}

func (f *fakeDreamService) runCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.runCalls
}

type portReportingServer struct {
	port int
}

func (s portReportingServer) Start(context.Context) error {
	return nil
}

func (s portReportingServer) Shutdown(context.Context) error {
	return nil
}

func (s portReportingServer) Port() int {
	return s.port
}

const (
	daemonExtensionHelperEnvKey      = "AGH_TEST_DAEMON_EXTENSION_HELPER"
	daemonExtensionHelperScenarioKey = "AGH_TEST_DAEMON_EXTENSION_SCENARIO"
	daemonExtensionHelperMarkerKey   = "AGH_TEST_DAEMON_EXTENSION_MARKER"
)

func TestDaemonExtensionHelperProcess(_ *testing.T) {
	if os.Getenv(daemonExtensionHelperEnvKey) != "1" {
		return
	}

	server := newDaemonExtensionHelperServer(
		strings.TrimSpace(os.Getenv(daemonExtensionHelperScenarioKey)),
		strings.TrimSpace(os.Getenv(daemonExtensionHelperMarkerKey)),
	)
	os.Exit(server.run())
}

type fakeExtensionRuntime struct {
	startCount  int
	stopCount   int
	reloadCount int
	startErr    error
	stopErr     error
	reloadErr   error
	hookDecls   []hookspkg.HookDecl
	hookErr     error
	getExt      *extensionpkg.Extension
	getErr      error
	getFn       func(string) (*extensionpkg.Extension, error)
	onStart     func()
	onStop      func()
	onReload    func(context.Context) error
}

func (f *fakeExtensionRuntime) Start(context.Context) error {
	f.startCount++
	if f.onStart != nil {
		f.onStart()
	}
	return f.startErr
}

func (f *fakeExtensionRuntime) Stop(context.Context) error {
	f.stopCount++
	if f.onStop != nil {
		f.onStop()
	}
	return f.stopErr
}

func (f *fakeExtensionRuntime) Reload(ctx context.Context) error {
	f.reloadCount++
	if f.onReload != nil {
		if err := f.onReload(ctx); err != nil {
			return err
		}
	}
	return f.reloadErr
}

func (f *fakeExtensionRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if f.getFn != nil {
		return f.getFn(name)
	}
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.getExt != nil {
		return f.getExt, nil
	}
	return nil, extensionpkg.ErrExtensionNotFound
}

func (f *fakeExtensionRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	decls := make([]hookspkg.HookDecl, 0, len(f.hookDecls))
	for _, decl := range f.hookDecls {
		cloned := decl
		cloned.Args = append([]string(nil), decl.Args...)
		if len(decl.Env) > 0 {
			cloned.Env = make(map[string]string, len(decl.Env))
			maps.Copy(cloned.Env, decl.Env)
		}
		if len(decl.Metadata) > 0 {
			cloned.Metadata = make(map[string]string, len(decl.Metadata))
			maps.Copy(cloned.Metadata, decl.Metadata)
		}
		decls = append(decls, cloned)
	}
	return decls, f.hookErr
}

type daemonTestExtensionOptions struct {
	runtimeCommand    string
	runtimeArgs       []string
	runtimeEnv        map[string]string
	hookCommand       string
	hookArgs          []string
	hookEvent         hookspkg.HookEvent
	capabilities      []string
	actions           []string
	security          []string
	bridgePlatform    string
	bridgeDisplayName string
}

func openDaemonTestGlobalDB(t *testing.T) *globaldb.GlobalDB {
	t.Helper()

	databasePath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	cloneDaemonTestStoreSeed(t, databasePath)
	db, err := globaldb.OpenGlobalDB(testutil.Context(t), databasePath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})
	return db
}

func cloneDaemonTestStoreSeed(t *testing.T, databasePath string) {
	t.Helper()

	if err := daemonTestStoreSeed.Clone(databasePath); err != nil {
		t.Fatalf("daemon store seed Clone() error = %v", err)
	}
}

func installDaemonTestExtension(
	t *testing.T,
	db *globaldb.GlobalDB,
	name string,
	opts daemonTestExtensionOptions,
	enabled bool,
) string {
	t.Helper()

	if db == nil {
		t.Fatal("installDaemonTestExtension() db = nil")
	}

	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	manifestPath := filepath.Join(dir, "extension.toml")
	if err := os.WriteFile(manifestPath, []byte(daemonTestExtensionManifest(name, opts)), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", manifestPath, err)
	}

	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", dir, err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", dir, err)
	}

	registry := extensionpkg.NewRegistry(db.DB())
	if err := registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("Registry.Install(%q) error = %v", name, err)
	}
	if !enabled {
		if err := registry.Disable(name); err != nil {
			t.Fatalf("Registry.Disable(%q) error = %v", name, err)
		}
	}

	return dir
}

func daemonTestExtensionManifest(name string, opts daemonTestExtensionOptions) string {
	command := strings.TrimSpace(opts.runtimeCommand)
	if command == "" {
		command = "fake-extension"
	}
	capabilities := append([]string(nil), opts.capabilities...)
	if opts.capabilities == nil {
		capabilities = []string{"memory.backend"}
	}
	actions := append([]string(nil), opts.actions...)
	if opts.actions == nil {
		actions = []string{"sessions/list"}
	}
	security := append([]string(nil), opts.security...)
	if opts.security == nil {
		security = []string{"session.read"}
	}
	bridgePlatform := strings.TrimSpace(opts.bridgePlatform)
	bridgeDisplayName := strings.TrimSpace(opts.bridgeDisplayName)
	if slices.Contains(capabilities, extensionprotocol.CapabilityProvideBridgeAdapter) {
		if bridgePlatform == "" {
			bridgePlatform = "telegram"
		}
		if bridgeDisplayName == "" {
			bridgeDisplayName = "Telegram"
		}
	}

	event := opts.hookEvent
	if event == "" {
		event = hookspkg.HookSessionPostCreate
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, `[extension]
name = %q
version = "0.2.1"
description = "Daemon extension test fixture"
min_agh_version = "0.5.0"

[resources]
`, name)

	if strings.TrimSpace(opts.hookCommand) != "" {
		fmt.Fprintf(&builder, `
[[resources.hooks]]
name = %q
event = %q
mode = "sync"
executor.kind = "subprocess"
executor.command = %q
`, name+"-hook", string(event), opts.hookCommand)
		if len(opts.hookArgs) > 0 {
			builder.WriteString("executor.args = " + daemonTOMLStringArray(opts.hookArgs) + "\n")
		}
	}

	builder.WriteString(`
[capabilities]
provides = ` + daemonTOMLStringArray(capabilities) + `

`)
	builder.WriteString(`
[actions]
requires = ` + daemonTOMLStringArray(actions) + `

[subprocess]
command = ` + fmt.Sprintf("%q", command) + `
`)
	if len(opts.runtimeArgs) > 0 {
		builder.WriteString("args = " + daemonTOMLStringArray(opts.runtimeArgs) + "\n")
	}
	if len(opts.runtimeEnv) > 0 {
		builder.WriteString("\n[subprocess.env]\n")
		keys := make([]string, 0, len(opts.runtimeEnv))
		for key := range opts.runtimeEnv {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			fmt.Fprintf(&builder, "%s = %q\n", key, opts.runtimeEnv[key])
		}
	}

	builder.WriteString(`
[security]
capabilities = ` + daemonTOMLStringArray(security) + `
`)
	if bridgePlatform != "" || bridgeDisplayName != "" {
		fmt.Fprintf(&builder, `
[bridge]
platform = %q
display_name = %q
`, bridgePlatform, bridgeDisplayName)
	}

	return builder.String()
}

func daemonTOMLStringArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func TestDaemonTestExtensionManifest(t *testing.T) {
	t.Run("ShouldApplyDefaultListsWhenOptionsAreNil", func(t *testing.T) {
		t.Parallel()

		manifest := daemonTestExtensionManifest("service-ext", daemonTestExtensionOptions{})
		for _, expected := range []string{
			`provides = ["memory.backend"]`,
			`requires = ["sessions/list"]`,
			`capabilities = ["session.read"]`,
		} {
			if !strings.Contains(manifest, expected) {
				t.Fatalf("daemonTestExtensionManifest() missing default %q in manifest %q", expected, manifest)
			}
		}
	})

	t.Run("ShouldPreserveExplicitEmptyLists", func(t *testing.T) {
		t.Parallel()

		manifest := daemonTestExtensionManifest("service-ext", daemonTestExtensionOptions{
			capabilities: []string{},
			actions:      []string{},
			security:     []string{},
		})

		for _, expected := range []string{
			"provides = []",
			"requires = []",
			"capabilities = []",
		} {
			if !strings.Contains(manifest, expected) {
				t.Fatalf(
					"daemonTestExtensionManifest() missing explicit empty list %q in manifest %q",
					expected,
					manifest,
				)
			}
		}
		for _, unexpected := range []string{"memory.backend", "sessions/list", "session.read"} {
			if strings.Contains(manifest, unexpected) {
				t.Fatalf(
					"daemonTestExtensionManifest() unexpectedly injected %q into manifest %q",
					unexpected,
					manifest,
				)
			}
		}
	})

	t.Run("ShouldEmitBridgeMetadataForBridgeAdapters", func(t *testing.T) {
		t.Parallel()

		manifest := daemonTestExtensionManifest("bridge-ext", daemonTestExtensionOptions{
			capabilities: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
		})
		for _, expected := range []string{
			`provides = ["bridge.adapter"]`,
			`[bridge]`,
			`platform = "telegram"`,
			`display_name = "Telegram"`,
		} {
			if !strings.Contains(manifest, expected) {
				t.Fatalf("daemonTestExtensionManifest() missing bridge metadata %q in manifest %q", expected, manifest)
			}
		}
	})
}

func TestDaemonExtensionHelperHarness(t *testing.T) {
	t.Parallel()

	command := daemonExtensionHelperCommand(t)
	if strings.TrimSpace(command) == "" {
		t.Fatal("daemonExtensionHelperCommand() returned an empty path")
	}

	if got := daemonExtensionHelperArgs(); !testutil.EqualStringSlices(
		got,
		[]string{"-test.run=TestDaemonExtensionHelperProcess"},
	) {
		t.Fatalf("daemonExtensionHelperArgs() = %#v, want helper test selector", got)
	}

	env := daemonExtensionHelperEnv("/tmp/daemon-helper-marker")
	if env[daemonExtensionHelperEnvKey] != "1" {
		t.Fatalf("daemonExtensionHelperEnv() helper flag = %q, want 1", env[daemonExtensionHelperEnvKey])
	}
	if env[daemonExtensionHelperMarkerKey] != "/tmp/daemon-helper-marker" {
		t.Fatalf(
			"daemonExtensionHelperEnv() marker = %q, want /tmp/daemon-helper-marker",
			env[daemonExtensionHelperMarkerKey],
		)
	}

	withoutMarker := daemonExtensionHelperEnv("")
	if _, ok := withoutMarker[daemonExtensionHelperMarkerKey]; ok {
		t.Fatalf("daemonExtensionHelperEnv(\"\") unexpectedly set %q", daemonExtensionHelperMarkerKey)
	}

	withScenario := daemonExtensionHelperScenarioEnv("record_initialize", "/tmp/daemon-helper-scenario")
	if got := withScenario[daemonExtensionHelperScenarioKey]; got != "record_initialize" {
		t.Fatalf("daemonExtensionHelperScenarioEnv() scenario = %q, want record_initialize", got)
	}
}

func daemonExtensionHelperCommand(t *testing.T) string {
	t.Helper()

	command, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	return command
}

func daemonExtensionHelperArgs() []string {
	return []string{"-test.run=TestDaemonExtensionHelperProcess"}
}

func daemonExtensionHelperEnv(markerPath string) map[string]string {
	env := map[string]string{
		daemonExtensionHelperEnvKey: "1",
	}
	if strings.TrimSpace(markerPath) != "" {
		env[daemonExtensionHelperMarkerKey] = markerPath
	}
	return env
}

func daemonExtensionHelperScenarioEnv(scenario string, markerPath string) map[string]string {
	env := daemonExtensionHelperEnv(markerPath)
	if strings.TrimSpace(scenario) != "" {
		env[daemonExtensionHelperScenarioKey] = scenario
	}
	return env
}

func TestDaemonExtensionHelperShutdownAppendsMarkerLine(t *testing.T) {
	t.Parallel()

	marker := filepath.Join(t.TempDir(), "helper-marker.jsonl")
	if err := appendMarkerLine(marker, `{"event":"initialize"}`); err != nil {
		t.Fatalf("appendMarkerLine(initialize) error = %v", err)
	}
	if err := appendMarkerLine(marker, `{"event":"delivery"}`); err != nil {
		t.Fatalf("appendMarkerLine(delivery) error = %v", err)
	}

	server := newDaemonExtensionHelperServer("", marker)
	server.encoder = json.NewEncoder(io.Discard)

	exit, err := server.handleRequest(daemonExtensionHelperRequest{ID: "1", Method: "shutdown"})
	if err != nil {
		t.Fatalf("handleRequest(shutdown) error = %v", err)
	}
	if exit {
		t.Fatal("handleRequest(shutdown) exit = true, want false")
	}

	payload, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("os.ReadFile(marker) error = %v", readErr)
	}
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	if got, want := len(lines), 3; got != want {
		t.Fatalf("marker line count = %d, want %d; payload=%q", got, want, string(payload))
	}
	if got, want := lines[2], "shutdown"; got != want {
		t.Fatalf("marker final line = %q, want %q", got, want)
	}
}

func TestDaemonExtensionHelperHandleRequest(t *testing.T) {
	t.Run("ShouldRejectInvalidDeliveryRequestsBeforeRecordingOrAcking", func(t *testing.T) {
		t.Parallel()

		marker := filepath.Join(t.TempDir(), "helper-marker.jsonl")
		var output bytes.Buffer

		server := newDaemonExtensionHelperServer("", marker)
		server.encoder = json.NewEncoder(&output)

		params, err := json.Marshal(bridgepkg.DeliveryRequest{
			Event: bridgepkg.DeliveryEvent{
				DeliveryID:       "delivery-1",
				BridgeInstanceID: "brg-1",
				RoutingKey: bridgepkg.RoutingKey{
					Scope:            bridgepkg.ScopeGlobal,
					BridgeInstanceID: "brg-1",
					PeerID:           "peer-1",
				},
				DeliveryTarget: bridgepkg.DeliveryTarget{
					BridgeInstanceID: "brg-1",
					PeerID:           "peer-1",
					Mode:             bridgepkg.DeliveryModeDirectSend,
				},
				Seq:       1,
				EventType: bridgepkg.DeliveryEventTypeResume,
			},
		})
		if err != nil {
			t.Fatalf("json.Marshal(delivery request) error = %v", err)
		}

		exit, err := server.handleRequest(daemonExtensionHelperRequest{
			ID:     "1",
			Method: "bridges/deliver",
			Params: params,
		})
		if exit {
			t.Fatal("handleRequest(bridges/deliver) exit = true, want false")
		}
		if err == nil {
			t.Fatal("handleRequest(bridges/deliver) error = nil, want delivery validation failure")
		}
		if !strings.Contains(err.Error(), "validate bridges/deliver request") {
			t.Fatalf("handleRequest(bridges/deliver) error = %q, want validation context", err)
		}

		payload, readErr := os.ReadFile(marker)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			t.Fatalf("os.ReadFile(marker) error = %v", readErr)
		}
		if strings.TrimSpace(string(payload)) != "" {
			t.Fatalf("marker payload = %q, want no recorded delivery", string(payload))
		}
		if strings.TrimSpace(output.String()) != "" {
			t.Fatalf("helper output = %q, want no ACK payload", output.String())
		}
	})
}

func TestDaemonExtensionHelperMarkerRecording(t *testing.T) {
	t.Run("ShouldWrapInitializeMarkerFailuresWithOperationContext", func(t *testing.T) {
		t.Parallel()

		marker := filepath.Join(t.TempDir(), "marker-dir")
		if err := os.Mkdir(marker, 0o755); err != nil {
			t.Fatalf("os.Mkdir(marker) error = %v", err)
		}

		server := newDaemonExtensionHelperServer("", marker)
		err := server.recordInitialize(subprocess.InitializeRequest{}, subprocess.InitializeResponse{})
		if err == nil {
			t.Fatal("recordInitialize() error = nil, want marker append failure")
		}
		if !strings.Contains(err.Error(), "record initialize marker") {
			t.Fatalf("recordInitialize() error = %q, want initialize context", err)
		}
		if !strings.Contains(err.Error(), "append marker line") {
			t.Fatalf("recordInitialize() error = %q, want append context", err)
		}
	})

	t.Run("ShouldWrapDeliveryMarkerFailuresWithOperationContext", func(t *testing.T) {
		t.Parallel()

		marker := filepath.Join(t.TempDir(), "marker-dir")
		if err := os.Mkdir(marker, 0o755); err != nil {
			t.Fatalf("os.Mkdir(marker) error = %v", err)
		}

		server := newDaemonExtensionHelperServer("", marker)
		err := server.recordDelivery(bridgepkg.DeliveryRequest{})
		if err == nil {
			t.Fatal("recordDelivery() error = nil, want marker append failure")
		}
		if !strings.Contains(err.Error(), "record delivery marker") {
			t.Fatalf("recordDelivery() error = %q, want delivery context", err)
		}
		if !strings.Contains(err.Error(), "append marker line") {
			t.Fatalf("recordDelivery() error = %q, want append context", err)
		}
	})
}

type daemonExtensionHelperServer struct {
	scenario                string
	marker                  string
	scanner                 *bufio.Scanner
	encoder                 *json.Encoder
	slowDeliveryRelease     chan struct{}
	slowDeliveryReleaseOnce sync.Once
	slowDeliveryWG          sync.WaitGroup
	mu                      sync.Mutex
}

type daemonExtensionHelperRequest struct {
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

func newDaemonExtensionHelperServer(scenario string, marker string) *daemonExtensionHelperServer {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)

	return &daemonExtensionHelperServer{
		scenario:            scenario,
		marker:              marker,
		scanner:             scanner,
		encoder:             encoder,
		slowDeliveryRelease: make(chan struct{}),
	}
}

func (h *daemonExtensionHelperServer) run() int {
	for h.scanner.Scan() {
		var req daemonExtensionHelperRequest
		if err := json.Unmarshal(h.scanner.Bytes(), &req); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		exitProcess, err := h.handleRequest(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if exitProcess {
			return 1
		}
	}
	if err := h.scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func (h *daemonExtensionHelperServer) handleRequest(req daemonExtensionHelperRequest) (bool, error) {
	switch req.Method {
	case "initialize":
		var params subprocess.InitializeRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return false, err
		}
		response := daemonExtensionInitializeResponse(params)
		if err := h.sendResult(req.ID, response); err != nil {
			return false, err
		}
		if h.scenario == "record_initialize" || h.scenario == "auto_exit_record_initialize" {
			if err := h.recordInitialize(params, response); err != nil {
				return false, err
			}
		}
		return h.scenario == "auto_exit_record_initialize", nil
	case "health_check":
		return false, h.sendResult(req.ID, subprocess.HealthCheckResponse{Healthy: true})
	case "bridges/deliver":
		var params bridgepkg.DeliveryRequest
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return false, fmt.Errorf("decode bridges/deliver request: %w", err)
		}
		if err := params.Validate(); err != nil {
			return false, fmt.Errorf("validate bridges/deliver request: %w", err)
		}
		if err := h.recordDelivery(params); err != nil {
			return false, err
		}

		ack := bridgepkg.DeliveryAck{
			DeliveryID: strings.TrimSpace(params.Event.DeliveryID),
			Seq:        params.Event.Seq,
		}
		if ack.Seq > 0 {
			ack.RemoteMessageID = fmt.Sprintf("remote-%d", ack.Seq)
		}
		if ack.Seq > 1 {
			ack.ReplaceRemoteMessageID = fmt.Sprintf("remote-%d", ack.Seq-1)
		}
		switch h.scenario {
		case "slow_record_deliveries":
			h.sendDelayedDeliveryResult(req.ID, ack)
			return false, nil
		case "exit_once_record_deliveries":
			if markerLineCount(h.marker) == 1 {
				return true, nil
			}
		}
		return false, h.sendResult(req.ID, ack)
	case "shutdown":
		h.releaseSlowDeliveries()
		h.waitSlowDeliveries()
		if strings.TrimSpace(h.marker) != "" {
			if err := appendMarkerLine(h.marker, "shutdown"); err != nil {
				return false, err
			}
		}
		return false, h.sendResult(req.ID, subprocess.ShutdownResponse{Acknowledged: true})
	default:
		return false, h.sendResult(req.ID, map[string]any{})
	}
}

func (h *daemonExtensionHelperServer) sendDelayedDeliveryResult(id any, ack bridgepkg.DeliveryAck) {
	h.slowDeliveryWG.Go(func() {
		<-h.slowDeliveryRelease
		if err := h.sendResult(id, ack); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	})
}

func (h *daemonExtensionHelperServer) releaseSlowDeliveries() {
	h.slowDeliveryReleaseOnce.Do(func() {
		close(h.slowDeliveryRelease)
	})
}

func (h *daemonExtensionHelperServer) waitSlowDeliveries() {
	h.slowDeliveryWG.Wait()
}

func (h *daemonExtensionHelperServer) sendResult(id any, result any) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func (h *daemonExtensionHelperServer) recordInitialize(
	request subprocess.InitializeRequest,
	response subprocess.InitializeResponse,
) error {
	if strings.TrimSpace(h.marker) == "" {
		return nil
	}

	payload, err := json.Marshal(daemonInitializeMarker{
		Request:  request,
		Response: response,
	})
	if err != nil {
		return fmt.Errorf("record initialize marker: marshal payload: %w", err)
	}
	if err := appendMarkerLine(h.marker, string(payload)); err != nil {
		return fmt.Errorf("record initialize marker: %w", err)
	}
	return nil
}

func (h *daemonExtensionHelperServer) recordDelivery(request bridgepkg.DeliveryRequest) error {
	if strings.TrimSpace(h.marker) == "" {
		return nil
	}

	payload, err := json.Marshal(daemonDeliveryMarker{
		PID:     os.Getpid(),
		Request: request,
	})
	if err != nil {
		return fmt.Errorf("record delivery marker: marshal payload: %w", err)
	}
	if err := appendMarkerLine(h.marker, string(payload)); err != nil {
		return fmt.Errorf("record delivery marker: %w", err)
	}
	return nil
}

func daemonExtensionInitializeResponse(req subprocess.InitializeRequest) subprocess.InitializeResponse {
	implementedMethods := []string{"health_check", "shutdown"}
	implementedMethods = append(
		implementedMethods,
		extensionprotocol.CapabilityServiceMethods(req.Capabilities.Provides)...)

	return subprocess.InitializeResponse{
		ProtocolVersion: req.ProtocolVersion,
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    req.Extension.Name,
			Version: req.Extension.Version,
		},
		AcceptedCapabilities: subprocess.AcceptedCapabilities{
			Provides: append([]string(nil), req.Capabilities.Provides...),
			Actions:  append([]extensionprotocol.HostAPIMethod(nil), req.Capabilities.GrantedActions...),
			Security: append([]string(nil), req.Capabilities.GrantedSecurity...),
		},
		ImplementedMethods:  implementedMethods,
		SupportedHookEvents: []string{string(hookspkg.HookSessionPostCreate)},
		Supports: subprocess.InitializeSupports{
			HealthCheck: true,
		},
	}
}

type daemonInitializeMarker struct {
	Request  subprocess.InitializeRequest  `json:"request"`
	Response subprocess.InitializeResponse `json:"response"`
}

type daemonDeliveryMarker struct {
	PID     int                       `json:"pid"`
	Request bridgepkg.DeliveryRequest `json:"request"`
}

func appendMarkerLine(path string, line string) (err error) {
	target := strings.TrimSpace(path)
	if target == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("append marker line: create marker directory: %w", err)
	}
	file, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("append marker line: open marker file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("append marker line: close marker file: %w", closeErr)
		}
	}()
	_, err = fmt.Fprintf(file, "%s\n", strings.TrimSpace(line))
	if err != nil {
		return fmt.Errorf("append marker line: write marker file: %w", err)
	}
	return nil
}

func markerLineCount(path string) int {
	payload, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		// The helper treats missing or unreadable markers as an empty state file.
		return 0
	}
	count := 0
	for line := range strings.SplitSeq(string(payload), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		count++
	}
	return count
}
