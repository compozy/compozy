package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestStartRemovesStaleArtifactsAndMarksReady(t *testing.T) {
	paths := mustHomePaths(t)
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	staleInfo := Info{
		PID:        999001,
		Version:    "old",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(10, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := WriteInfo(paths.InfoPath, staleInfo); err != nil {
		t.Fatalf("WriteInfo(stale) error = %v", err)
	}
	if err := os.WriteFile(paths.SocketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale socket marker: %v", err)
	}
	if err := os.WriteFile(paths.LockPath, []byte("999001\n"), 0o600); err != nil {
		t.Fatalf("write stale lock file: %v", err)
	}

	startedAt := time.Unix(20, 0).UTC()
	result, err := Start(context.Background(), StartOptions{
		HomePaths:    paths,
		PID:          4242,
		Version:      "test-version",
		Now:          func() time.Time { return startedAt },
		ProcessAlive: func(pid int) bool { return pid == 4242 },
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	closeHostOnCleanup(t, result.Host)

	if result.Outcome != StartOutcomeStarted {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, StartOutcomeStarted)
	}
	if result.Info.PID != 4242 {
		t.Fatalf("Info.PID = %d, want 4242", result.Info.PID)
	}
	if result.Info.State != ReadyStateReady {
		t.Fatalf("Info.State = %q, want %q", result.Info.State, ReadyStateReady)
	}

	if _, err := os.Stat(paths.SocketPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale socket to be removed, stat err = %v", err)
	}

	currentInfo, err := ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if currentInfo.PID != 4242 {
		t.Fatalf("current info pid = %d, want 4242", currentInfo.PID)
	}
	if currentInfo.State != ReadyStateReady {
		t.Fatalf("current info state = %q, want %q", currentInfo.State, ReadyStateReady)
	}
	lockPID, err := readLockPID(paths.LockPath)
	if err != nil {
		t.Fatalf("readLockPID() error = %v", err)
	}
	if lockPID != 4242 {
		t.Fatalf("lock pid = %d, want 4242", lockPID)
	}
}

func TestStartReturnsAlreadyRunningWhenHealthyDaemonExists(t *testing.T) {
	paths := mustHomePaths(t)
	healthyInfo := Info{
		PID:        8181,
		Version:    "existing",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(30, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := WriteInfo(paths.InfoPath, healthyInfo); err != nil {
		t.Fatalf("WriteInfo(healthy) error = %v", err)
	}

	restore := stubAcquireDaemonLock(t, func(_ string, _ int, _ func(int) bool) (*Lock, error) {
		return nil, errAlreadyRunning{pid: healthyInfo.PID}
	})
	defer restore()

	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       9191,
		ProcessAlive: func(pid int) bool {
			return pid == healthyInfo.PID
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if result.Outcome != StartOutcomeAlreadyRunning {
		t.Fatalf("Outcome = %q, want %q", result.Outcome, StartOutcomeAlreadyRunning)
	}
	if result.Info.PID != healthyInfo.PID {
		t.Fatalf("Info.PID = %d, want %d", result.Info.PID, healthyInfo.PID)
	}

	currentInfo, err := ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if currentInfo.PID != healthyInfo.PID {
		t.Fatalf("current info pid = %d, want %d", currentInfo.PID, healthyInfo.PID)
	}
}

func TestStartDefaultsHTTPPortWhenUnset(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       5151,
		Version:   "default-http-port",
		Now: func() time.Time {
			return time.Unix(50, 0).UTC()
		},
		ProcessAlive: func(pid int) bool { return pid == 5151 },
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	closeHostOnCleanup(t, result.Host)

	if result.Info.HTTPPort != DefaultHTTPPort {
		t.Fatalf("Info.HTTPPort = %d, want %d", result.Info.HTTPPort, DefaultHTTPPort)
	}

	currentInfo, err := ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if currentInfo.HTTPPort != DefaultHTTPPort {
		t.Fatalf("currentInfo.HTTPPort = %d, want %d", currentInfo.HTTPPort, DefaultHTTPPort)
	}
}

func TestNormalizeStartOptionsUsesEphemeralHTTPPortWhenRequested(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	result, err := normalizeStartOptions(StartOptions{
		HomePaths: paths,
		HTTPPort:  EphemeralHTTPPort,
		PID:       5151,
	})
	if err != nil {
		t.Fatalf("normalizeStartOptions() error = %v", err)
	}
	if result.HTTPPort != 0 {
		t.Fatalf("result.HTTPPort = %d, want 0", result.HTTPPort)
	}
}

func TestQueryStatusReportsStoppedWhenInfoIsMissing(t *testing.T) {
	paths := mustHomePaths(t)

	status, err := QueryStatus(context.Background(), paths, ProbeOptions{})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if status.State != ReadyStateStopped {
		t.Fatalf("State = %q, want %q", status.State, ReadyStateStopped)
	}
	if status.Healthy {
		t.Fatal("Healthy = true, want false")
	}
	if status.Info != nil {
		t.Fatalf("Info = %#v, want nil", status.Info)
	}
}

func TestQueryStatusReportsStoppedWhenProcessIsDead(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	info := Info{
		PID:        5150,
		Version:    "test",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(40, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := WriteInfo(paths.InfoPath, info); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	status, err := QueryStatus(context.Background(), paths, ProbeOptions{
		ProcessAlive: func(int) bool { return false },
	})
	if err != nil {
		t.Fatalf("QueryStatus() error = %v", err)
	}
	if status.State != ReadyStateStopped {
		t.Fatalf("State = %q, want %q", status.State, ReadyStateStopped)
	}
	if status.Healthy {
		t.Fatal("Healthy = true, want false")
	}
	if status.Info == nil || status.Info.PID != info.PID {
		t.Fatalf("Info = %#v, want pid %d", status.Info, info.PID)
	}
}

func TestStartCleansUpAfterPrepareFailure(t *testing.T) {
	paths := mustHomePaths(t)

	restore := stubAcquireDaemonLock(t, func(path string, pid int, _ func(int) bool) (*Lock, error) {
		return &Lock{
			path: path,
			pid:  pid,
		}, nil
	})
	defer restore()

	prepareErr := errors.New("prepare failed")
	_, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       3131,
		Prepare: func(context.Context, *Host) error {
			return prepareErr
		},
		ProcessAlive: func(pid int) bool { return pid == 3131 },
	})
	if !errors.Is(err, prepareErr) {
		t.Fatalf("Start() error = %v, want %v", err, prepareErr)
	}
	if _, statErr := os.Stat(paths.InfoPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected info file to be removed after prepare failure, stat err = %v", statErr)
	}
	pid, readErr := readLockPID(paths.LockPath)
	if readErr != nil {
		t.Fatalf("readLockPID() error = %v", readErr)
	}
	if pid != 0 {
		t.Fatalf("readLockPID() = %d, want 0", pid)
	}
}

func TestStartReturnsLockErrorWhenHealthyInfoIsUnavailable(t *testing.T) {
	paths := mustHomePaths(t)

	restore := stubAcquireDaemonLock(t, func(_ string, _ int, _ func(int) bool) (*Lock, error) {
		return nil, errAlreadyRunning{pid: 8888}
	})
	defer restore()

	_, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       9999,
		ProcessAlive: func(int) bool {
			return false
		},
	})
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("Start() error = %v, want ErrAlreadyRunning", err)
	}
}

func TestStartReturnsHomeResolutionError(t *testing.T) {
	t.Parallel()

	homeErr := errors.New("home unavailable")
	restore := stubResolveDaemonHomePaths(t, func() (compozyconfig.HomePaths, error) {
		return compozyconfig.HomePaths{}, homeErr
	})
	defer restore()

	_, err := Start(context.Background(), StartOptions{})
	if !errors.Is(err, homeErr) {
		t.Fatalf("Start() error = %v, want %v", err, homeErr)
	}
}

func TestStartReturnsLayoutErrorWhenDaemonDirectoryCannotBeCreated(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	if err := os.MkdirAll(filepath.Dir(paths.DaemonDir), 0o755); err != nil {
		t.Fatalf("mkdir daemon parent: %v", err)
	}
	if err := os.WriteFile(paths.DaemonDir, []byte("not a dir"), 0o600); err != nil {
		t.Fatalf("write daemon dir file: %v", err)
	}

	_, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       4242,
	})
	if err == nil {
		t.Fatal("Start() error = nil, want non-nil")
	}
}

func TestStartRebuildsMismatchedRuntimeArtifacts(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	existingInfo := Info{
		PID:        1111,
		Version:    "old",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(90, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := WriteInfo(paths.InfoPath, existingInfo); err != nil {
		t.Fatalf("WriteInfo(existing) error = %v", err)
	}
	if err := os.WriteFile(paths.LockPath, []byte("2222\n"), 0o600); err != nil {
		t.Fatalf("write stale lock file: %v", err)
	}
	if err := os.WriteFile(paths.SocketPath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale socket marker: %v", err)
	}

	startedAt := time.Unix(100, 0).UTC()
	result, err := Start(context.Background(), StartOptions{
		HomePaths: paths,
		PID:       3333,
		Version:   "new",
		Now:       func() time.Time { return startedAt },
		ProcessAlive: func(pid int) bool {
			return pid == 3333
		},
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	closeHostOnCleanup(t, result.Host)

	currentInfo, err := ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if currentInfo.PID != 3333 {
		t.Fatalf("current info pid = %d, want 3333", currentInfo.PID)
	}
	if currentInfo.StartedAt != startedAt {
		t.Fatalf("current info started_at = %v, want %v", currentInfo.StartedAt, startedAt)
	}

	lockPID, err := readLockPID(paths.LockPath)
	if err != nil {
		t.Fatalf("readLockPID() error = %v", err)
	}
	if lockPID != 3333 {
		t.Fatalf("lock pid = %d, want 3333", lockPID)
	}

	if _, err := os.Stat(paths.SocketPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale socket to be removed, stat err = %v", err)
	}
}

func TestHostClosePreservesForeignInfo(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	foreignInfo := Info{
		PID:        9090,
		Version:    "foreign",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(50, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := WriteInfo(paths.InfoPath, foreignInfo); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	host := &Host{
		paths: paths,
		lock:  &Lock{path: paths.LockPath, pid: 8080},
		info: Info{
			PID:       8080,
			StartedAt: time.Unix(60, 0).UTC(),
			State:     ReadyStateReady,
		},
	}
	if err := host.Close(context.Background()); err != nil {
		t.Fatalf("host.Close() error = %v", err)
	}

	currentInfo, err := ReadInfo(paths.InfoPath)
	if err != nil {
		t.Fatalf("ReadInfo() error = %v", err)
	}
	if currentInfo.PID != foreignInfo.PID {
		t.Fatalf("foreign info pid = %d, want %d", currentInfo.PID, foreignInfo.PID)
	}
}

func TestCleanupStaleRuntimeRejectsHealthyExistingInfo(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	existingInfo := Info{
		PID:        1234,
		Version:    "healthy",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(70, 0).UTC(),
		State:      ReadyStateReady,
	}
	if err := WriteInfo(paths.InfoPath, existingInfo); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	host := &Host{
		paths:        paths,
		lock:         &Lock{path: paths.LockPath, pid: 5678},
		processAlive: func(pid int) bool { return pid == existingInfo.PID },
		healthy: func(_ context.Context, info Info) error {
			if info.PID == existingInfo.PID {
				return nil
			}
			return errors.New("unhealthy")
		},
	}
	if err := host.cleanupStaleRuntime(context.Background()); err == nil {
		t.Fatal("cleanupStaleRuntime() error = nil, want non-nil")
	}
}

func TestMarkReadyRejectsNilHost(t *testing.T) {
	t.Parallel()

	var host *Host
	if err := host.MarkReady(context.Background()); err == nil {
		t.Fatal("MarkReady() error = nil, want non-nil")
	}
}

func TestNilHostHelpersReturnZeroValues(t *testing.T) {
	t.Parallel()

	var host *Host
	if got := host.Paths(); got.HomeDir != "" {
		t.Fatalf("host.Paths().HomeDir = %q, want empty", got.HomeDir)
	}
	if got := host.Info(); got.PID != 0 {
		t.Fatalf("host.Info().PID = %d, want 0", got.PID)
	}
	if err := host.Close(context.Background()); err != nil {
		t.Fatalf("host.Close(nil) error = %v", err)
	}
}

func TestRemoveSocketPathRejectsDirectory(t *testing.T) {
	t.Parallel()

	socketDir := filepath.Join(t.TempDir(), "daemon.sock")
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		t.Fatalf("mkdir socket dir: %v", err)
	}

	if err := removeSocketPath(socketDir); err == nil {
		t.Fatal("removeSocketPath() error = nil, want non-nil")
	}
}

func TestCleanupStaleRuntimeRemovesCorruptInfo(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := os.WriteFile(paths.InfoPath, []byte("{"), 0o600); err != nil {
		t.Fatalf("write corrupt daemon info: %v", err)
	}

	host := &Host{
		paths:   paths,
		lock:    &Lock{path: paths.LockPath, pid: 1111},
		healthy: func(context.Context, Info) error { return nil },
	}
	if err := host.cleanupStaleRuntime(context.Background()); err != nil {
		t.Fatalf("cleanupStaleRuntime() error = %v", err)
	}
	if _, err := os.Stat(paths.InfoPath); !os.IsNotExist(err) {
		t.Fatalf("expected corrupt info to be removed, stat err = %v", err)
	}
}

func TestExistingHealthyDaemonInfoRejectsUnhealthyInfo(t *testing.T) {
	t.Parallel()

	paths := mustHomePaths(t)
	info := Info{
		PID:        7777,
		Version:    "test",
		SocketPath: paths.SocketPath,
		StartedAt:  time.Unix(80, 0).UTC(),
		State:      ReadyStateStarting,
	}
	if err := compozyconfig.EnsureHomeLayout(paths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := WriteInfo(paths.InfoPath, info); err != nil {
		t.Fatalf("WriteInfo() error = %v", err)
	}

	if _, ok := existingHealthyDaemonInfo(context.Background(), paths.InfoPath, func(context.Context, Info) error {
		return errors.New("not ready")
	}); ok {
		t.Fatal("existingHealthyDaemonInfo() ok = true, want false")
	}
}

func mustHomePaths(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), ".compozy"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	return paths
}

func closeHostOnCleanup(t *testing.T, host *Host) {
	t.Helper()

	t.Cleanup(func() {
		if err := host.Close(context.Background()); err != nil {
			t.Errorf("Host.Close() error = %v", err)
		}
	})
}

func stubAcquireDaemonLock(
	t *testing.T,
	fn func(path string, pid int, processAlive func(int) bool) (*Lock, error),
) func() {
	t.Helper()

	original := acquireDaemonLock
	acquireDaemonLock = fn
	t.Cleanup(func() {
		acquireDaemonLock = original
	})
	return func() {
		acquireDaemonLock = original
	}
}

func stubResolveDaemonHomePaths(
	t *testing.T,
	fn func() (compozyconfig.HomePaths, error),
) func() {
	t.Helper()

	original := resolveDaemonHomePaths
	resolveDaemonHomePaths = fn
	t.Cleanup(func() {
		resolveDaemonHomePaths = original
	})
	return func() {
		resolveDaemonHomePaths = original
	}
}
