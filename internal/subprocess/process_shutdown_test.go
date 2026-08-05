package subprocess

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/toolruntime"
)

func TestProcessShutdownCancellationContract(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == subprocessWindowsKey {
		t.Skip("shutdown signal escalation semantics are unix-only")
	}

	t.Run("Should terminate descendants after the root exits naturally", func(t *testing.T) {
		t.Parallel()

		fixtureDir := t.TempDir()
		childPIDPath := filepath.Join(fixtureDir, "child.pid")
		releasePath := filepath.Join(fixtureDir, "release")
		process, err := Launch(testContext(t), LaunchConfig{
			Command: "sh",
			Args: []string{
				"-c",
				`sleep 30 </dev/null >/dev/null 2>&1 & child=$!; printf '%s\n' "$child" > "$1"; ` +
					`while [ ! -e "$2" ]; do sleep 0.05; done`,
				"sh",
				childPIDPath,
				releasePath,
			},
			DisableTransport: true,
		})
		if err != nil {
			t.Fatalf("Launch(natural exit wrapper) error = %v", err)
		}
		cleanupProcessShutdownContract(t, process)

		childPID := 0
		waitForCondition(t, time.Second, func() bool {
			payload, readErr := os.ReadFile(childPIDPath)
			if readErr != nil {
				return false
			}
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(string(payload)))
			if parseErr != nil || parsed <= 0 {
				return false
			}
			childPID = parsed
			return procutil.Alive(childPID)
		})
		t.Cleanup(func() {
			if childPID <= 0 || !procutil.Alive(childPID) {
				return
			}
			if err := procutil.Signal(childPID, syscall.SIGKILL); err != nil {
				t.Errorf("cleanup child process %d error = %v", childPID, err)
			}
		})
		if err := os.WriteFile(releasePath, []byte("release"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(release root process) error = %v", err)
		}

		select {
		case <-process.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("process completion did not join natural process-tree cleanup")
		}
		if err := process.Wait(); err != nil {
			t.Fatalf("Wait() after natural root exit error = %v", err)
		}
		waitForCondition(t, time.Second, func() bool {
			return !procutil.Alive(childPID)
		})
	})

	t.Run("Should accept transport closure after a cooperative shutdown request", func(t *testing.T) {
		t.Parallel()

		process := launchHelperProcess(t, "shutdown_exit_no_ack", LaunchConfig{})
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     250,
			DefaultHookTimeoutMS:  100,
		})

		if err := process.Shutdown(testContext(t)); err != nil {
			t.Fatalf("Shutdown(transport closed before response) error = %v, want nil", err)
		}
	})

	t.Run("Should not start shutdown when caller context is already canceled", func(t *testing.T) {
		t.Parallel()

		markerPath := filepath.Join(t.TempDir(), "shutdown.marker")
		process := launchHelperProcess(t, "shutdown_hang", LaunchConfig{
			ShutdownTimeout: 50 * time.Millisecond,
			PostSignalGrace: 25 * time.Millisecond,
		}, testShutdownMarkerEnv+"="+markerPath)
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     50,
			DefaultHookTimeoutMS:  100,
		})

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := process.Shutdown(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Shutdown(already canceled) error = %v, want context.Canceled", err)
		}
		if _, statErr := os.Stat(markerPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v, want shutdown marker absent", markerPath, statErr)
		}
		select {
		case <-process.Done():
			t.Fatal("process exited after already-canceled Shutdown(), want caller to retain ownership")
		default:
		}
	})

	t.Run("Should escalate shutdown after cancellation once stop is requested", func(t *testing.T) {
		t.Parallel()

		markerPath := filepath.Join(t.TempDir(), "shutdown.marker")
		process := launchHelperProcess(t, "shutdown_hang", LaunchConfig{
			ShutdownTimeout: 50 * time.Millisecond,
			PostSignalGrace: 25 * time.Millisecond,
		}, testShutdownMarkerEnv+"="+markerPath)
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     50,
			DefaultHookTimeoutMS:  100,
		})

		ctx, cancel := context.WithCancel(context.Background())
		errCh := make(chan error, 1)
		go func() {
			errCh <- process.Shutdown(ctx)
		}()

		waitForCondition(t, time.Second, func() bool {
			_, err := os.Stat(markerPath)
			return err == nil
		})
		cancel()

		select {
		case err := <-errCh:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Shutdown(canceled after request) error = %v, want context.Canceled", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown(canceled after request) timed out")
		}

		select {
		case <-process.Done():
		case <-time.After(250 * time.Millisecond):
			t.Fatal("Shutdown returned before process exited after cancellation")
		}
	})

	t.Run("Should share one shutdown operation across concurrent callers", func(t *testing.T) {
		t.Parallel()

		markerPath := filepath.Join(t.TempDir(), "shutdown.marker")
		process := launchHelperProcess(t, "shutdown_hang", LaunchConfig{
			ShutdownTimeout: 25 * time.Millisecond,
			PostSignalGrace: 25 * time.Millisecond,
		}, testShutdownMarkerEnv+"="+markerPath)
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     25,
			DefaultHookTimeoutMS:  100,
		})

		const callers = 8
		start := make(chan struct{})
		errs := make(chan error, callers)
		for range callers {
			go func() {
				<-start
				errs <- process.Shutdown(testContext(t))
			}()
		}
		close(start)
		for range callers {
			if err := <-errs; err != nil {
				t.Fatalf("Shutdown(concurrent) error = %v", err)
			}
		}
		payload, err := os.ReadFile(markerPath)
		if err != nil {
			t.Fatalf("os.ReadFile(shutdown marker) error = %v", err)
		}
		if got, want := strings.Count(string(payload), "shutdown\n"), 1; got != want {
			t.Fatalf("shutdown request count = %d, want %d; payload=%q", got, want, payload)
		}
	})

	t.Run("Should start a fresh attempt after a failed shutdown", func(t *testing.T) {
		t.Parallel()

		var frames bytes.Buffer
		process := newStalledShutdownProcess(t, &frames)

		firstErr := process.Shutdown(t.Context())
		if !errors.Is(firstErr, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(stalled process) error = %v, want context.DeadlineExceeded", firstErr)
		}
		secondErr := process.Shutdown(t.Context())
		if !errors.Is(secondErr, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(retry after failure) error = %v, want context.DeadlineExceeded", secondErr)
		}
		if got, want := strings.Count(frames.String(), `"method":"shutdown"`), 2; got != want {
			t.Fatalf("cooperative shutdown request count = %d, want %d; frames=%q", got, want, frames.String())
		}
	})

	t.Run("Should bound process record completion before publishing Done", func(t *testing.T) {
		t.Parallel()

		store := newBlockingCompletionProcessStore()
		registry := toolruntime.NewRegistry(store)
		process := launchHelperProcess(t, "shutdown_exit_no_ack", LaunchConfig{
			ShutdownTimeout: 20 * time.Millisecond,
			PostSignalGrace: 20 * time.Millisecond,
			ProcessRegistry: registry,
			ProcessRecord: toolruntime.RegisterConfig{
				ID: "process-blocked-completion",
			},
		})
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     20,
			DefaultHookTimeoutMS:  100,
		})

		err := process.Shutdown(testContext(t))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Shutdown(blocked completion) error = %v, want deadline exceeded", err)
		}
		select {
		case <-store.completionStarted:
		case <-time.After(time.Second):
			t.Fatal("process record completion did not start")
		}
		select {
		case <-process.Done():
		case <-time.After(time.Second):
			t.Fatal("process Done remained blocked by process record persistence")
		}
	})

	t.Run("Should treat cooperative timeout as escalation when caller context remains active", func(t *testing.T) {
		t.Parallel()

		process := launchHelperProcess(t, "shutdown_delayed_ack", LaunchConfig{
			ShutdownTimeout: 20 * time.Millisecond,
			PostSignalGrace: 25 * time.Millisecond,
		})
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     20,
			DefaultHookTimeoutMS:  100,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		startedAt := time.Now()
		if err := process.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown(cooperative timeout) error = %v, want nil", err)
		}
		if elapsed := time.Since(startedAt); elapsed < 20*time.Millisecond {
			t.Fatalf("Shutdown(cooperative timeout) elapsed = %v, want wait through shutdown timeout", elapsed)
		}
	})

	t.Run("Should wait for active inbound handlers before reporting process completion", func(t *testing.T) {
		t.Parallel()

		process := launchHelperProcess(t, "default", LaunchConfig{})
		cleanupProcessShutdownContract(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     250,
			DefaultHookTimeoutMS:  100,
		})

		handlerStarted := make(chan struct{})
		handlerCanceled := make(chan struct{})
		releaseHandler := make(chan struct{})
		releaseOnce := sync.OnceFunc(func() { close(releaseHandler) })
		t.Cleanup(releaseOnce)
		if err := process.HandleMethod(
			"host/block",
			func(ctx context.Context, _ json.RawMessage) (any, error) {
				close(handlerStarted)
				<-ctx.Done()
				close(handlerCanceled)
				<-releaseHandler
				return nil, ctx.Err()
			},
		); err != nil {
			t.Fatalf("HandleMethod(host/block) error = %v", err)
		}

		callErr := make(chan error, 1)
		go func() {
			var response json.RawMessage
			callErr <- process.Call(testContext(t), "relay_to_host", map[string]any{
				"method": "host/block",
				"params": map[string]string{"value": "blocked"},
			}, &response)
		}()
		select {
		case <-handlerStarted:
		case <-time.After(time.Second):
			t.Fatal("inbound handler did not start")
		}

		shutdownErr := make(chan error, 1)
		go func() { shutdownErr <- process.Shutdown(testContext(t)) }()
		select {
		case <-handlerCanceled:
		case <-time.After(2 * time.Second):
			t.Fatal("process shutdown did not cancel inbound handler")
		}
		select {
		case <-process.Done():
			t.Fatal("process completion was reported before inbound handler returned")
		default:
		}

		releaseOnce()
		select {
		case err := <-shutdownErr:
			if err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown() did not return after inbound handler settled")
		}
		select {
		case <-callErr:
		case <-time.After(time.Second):
			t.Fatal("relay Call() did not settle after shutdown")
		}
	})
}

type blockingCompletionProcessStore struct {
	base              *toolruntime.MemoryStore
	completionStarted chan struct{}
	startOnce         sync.Once
}

func newBlockingCompletionProcessStore() *blockingCompletionProcessStore {
	return &blockingCompletionProcessStore{
		base:              toolruntime.NewMemoryStore(),
		completionStarted: make(chan struct{}),
	}
}

func (s *blockingCompletionProcessStore) UpsertProcessRecord(
	ctx context.Context,
	record toolruntime.ProcessRecord,
) error {
	return s.base.UpsertProcessRecord(ctx, record)
}

func (s *blockingCompletionProcessStore) UpdateProcessRecordState(
	ctx context.Context,
	update toolruntime.ProcessStateUpdate,
) error {
	if update.State == toolruntime.ProcessStateCompleted || update.State == toolruntime.ProcessStateFailed {
		s.startOnce.Do(func() { close(s.completionStarted) })
		<-ctx.Done()
		return ctx.Err()
	}
	return s.base.UpdateProcessRecordState(ctx, update)
}

func (s *blockingCompletionProcessStore) ListProcessRecords(
	ctx context.Context,
	query toolruntime.ProcessQuery,
) ([]toolruntime.ProcessRecord, error) {
	return s.base.ListProcessRecords(ctx, query)
}

// The nil command and never-closed done channel model a process group that never drains.
func newStalledShutdownProcess(t *testing.T, frames io.Writer) *Process {
	t.Helper()

	lifecycleCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	process := &Process{
		stdin:           discardWriteCloser{Writer: frames},
		lifecycleCtx:    lifecycleCtx,
		cancelLifecycle: cancel,
		done:            make(chan struct{}),
		state:           processStateReady,
		shutdownTimeout: 20 * time.Millisecond,
		postSignalGrace: 10 * time.Millisecond,
	}
	process.transport = newTransport(process, defaultMaxMessageBytes)
	return process
}

func cleanupProcessShutdownContract(t *testing.T, process *Process) {
	t.Helper()

	t.Cleanup(func() {
		if process == nil {
			return
		}
		select {
		case <-process.Done():
			return
		default:
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := process.Shutdown(ctx); err != nil {
			t.Logf("cleanup Shutdown() error = %v", err)
		}
	})
}
