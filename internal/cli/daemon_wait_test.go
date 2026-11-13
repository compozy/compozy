package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/procutil"
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

func TestRefreshDesktopOwnedRuntimePath(t *testing.T) {
	t.Run("Should preserve the inherited PATH for an operator-managed runtime", func(t *testing.T) {
		t.Parallel()

		resolved := false
		set := false
		err := refreshDesktopOwnedRuntimePath(t.Context(), compozyconfig.HomePaths{}, desktopRuntimePathDependencies{
			executable:     func() (string, error) { return "/operator/compozy", nil },
			ownedByDesktop: func(compozyconfig.HomePaths, string) bool { return false },
			environment:    func() []string { return []string{"PATH=/inherited/bin"} },
			resolve: func(context.Context, []string) (string, error) {
				resolved = true
				return "/login/bin", nil
			},
			setenv: func(string, string) error {
				set = true
				return nil
			},
		})
		if err != nil {
			t.Fatalf("refreshDesktopOwnedRuntimePath() error = %v", err)
		}
		if resolved || set {
			t.Fatalf("operator-managed runtime changed PATH: resolved=%t set=%t", resolved, set)
		}
	})

	t.Run("Should install the login shell PATH for a desktop-owned runtime", func(t *testing.T) {
		t.Parallel()

		var gotKey string
		var gotValue string
		err := refreshDesktopOwnedRuntimePath(t.Context(), compozyconfig.HomePaths{}, desktopRuntimePathDependencies{
			executable:     func() (string, error) { return "/desktop/compozy", nil },
			ownedByDesktop: func(compozyconfig.HomePaths, string) bool { return true },
			environment:    func() []string { return []string{"PATH=/gui/bin"} },
			resolve: func(_ context.Context, environment []string) (string, error) {
				if !slices.Contains(environment, "PATH=/gui/bin") {
					t.Fatalf("resolve environment = %#v, want inherited PATH", environment)
				}
				return "/login/bin:/usr/bin", nil
			},
			setenv: func(key string, value string) error {
				gotKey = key
				gotValue = value
				return nil
			},
		})
		if err != nil {
			t.Fatalf("refreshDesktopOwnedRuntimePath() error = %v", err)
		}
		if gotKey != "PATH" || gotValue != "/login/bin:/usr/bin" {
			t.Fatalf("setenv() = %q/%q, want corrected PATH", gotKey, gotValue)
		}
	})

	t.Run("Should preserve the inherited PATH when login shell resolution fails", func(t *testing.T) {
		t.Parallel()

		resolveErr := errors.New("login shell failed")
		set := false
		err := refreshDesktopOwnedRuntimePath(t.Context(), compozyconfig.HomePaths{}, desktopRuntimePathDependencies{
			executable:     func() (string, error) { return "/desktop/compozy", nil },
			ownedByDesktop: func(compozyconfig.HomePaths, string) bool { return true },
			environment:    func() []string { return []string{"PATH=/gui/bin"} },
			resolve:        func(context.Context, []string) (string, error) { return "", resolveErr },
			setenv: func(string, string) error {
				set = true
				return nil
			},
		})
		if !errors.Is(err, resolveErr) {
			t.Fatalf("refreshDesktopOwnedRuntimePath() error = %v, want resolver error", err)
		}
		if set {
			t.Fatal("refreshDesktopOwnedRuntimePath() changed PATH after resolver failure")
		}
	})
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

func TestWaitForDaemonStopReturnsStoppedStatusWhenProcessExits(t *testing.T) {
	t.Parallel()

	// Invariant: process exit remains observable while the shutting-down API is unresponsive.
	// Owner: CLI daemon lifecycle; canonical suite: daemon_wait_test.go.
	t.Run("Should observe process exit without waiting for an unresponsive status API", func(t *testing.T) {
		t.Parallel()
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(ctx context.Context) (DaemonStatus, error) {
				<-ctx.Done()
				return DaemonStatus{}, ctx.Err()
			},
		})
		deps.pollInterval = time.Millisecond
		info := compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) { return info, nil }
		aliveChecks := 0
		deps.processAlive = func(int) bool {
			aliveChecks++
			return aliveChecks < 2
		}
		runtime, err := loadRuntimeContext(deps)
		if err != nil {
			t.Fatalf("loadRuntimeContext() error = %v", err)
		}
		ctx, cancel := context.WithTimeout(testutil.Context(t), time.Second)
		defer cancel()
		status, err := waitForDaemonStop(ctx, deps, runtime, info)
		if err != nil || ctx.Err() != nil || status.Status != "stopped" || status.PID != 42 {
			t.Fatalf("waitForDaemonStop() = %#v, error = %v, context error = %v", status, err, ctx.Err())
		}
	})

	t.Run("Should outlive the daemon graceful shutdown budget", func(t *testing.T) {
		t.Parallel()

		config := compozyconfig.Config{}
		want := max(defaultStopTimeout, compozydaemon.GracefulShutdownTimeout(&config)+daemonStopWaitMargin)
		if got := daemonStopWaitTimeout(defaultStopTimeout, &config); got != want {
			t.Fatalf("daemonStopWaitTimeout() = %s, want %s", got, want)
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
							{Stream: "workspace", Version: 1, AppliedCount: 1, SumDigest: "sha256:workspace"},
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
			{Stream: "workspace", Version: 1, AppliedCount: 1, SumDigest: "sha256:workspace"},
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

	// UT-026: every row of the _dx.md OTel line table.
	exported := time.Date(2026, 11, 7, 14, 2, 11, 0, time.UTC)
	healthy := contract.OTelSignalStatusPayload{Enabled: true, LastExportAt: exported}
	otelLines := []struct {
		name string
		otel *contract.OTelStatusPayload
		want string
	}{
		{name: "Should render OTel disabled", otel: &contract.OTelStatusPayload{State: "disabled"}, want: "disabled"},
		{
			name: "Should render OTel inactive when local observability is off",
			otel: &contract.OTelStatusPayload{State: "inactive_observability_disabled"},
			want: "inactive (observability.enabled = false)",
		},
		{
			name: "Should render healthy OTel export",
			otel: &contract.OTelStatusPayload{
				State: "ok", Protocol: "grpc", EndpointHost: "collector.local:4317", SampleRatio: 0.25,
				Traces: healthy, Metrics: healthy,
			},
			want: "ok · grpc → collector.local:4317 · traces ok, metrics ok · sample 0.25",
		},
		{
			name: "Should render OTel metrics off",
			otel: &contract.OTelStatusPayload{
				State: "ok", Protocol: "http/protobuf", EndpointHost: "localhost:4318", SampleRatio: 1, Traces: healthy,
			},
			want: "ok · http/protobuf → localhost:4318 · traces ok, metrics off · sample 1.0",
		},
		{
			name: "Should render a degraded OTel signal with its failures",
			otel: &contract.OTelStatusPayload{
				State: "degraded", Protocol: "grpc", EndpointHost: "collector.local:4317", SampleRatio: 0.25,
				Traces: healthy,
				Metrics: contract.OTelSignalStatusPayload{
					Enabled: true, LastExportAt: exported, LastErrorClass: "http_401", FailedExports: 12,
				},
			},
			want: "degraded · grpc → collector.local:4317 · traces ok, metrics http_401 (12 failed) · sample 0.25",
		},
		{
			name: "Should render OTel forwarding to providers",
			otel: &contract.OTelStatusPayload{
				State: "ok", Protocol: "grpc", EndpointHost: "collector.local:4317", SampleRatio: 0.25,
				ForwardToProviders: true, Traces: healthy, Metrics: healthy,
			},
			want: "ok · grpc → collector.local:4317 · traces ok, metrics ok · sample 0.25 · forwarding to providers",
		},
		{
			name: "Should render OTel signals pending before their first export",
			otel: &contract.OTelStatusPayload{
				State: "ok", Protocol: "http/protobuf", EndpointHost: "localhost:4318", SampleRatio: 1,
				Traces:  contract.OTelSignalStatusPayload{Enabled: true},
				Metrics: contract.OTelSignalStatusPayload{Enabled: true},
			},
			want: "ok · http/protobuf → localhost:4318 · traces pending, metrics pending · sample 1.0",
		},
		{name: "Should render unknown OTel status from an older daemon", want: "--"},
	}
	for _, tt := range otelLines {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				statusFn: func(context.Context) (StatusRecord, error) {
					return StatusRecord{
						Daemon: DaemonStatus{Status: "ready", PID: 42, StartedAt: fixedTestNow},
						OTel:   tt.otel,
					}, nil
				},
			})

			stdout, _, err := executeRootCommand(t, deps, "status")
			if err != nil {
				t.Fatalf("executeRootCommand() error = %v", err)
			}
			want := "\nOTel:               " + tt.want + "\n"
			if !strings.Contains(stdout, want) {
				t.Fatalf("status output = %q, want line %q", stdout, want)
			}
		})
	}
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

func TestDaemonClockDrift(t *testing.T) {
	t.Parallel()
	for _, action := range []string{"stop", "start", "bootstrap", "update"} {
		t.Run("Should preserve daemon identity across clock drift during "+action, func(t *testing.T) {
			t.Parallel()
			restarted := false
			client := &stubClient{
				triggerSettingsRestartFn: func(context.Context) (SettingsRestartActionRecord, error) {
					restarted = true
					return SettingsRestartActionRecord{OperationID: "clock-drift"}, nil
				},
				getSettingsRestartStatusFn: func(context.Context, string) (SettingsRestartStatusRecord, error) {
					return SettingsRestartStatusRecord{OperationID: "clock-drift", Status: "ready"}, nil
				},
			}
			deps := newTestDeps(t, client).withDefaults()
			paths, err := deps.resolveHome()
			if err != nil {
				t.Fatal(err)
			}
			info := compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}
			if err := compozydaemon.WriteInfo(paths.DaemonInfo, info); err != nil {
				t.Fatal(err)
			}
			deps.processMatchesStartTime = func(_ int, recorded time.Time) bool {
				return procutil.MatchesObservedStartTime(info.StartedAt.Add(40*time.Hour), recorded)
			}
			deps.processAlive = func(int) bool { return true }
			deps.processOwnsFileLock = func(pid int, path string) (bool, error) {
				if pid != info.PID || path != paths.DaemonLock {
					t.Fatalf("lock lookup = %d, %s", pid, path)
				}
				return true, nil
			}
			signaled := false
			deps.signalProcess = func(pid int, signal syscall.Signal) error {
				if pid != info.PID || signal != syscall.SIGTERM {
					t.Fatalf("signal = %d, %v", pid, signal)
				}
				signaled = true
				return compozydaemon.RemoveInfo(paths.DaemonInfo)
			}
			switch action {
			case "stop":
				_, _, err = executeRootCommand(t, deps, "daemon", "stop")
				if err != nil || !signaled {
					t.Fatalf("stop = %v; signaled = %v", err, signaled)
				}
			case "start":
				_, _, err = executeRootCommand(t, deps, "daemon", "start")
				if err == nil || !strings.Contains(err.Error(), "already running (pid=42)") {
					t.Fatalf("start = %v", err)
				}
			case "bootstrap":
				if err := cleanupStaleBootstrapDaemonRecord(paths, deps); err != nil {
					t.Fatal(err)
				}
				got, err := compozydaemon.ReadInfo(paths.DaemonInfo)
				if err != nil || got != info {
					t.Fatalf("preserved info = %#v, %v", got, err)
				}
			case "update":
				if err := newCLIUpdateRuntime(deps, paths).RestartDaemon(t.Context()); err != nil {
					t.Fatal(err)
				}
				if !restarted {
					t.Fatal("update did not request restart of the existing daemon")
				}
			}
		})
	}
	t.Run("Should preserve an unverifiable record and report the kernel lookup failure", func(t *testing.T) {
		t.Parallel()
		deps := newTestDeps(t, &stubClient{}).withDefaults()
		paths, err := deps.resolveHome()
		if err != nil {
			t.Fatal(err)
		}
		info := compozydaemon.Info{PID: 42, StartedAt: fixedTestNow}
		if err := compozydaemon.WriteInfo(paths.DaemonInfo, info); err != nil {
			t.Fatal(err)
		}
		deps.processAlive = func(int) bool { return true }
		deps.processMatchesStartTime = func(int, time.Time) bool { return false }
		deps.processOwnsFileLock = func(int, string) (bool, error) { return false, os.ErrPermission }
		if err := cleanupStaleBootstrapDaemonRecord(paths, deps); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("cleanup = %v", err)
		}
		if _, err := compozydaemon.ReadInfo(paths.DaemonInfo); err != nil {
			t.Fatal(err)
		}
	})
}

type profileTestClient struct {
	DaemonClient
	capture func(context.Context, string, int, io.Writer) error
}

var _ daemonProfileClient = (*profileTestClient)(nil)

func (c *profileTestClient) CaptureDaemonProfile(ctx context.Context, kind string, seconds int, dst io.Writer) error {
	return c.capture(ctx, kind, seconds, dst)
}

func TestDaemonProfileCommand(t *testing.T) {
	t.Parallel()
	t.Run("Should save raw profile bytes and report the destination as JSON", func(t *testing.T) {
		t.Parallel()
		client := &profileTestClient{capture: func(_ context.Context, kind string, seconds int, dst io.Writer) error {
			if kind != "cpu" || seconds != 1 {
				t.Errorf("capture = %s %d", kind, seconds)
			}
			_, err := dst.Write([]byte{0x1f, 0x8b, 0x00})
			return err
		}}
		path := filepath.Join(t.TempDir(), "cpu.pprof")
		output, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"daemon",
			"profile",
			"cpu",
			"--seconds",
			"1",
			"--file",
			path,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, []byte{0x1f, 0x8b, 0x00}) {
			t.Fatalf("file=%v", data)
		}
		var report struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(output), &report); err != nil {
			t.Fatalf("invalid JSON: %v; output=%s", err, output)
		}
		if report.Kind != "cpu" || report.Path != path {
			t.Fatalf("report=%+v", report)
		}
		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o600 {
				t.Fatalf("mode = %#o, want 0600", info.Mode().Perm())
			}
		}
	})
	t.Run("Should preserve an existing destination and remove partial files on capture failure", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "heap.pprof")
		if err := os.WriteFile(path, []byte("existing"), 0o600); err != nil {
			t.Fatal(err)
		}
		client := &profileTestClient{capture: func(_ context.Context, _ string, _ int, dst io.Writer) error {
			if _, err := io.WriteString(dst, "partial"); err != nil {
				return err
			}
			return errors.New("capture failed")
		}}
		_, _, err := executeRootCommand(t, newTestDeps(t, client), "daemon", "profile", "heap", "--file", path)
		if err == nil || !strings.Contains(err.Error(), "capture failed") {
			t.Fatalf("error=%v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "existing" {
			t.Fatalf("file=%s", data)
		}
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Fatalf("partial files=%v", files)
		}
	})
	t.Run("Should reject invalid kinds and explicit out of range or inapplicable durations", func(t *testing.T) {
		t.Parallel()
		for _, args := range [][]string{{"cpu", "--seconds", "0"}, {"cpu", "--seconds", "121"}, {"heap", "--seconds", "1"}, {"unknown"}} {
			full := append([]string{"daemon", "profile"}, args...)
			full = append(full, "--file", filepath.Join(t.TempDir(), "invalid.pprof"))
			if _, _, err := executeRootCommand(t, newTestDeps(t, &profileTestClient{}), full...); err == nil {
				t.Fatalf("accepted %v", args)
			}
		}
	})
}
