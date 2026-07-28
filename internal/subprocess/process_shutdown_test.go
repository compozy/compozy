package subprocess

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestProcessShutdownCancellationContract(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("shutdown signal escalation semantics are unix-only")
	}

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
