package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/testutil"
)

type stubDaemonProcess struct {
	pid     int
	done    chan struct{}
	waitErr error
}

func (p *stubDaemonProcess) PID() int {
	if p.pid > 0 {
		return p.pid
	}
	return 42
}

func (p *stubDaemonProcess) Done() <-chan struct{} {
	return p.done
}

func (p *stubDaemonProcess) Wait() error {
	<-p.done
	return p.waitErr
}

func (p *stubDaemonProcess) Terminate() error {
	select {
	case <-p.done:
	default:
		p.complete(nil)
	}
	return nil
}

func (p *stubDaemonProcess) complete(err error) {
	p.waitErr = err
	close(p.done)
}

func TestPollingLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a missing caller context", func(t *testing.T) {
		t.Parallel()

		called := false
		var missingContext context.Context
		_, err := pollUntil(
			missingContext,
			time.Second,
			time.Millisecond,
			nil,
			"poll timed out",
			func(context.Context, pollEvent) (struct{}, bool, error) {
				called = true
				return struct{}{}, true, nil
			},
		)
		if err == nil || !strings.Contains(err.Error(), "polling context is required") {
			t.Fatalf("pollUntil() error = %v, want required-context error", err)
		}
		if called {
			t.Fatal("pollUntil() called the polling step without a caller context")
		}
	})

	t.Run("Should preserve caller cancellation before the first tick", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		called := false
		_, err := pollUntil(
			ctx,
			time.Second,
			time.Hour,
			nil,
			"poll timed out",
			func(context.Context, pollEvent) (struct{}, bool, error) {
				called = true
				return struct{}{}, true, nil
			},
		)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pollUntil() error = %v, want context.Canceled", err)
		}
		if called {
			t.Fatal("pollUntil() called the polling step before its first tick")
		}
	})

	t.Run("Should dispatch an explicit interrupt without waiting for a tick", func(t *testing.T) {
		t.Parallel()

		interrupt := make(chan struct{})
		close(interrupt)
		value, err := pollUntil(
			t.Context(),
			time.Second,
			time.Hour,
			interrupt,
			"poll timed out",
			func(_ context.Context, event pollEvent) (string, bool, error) {
				if event != pollEventInterrupt {
					t.Fatalf("pollUntil() event = %d, want interrupt", event)
				}
				return sessionPromptInterruptedKey, true, nil
			},
		)
		if err != nil {
			t.Fatalf("pollUntil() error = %v", err)
		}
		if value != sessionPromptInterruptedKey {
			t.Fatalf("pollUntil() value = %q, want interrupted", value)
		}
	})

	t.Run("Should consume one interrupt before the first tick and preserve callback errors", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			const interval = time.Hour
			errTick := errors.New("tick failed")
			interrupt := make(chan struct{})
			events := make(chan pollEvent, 3)
			result := make(chan error, 1)

			go func() {
				_, err := pollUntil(
					t.Context(),
					4*interval,
					interval,
					interrupt,
					"poll timed out",
					func(_ context.Context, event pollEvent) (struct{}, bool, error) {
						events <- event
						if event == pollEventInterrupt {
							return struct{}{}, false, nil
						}
						return struct{}{}, false, errTick
					},
				)
				result <- err
			}()

			synctest.Wait()
			if got := len(events); got != 0 {
				t.Fatalf("poll callbacks before interrupt or first interval = %d, want 0", got)
			}

			close(interrupt)
			synctest.Wait()
			if event := <-events; event != pollEventInterrupt {
				t.Fatalf("pollUntil() first event = %d, want interrupt", event)
			}
			if got := len(events); got != 0 {
				t.Fatalf("repeated callbacks from one interrupt = %d, want 0", got)
			}

			time.Sleep(interval)
			synctest.Wait()
			if event := <-events; event != pollEventTick {
				t.Fatalf("pollUntil() second event = %d, want tick", event)
			}
			if err := <-result; !errors.Is(err, errTick) {
				t.Fatalf("pollUntil() error = %v, want tick sentinel", err)
			}
			if got := len(events); got != 0 {
				t.Fatalf("poll callbacks after terminal tick = %d, want 0", got)
			}

			deadlineCtx, cancelDeadline := context.WithTimeout(t.Context(), interval)
			defer cancelDeadline()
			deadlineCalls := 0
			deadlineResult := make(chan error, 1)
			go func() {
				_, err := pollUntil(
					deadlineCtx,
					4*interval,
					2*interval,
					nil,
					"poll timed out",
					func(context.Context, pollEvent) (struct{}, bool, error) {
						deadlineCalls++
						return struct{}{}, false, nil
					},
				)
				deadlineResult <- err
			}()

			synctest.Wait()
			if deadlineCalls != 0 {
				t.Fatalf("deadline poll callbacks before first interval = %d, want 0", deadlineCalls)
			}
			time.Sleep(interval)
			synctest.Wait()
			if err := <-deadlineResult; !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline pollUntil() error = %v, want context.DeadlineExceeded", err)
			}
			time.Sleep(3 * interval)
			synctest.Wait()
			if deadlineCalls != 0 {
				t.Fatalf("deadline poll callbacks after shutdown = %d, want 0", deadlineCalls)
			}
		})
	})
}

func TestWaitForDaemonStartReturnsStatusWhenDaemonBecomesReady(t *testing.T) {
	t.Parallel()

	t.Run("Should return daemon status when daemon becomes ready", func(t *testing.T) {
		t.Parallel()

		child := &stubDaemonProcess{done: make(chan struct{})}
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{Status: daemonRunningStatus, PID: 42}, nil
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 100 * time.Millisecond

		status, err := waitForDaemonStart(testutil.Context(t), deps, child)
		child.complete(nil)
		if err != nil {
			t.Fatalf("waitForDaemonStart() error = %v", err)
		}
		if status.Status != daemonRunningStatus || status.PID != 42 {
			t.Fatalf("waitForDaemonStart() status = %#v, want running pid 42", status)
		}
	})
}

func TestWaitForDaemonStartReturnsDeadlineExceededWhenReadyTimeoutExpires(t *testing.T) {
	t.Parallel()

	t.Run("Should wrap deadline exceeded when daemon readiness times out", func(t *testing.T) {
		t.Parallel()

		child := &stubDaemonProcess{done: make(chan struct{})}
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, errors.New("daemon unavailable")
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 5 * time.Millisecond
		deps.processAlive = func(int) bool { return true }

		_, err := waitForDaemonStart(testutil.Context(t), deps, child)
		child.complete(nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("waitForDaemonStart() error = %v, want context.DeadlineExceeded", err)
		}
		if !strings.Contains(err.Error(), "daemon did not become ready before timeout") {
			t.Fatalf("waitForDaemonStart() error = %v, want readiness timeout context", err)
		}
	})
}

func TestWaitForDaemonStopReturnsStoppedStatusWhenProcessExits(t *testing.T) {
	t.Parallel()

	t.Run("Should return stopped status when process exits", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, errors.New("daemon unavailable")
			},
		})
		deps.pollInterval = time.Millisecond
		deps.stopTimeout = 100 * time.Millisecond
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{
				PID:       42,
				StartedAt: fixedTestNow,
			}, nil
		}

		aliveChecks := 0
		deps.processAlive = func(int) bool {
			aliveChecks++
			return aliveChecks < 2
		}

		runtime, err := loadRuntimeContext(deps)
		if err != nil {
			t.Fatalf("loadRuntimeContext() error = %v", err)
		}
		info := compozydaemon.Info{
			PID:       42,
			StartedAt: fixedTestNow,
		}

		status, err := waitForDaemonStop(testutil.Context(t), deps, runtime, info)
		if err != nil {
			t.Fatalf("waitForDaemonStop() error = %v", err)
		}
		if status.Status != "stopped" || status.PID != 42 {
			t.Fatalf("waitForDaemonStop() status = %#v, want stopped pid 42", status)
		}
	})
}

func TestWaitForDaemonStopClearsStaleNetworkSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should clear stale network snapshot when daemon stops", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, errors.New("daemon unavailable")
			},
		})
		deps.pollInterval = time.Millisecond
		deps.stopTimeout = 100 * time.Millisecond
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{
				PID:       42,
				StartedAt: fixedTestNow,
				Network: &compozydaemon.NetworkInfo{
					Enabled: true,
					Status:  "active",
				},
			}, nil
		}

		aliveChecks := 0
		deps.processAlive = func(int) bool {
			aliveChecks++
			return aliveChecks < 2
		}

		runtime, err := loadRuntimeContext(deps)
		if err != nil {
			t.Fatalf("loadRuntimeContext() error = %v", err)
		}
		info := compozydaemon.Info{
			PID:       42,
			StartedAt: fixedTestNow,
			Network: &compozydaemon.NetworkInfo{
				Enabled: true,
				Status:  "active",
			},
		}

		status, err := waitForDaemonStop(testutil.Context(t), deps, runtime, info)
		if err != nil {
			t.Fatalf("waitForDaemonStop() error = %v", err)
		}
		if status.Status != "stopped" || status.PID != 42 {
			t.Fatalf("waitForDaemonStop() status = %#v, want stopped pid 42", status)
		}
		if status.Network != nil {
			t.Fatalf("waitForDaemonStop() network = %#v, want nil after stop", status.Network)
		}
	})
}

func TestDaemonStopCommandSignalsAndWaitsForShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Should signal daemon and wait for stopped status", func(t *testing.T) {
		t.Parallel()

		var (
			signalPID  int
			signalSent bool
		)

		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, errors.New("daemon unavailable")
			},
		})
		deps.pollInterval = time.Millisecond
		deps.stopTimeout = 100 * time.Millisecond
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{
				PID:       42,
				StartedAt: fixedTestNow,
			}, nil
		}
		aliveChecks := 0
		deps.processAlive = func(int) bool {
			aliveChecks++
			return aliveChecks < 2
		}
		deps.signalProcess = func(pid int, _ syscall.Signal) error {
			signalPID = pid
			signalSent = true
			return nil
		}

		stdout, _, err := executeRootCommand(t, deps, "daemon", "stop", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if !signalSent || signalPID != 42 {
			t.Fatalf("signalProcess() = (%v, %d), want true pid 42", signalSent, signalPID)
		}

		var decoded DaemonStatus
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Status != "stopped" || decoded.PID != 42 {
			t.Fatalf("decoded = %#v, want stopped pid 42", decoded)
		}
	})
}

func TestDaemonStopCommandRejectsReusedPIDFromDaemonInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse to signal a reused PID when daemon info start time no longer matches", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{
				PID:       42,
				StartedAt: fixedTestNow,
			}, nil
		}
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return false }

		signalCalled := false
		deps.signalProcess = func(int, syscall.Signal) error {
			signalCalled = true
			return nil
		}

		_, _, err := executeRootCommand(t, deps, "daemon", "stop")
		if err == nil || !strings.Contains(err.Error(), "daemon is not running") {
			t.Fatalf("daemon stop error = %v, want daemon is not running", err)
		}
		if signalCalled {
			t.Fatal("signalProcess() called for reused PID, want no signal")
		}
	})
}

func TestStatusCommandReturnsDaemonStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should return daemon status payload", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			statusFn: func(context.Context) (StatusRecord, error) {
				return StatusRecord{
					Daemon: DaemonStatus{
						Status:    "ready",
						PID:       42,
						StartedAt: fixedTestNow,
						SchemaStreams: []contract.SchemaStreamStatus{
							{Stream: "global", Version: 1, AppliedCount: 1, SumDigest: "sha256:global"},
							{Stream: "memory", Version: 1, AppliedCount: 1, SumDigest: "sha256:memory"},
						},
					},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "status", "-o", "json")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}

		var decoded StatusRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if decoded.Daemon.Status != "ready" || decoded.Daemon.PID != 42 {
			t.Fatalf("decoded = %#v, want ready pid 42", decoded)
		}
		if got, want := decoded.Daemon.SchemaStreams, []contract.SchemaStreamStatus{
			{Stream: "global", Version: 1, AppliedCount: 1, SumDigest: "sha256:global"},
			{Stream: "memory", Version: 1, AppliedCount: 1, SumDigest: "sha256:memory"},
		}; !slices.Equal(got, want) {
			t.Fatalf("decoded schema streams = %#v, want %#v", got, want)
		}
	})

	t.Run("Should render degraded subprocess health and needs-attention runs", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			statusFn: func(context.Context) (StatusRecord, error) {
				return StatusRecord{
					Daemon: DaemonStatus{Status: "ready", PID: 42, StartedAt: fixedTestNow},
					SubprocessHealth: contract.SubprocessHealthAggregatePayload{
						Status:    "degraded",
						Monitored: 1,
						Unhealthy: 1,
					},
					Tasks: contract.TaskHealthPayload{RunTotals: []contract.TaskRunTotalPayload{{
						Status: "needs_attention",
						Count:  1,
					}}},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(t, deps, "status")
		if err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		for _, expected := range []string{"Subprocess Health", "degraded", "Needs Attention", "1 task run"} {
			if !strings.Contains(stdout, expected) {
				t.Fatalf("status output = %q, want %q", stdout, expected)
			}
		}
	})
}

func TestDrainCommandsUpdateDaemonAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		command   string
		wantState contract.DrainState
		client    *stubClient
	}{
		{
			name:      "Should drain daemon admission",
			command:   drainCommandKey,
			wantState: contract.DrainStateDraining,
			client: &stubClient{drainFn: func(context.Context) (DrainStatusRecord, error) {
				return DrainStatusRecord{State: contract.DrainStateDraining}, nil
			}},
		},
		{
			name:      "Should resume daemon admission",
			command:   undrainCommandKey,
			wantState: contract.DrainStateActive,
			client: &stubClient{undrainFn: func(context.Context) (DrainStatusRecord, error) {
				return DrainStatusRecord{State: contract.DrainStateActive}, nil
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, tt.client)
			stdout, _, err := executeRootCommand(t, deps, tt.command, "-o", "json")
			if err != nil {
				t.Fatalf("executeRootCommand() error = %v", err)
			}

			var decoded DrainStatusRecord
			if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if decoded.State != tt.wantState {
				t.Fatalf("state = %q, want %q", decoded.State, tt.wantState)
			}
		})
	}
}

func TestRunDaemonForegroundRunsDaemonWhenNotAlreadyRunning(t *testing.T) {
	t.Parallel()

	t.Run("Should run daemon when no daemon is already running", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		runner := &stubRunner{runFn: func(context.Context) error {
			lock, err := acquireDaemonStartUpdateLock(daemonStartUpdateLockPath(homePaths))
			if err != nil {
				return err
			}
			return lock.Release()
		}}
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{}, os.ErrNotExist
		}
		deps.newDaemon = func() (daemonRunner, error) {
			return runner, nil
		}

		if err := runDaemonForeground(testutil.Context(t), deps, false); err != nil {
			t.Fatalf("runDaemonForeground() error = %v", err)
		}
		if !runner.ran {
			t.Fatal("daemon runner did not execute")
		}
	})

	t.Run("Should run daemon and reap the parent-exit watcher when orphan exit is enabled", func(t *testing.T) {
		t.Parallel()

		runner := &stubRunner{}
		deps := newTestDeps(t, &stubClient{})
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{}, os.ErrNotExist
		}
		deps.newDaemon = func() (daemonRunner, error) {
			return runner, nil
		}

		// Returns only after the watcher goroutine is canceled and joined, so a
		// hang here would prove the watchdog leaks its goroutine.
		if err := runDaemonForeground(testutil.Context(t), deps, true); err != nil {
			t.Fatalf("runDaemonForeground() error = %v", err)
		}
		if !runner.ran {
			t.Fatal("daemon runner did not execute")
		}
	})
}

func TestRunDaemonDetachedReturnsRunningStatus(t *testing.T) {
	t.Parallel()

	t.Run("Should return running status when detached daemon becomes ready", func(t *testing.T) {
		t.Parallel()

		child := &stubDaemonProcess{done: make(chan struct{})}
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{Status: daemonRunningStatus, PID: 42}, nil
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 100 * time.Millisecond
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{}, os.ErrNotExist
		}
		deps.spawnDetached = func(context.Context, compozyconfig.HomePaths) (daemonProcess, error) {
			return child, nil
		}

		status, err := runDaemonDetached(testutil.Context(t), deps)
		child.complete(nil)
		if err != nil {
			t.Fatalf("runDaemonDetached() error = %v", err)
		}
		if status.Status != daemonRunningStatus || status.PID != 42 {
			t.Fatalf("runDaemonDetached() status = %#v, want running pid 42", status)
		}
	})
}

func TestRunDaemonDetachedIgnoresReusedPIDFromDaemonInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should start a detached daemon when daemon info points to a reused PID", func(t *testing.T) {
		t.Parallel()

		child := &stubDaemonProcess{pid: 84, done: make(chan struct{})}
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{Status: daemonRunningStatus, PID: 84}, nil
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 100 * time.Millisecond
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{
				PID:       42,
				StartedAt: fixedTestNow,
			}, nil
		}
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return false }

		spawned := false
		deps.spawnDetached = func(context.Context, compozyconfig.HomePaths) (daemonProcess, error) {
			spawned = true
			return child, nil
		}

		status, err := runDaemonDetached(testutil.Context(t), deps)
		child.complete(nil)
		if err != nil {
			t.Fatalf("runDaemonDetached() error = %v", err)
		}
		if !spawned {
			t.Fatal("spawnDetached() not called, want detached launch for stale daemon info")
		}
		if status.Status != daemonRunningStatus || status.PID != 84 {
			t.Fatalf("runDaemonDetached() status = %#v, want running pid 84", status)
		}
	})
}

func TestRunDaemonDetachedReprobesAfterAcquiringUpdateLock(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse to spawn when the post-lock probe finds a running daemon", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return true }
		spawned := false
		deps.spawnDetached = func(context.Context, compozyconfig.HomePaths) (daemonProcess, error) {
			spawned = true
			return &stubDaemonProcess{done: make(chan struct{})}, nil
		}

		_, err := runDaemonDetached(testutil.Context(t), deps)
		assertErrorContains(t, err, "daemon already running")
		if spawned {
			t.Fatal("spawnDetached() called after post-lock probe found a running daemon")
		}
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("resolveHome() error = %v", err)
		}
		contents, err := os.ReadFile(daemonStartUpdateLockPath(homePaths))
		if err != nil {
			t.Fatalf("os.ReadFile(update lock) error = %v", err)
		}
		if len(contents) != 0 {
			t.Fatalf("released update lock contents = %q, want empty", contents)
		}
	})
}
