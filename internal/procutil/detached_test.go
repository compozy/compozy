package procutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const detachedSleepHelperEnv = "COMPOZY_PROCUTIL_DETACHED_SLEEP_HELPER"

func TestDetachedSleepHelperProcess(t *testing.T) {
	t.Run("Should sleep until the parent cleanup path terminates it", func(t *testing.T) {
		if os.Getenv(detachedSleepHelperEnv) != "1" {
			t.Skip("helper process only")
		}
		time.Sleep(10 * time.Second)
	})
}

// not parallel: mutates the detached-process wait hook.
func TestNewDetachedProcessStartsReaperEagerly(t *testing.T) {
	t.Run("Should start the reaper before any caller observes the process", func(t *testing.T) {
		originalWait := waitDetachedProcess
		waitStarted := make(chan *os.Process, 1)
		releaseWait := make(chan struct{})
		waitDetachedProcess = func(process *os.Process) (*os.ProcessState, error) {
			waitStarted <- process
			<-releaseWait
			return nil, nil
		}

		process := &os.Process{Pid: 42}
		detached := newDetachedProcess(process, "", 0)
		t.Cleanup(func() {
			close(releaseWait)
			if err := detached.Wait(); err != nil {
				t.Errorf("DetachedProcess.Wait() cleanup error = %v", err)
			}
			waitDetachedProcess = originalWait
		})

		waitCtx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		select {
		case got := <-waitStarted:
			if got != process {
				t.Fatalf("wait process = %p, want %p", got, process)
			}
		case <-waitCtx.Done():
			t.Fatalf("detached reaper did not start before observation: %v", waitCtx.Err())
		}
	})
}

// not parallel: mutates detached launch hooks shared by SpawnDetachedLoggedProcess.
func TestSpawnDetachedLoggedProcessLifecycle(t *testing.T) {
	t.Run("Should reject a nil context before starting a child", func(t *testing.T) {
		oldStart := startDetachedProcess
		startCalled := false
		startDetachedProcess = func(string, []string, *os.ProcAttr) (*os.Process, error) {
			startCalled = true
			return nil, errors.New("unexpected process start")
		}
		t.Cleanup(func() { startDetachedProcess = oldStart })

		var ctx context.Context
		process, err := SpawnDetachedLoggedProcess(ctx, DetachedLaunchRequest{
			Binary:  os.Args[0],
			LogPath: filepath.Join(t.TempDir(), "detached.log"),
		})
		if err == nil {
			t.Fatal("SpawnDetachedLoggedProcess(nil) error = nil, want context validation failure")
		}
		if process != nil {
			t.Fatalf("SpawnDetachedLoggedProcess(nil) process = %#v, want nil", process)
		}
		if startCalled {
			t.Fatal("process start called after rejected nil context")
		}
	})

	t.Run("Should terminate and reap child when a parent handle close fails after start", func(t *testing.T) {
		closeErr := errors.New("forced post-start close failure")
		oldStart := startDetachedProcess
		oldClose := closeDetachedLaunchFile
		var started *os.Process
		forcedClose := false
		startDetachedProcess = func(name string, argv []string, attr *os.ProcAttr) (*os.Process, error) {
			process, err := oldStart(name, argv, attr)
			if process != nil {
				started = process
			}
			return process, err
		}
		closeDetachedLaunchFile = func(file *os.File) error {
			err := oldClose(file)
			if file != nil && file.Name() == os.DevNull && !forcedClose {
				forcedClose = true
				return errors.Join(err, closeErr)
			}
			return err
		}
		t.Cleanup(func() {
			startDetachedProcess = oldStart
			closeDetachedLaunchFile = oldClose
		})

		process, err := SpawnDetachedLoggedProcess(context.Background(), DetachedLaunchRequest{
			Binary:  os.Args[0],
			Args:    []string{"-test.run=^TestDetachedSleepHelperProcess$"},
			Sandbox: []string{detachedSleepHelperEnv + "=1"},
			LogPath: filepath.Join(t.TempDir(), "detached.log"),
		})
		if err == nil {
			if process != nil {
				if waitErr := process.Wait(); waitErr != nil {
					t.Fatalf("Wait(unexpected process) error = %v", waitErr)
				}
			}
			t.Fatal("SpawnDetachedLoggedProcess() error = nil, want post-start close failure")
		}
		if process != nil {
			t.Fatalf("SpawnDetachedLoggedProcess() process = %#v, want nil on cleaned-up failure", process)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("SpawnDetachedLoggedProcess() error = %v, want wrapped close error", err)
		}
		if !forcedClose {
			t.Fatal("close hook was not forced, want post-start close failure")
		}
		if started == nil || started.Pid <= 0 {
			t.Fatalf("started process = %#v, want captured child pid", started)
		}
		assertEventuallyNotAlive(t, started.Pid)
	})
}

func assertEventuallyNotAlive(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for Alive(pid) {
		if time.Now().After(deadline) {
			t.Fatalf("Alive(%d) = true after cleanup deadline, want child reaped", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
