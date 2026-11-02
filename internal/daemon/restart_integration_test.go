//go:build integration

package daemon

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/transcript"
)

type restartLabSession struct {
	ID, ACPID, State, StopReason string
	Detail, Transition           string
	Failure                      *store.SessionFailure
	Partial, Finished            bool
	Continuations                int
	Badge                        string
	PublicReads                  int
}

type restartLabProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

// Invariant: real daemon replacement preserves three in-flight conversations, resumes each once,
// reaps their old processes, and never continues explicit stops. Fresh HTTP/UDS reads retain the
// transcript and never project failed. Owner: composition-root restart suite.
func TestDaemonRestartContinuesSessions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("subprocess signal scenario requires Unix")
	}
	driver := acpmock.RequireDriver(t)
	for _, mode := range []string{"restart", "crash", "stop"} {
		t.Run("Should recover concurrent sessions after "+mode, func(t *testing.T) {
			root := t.TempDir()
			restartLabFixture(t, root)
			first, original := startRestartLabDaemon(t, root, driver, mode, "first")
			t.Logf("before interruption: %+v", original)
			if len(original) != 3 {
				t.Fatalf("initial sessions = %#v", original)
			}
			if mode == "crash" {
				if err := first.cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				<-first.done
			} else {
				stopRestartLabDaemon(t, first)
			}
			for _, phase := range []string{"second", "third"} {
				next, restored := startRestartLabDaemon(t, root, driver, "stop", phase)
				t.Logf("%s public HTTP/UDS state: %+v", phase, restored)
				if len(restored) != len(original) {
					t.Fatalf("restored sessions = %#v", restored)
				}
				for i, got := range restored {
					want := original[i]
					if got.ID != want.ID || got.ACPID != want.ACPID || !got.Partial {
						t.Fatalf("lost conversation: original=%#v restored=%#v", want, got)
					}
					if mode == "stop" {
						if got.Continuations != 0 || got.Finished || got.StopReason != string(store.StopUserCanceled) {
							t.Fatalf("explicit stop resumed: %#v", got)
						}
					} else if got.Continuations != 1 || !got.Finished {
						t.Fatalf("restart did not finish exactly once: %#v", got)
					}
				}
				stopRestartLabDaemon(t, next)
				records, err := acpmock.ReadDiagnostics(
					filepath.Join(root, "runtime", "logs", "acpmock", "restart-lab.jsonl"),
				)
				if err != nil {
					t.Fatal(err)
				}
				continued := 0
				for _, record := range records {
					if record.TurnName == "continued" {
						continued++
					}
				}
				wantContinued := 3
				if mode == "stop" {
					wantContinued = 0
				}
				if continued != wantContinued {
					t.Fatalf("actual ACP continuations after %s = %d, want %d", phase, continued, wantContinued)
				}
			}
		})
	}
}

func restartLabFixture(t *testing.T, root string) {
	t.Helper()
	fixture := acpmock.Fixture{Version: acpmock.FixtureVersion, Agents: []acpmock.AgentFixture{{
		Name: "restart-lab", Provider: acpmock.ProviderName, Permissions: "approve-all",
		Turns: []acpmock.TurnFixture{
			{Name: "in-flight", Match: acpmock.TurnMatch{UserText: "restart lab work"}, Steps: []acpmock.Step{
				{Kind: acpmock.StepKindAssistant, Text: "Partial work preserved."},
				{Kind: acpmock.StepKindDriverControl, DriverControl: &acpmock.DriverControlStep{
					Action: acpmock.DriverControlBlockUntilCancel}},
			}},
			{
				Name:  "continued",
				Match: acpmock.TurnMatch{UserTextContains: "The daemon restarted"},
				Steps: []acpmock.Step{
					{Kind: acpmock.StepKindAssistant, Text: "Restart work finished."},
				},
			},
		},
	}}}
	data, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixture.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func startRestartLabDaemon(t *testing.T, root, driver, mode, phase string) (*restartLabProcess, []restartLabSession) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ready := filepath.Join(root, phase+".json")
	logFile, err := os.Create(filepath.Join(root, phase+".log"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = logFile.Close() })
	cmd := exec.Command(binary, "-test.run=^TestRestartSessionDaemonProcess$", "-test.timeout=90s")
	cmd.Env = append(os.Environ(), "COMPOZY_RESTART_LAB="+root, "COMPOZY_RESTART_MODE="+mode,
		"COMPOZY_RESTART_PHASE="+phase, "COMPOZY_TEST_ACPMOCK_DRIVER_BIN="+driver,
		"HOME="+root, "COMPOZY_HOME="+filepath.Join(root, "runtime"), "CODEX_HOME="+filepath.Join(root, ".codex"))
	cmd.Stdout, cmd.Stderr = logFile, logFile
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	process := &restartLabProcess{cmd: cmd, done: make(chan struct{})}
	go func() {
		process.err = cmd.Wait()
		close(process.done)
	}()
	t.Cleanup(func() {
		select {
		case <-process.done:
			return
		default:
		}
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-process.done:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-process.done
		}
	})
	if helper := os.Getenv("COMPOZY_QA_PROCESS_HELPER"); helper != "" {
		pidFile := filepath.Join(
			os.Getenv("QA_OUTPUT_PATH"),
			"qa",
			"pids",
			fmt.Sprintf("daemon-%d.pid", cmd.Process.Pid),
		)
		output, err := exec.Command("python3", helper, "--pid", strconv.Itoa(cmd.Process.Pid), "--file", pidFile).
			CombinedOutput()
		if err != nil {
			t.Fatalf("register daemon: %v: %s", err, output)
		}
	}
	deadline := time.NewTimer(45 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(ready)
		var report []restartLabSession
		if err == nil && json.Unmarshal(data, &report) == nil {
			return process, report
		}
		select {
		case <-ticker.C:
		case <-process.done:
			log, _ := os.ReadFile(logFile.Name())
			t.Fatalf("daemon exited before readiness (%s): %v: %s", phase, process.err, log)
		case <-deadline.C:
			log, _ := os.ReadFile(logFile.Name())
			t.Fatalf("daemon did not become ready (%s): %s", phase, log)
		}
	}
}

func stopRestartLabDaemon(t *testing.T, process *restartLabProcess) {
	t.Helper()
	if err := process.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	<-process.done
	if process.err != nil {
		t.Fatalf("daemon stop: %v", process.err)
	}
}

// TestRestartSessionDaemonProcess runs the production composition root in a disposable process.
// Only the relaunch process launcher is replaced: the parent owns and verifies the replacement.
func TestRestartSessionDaemonProcess(t *testing.T) {
	root := os.Getenv("COMPOZY_RESTART_LAB")
	if root == "" {
		return
	}
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	home, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(root, "runtime"))
	if err != nil {
		t.Fatal(err)
	}
	home.DaemonSocket = shortSocketPath(t)
	registration, err := acpmock.Register(home, acpmock.RegisterOptions{
		FixturePath: filepath.Join(root, "fixture.json"), FixtureAgent: "restart-lab",
		DriverPath: os.Getenv("COMPOZY_TEST_ACPMOCK_DRIVER_BIN"),
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := e2etest.SeedConfig(t, home, e2etest.ConfigSeedOptions{
		SocketPath: home.DaemonSocket, DefaultAgent: "restart-lab", DefaultProvider: acpmock.ProviderName,
		Providers: map[string]compozyconfig.ProviderConfig{
			acpmock.ProviderName: acpmock.ProviderConfig(registration.DriverPath),
		},
		PermissionMode: compozyconfig.PermissionModeApproveAll,
		Mutate: func(cfg *compozyconfig.Config) {
			cfg.Daemon.ShutdownGrace = time.Millisecond
			cfg.Session.Stop.CooperativeGrace = 100 * time.Millisecond
			cfg.Automation.Enabled = false
			cfg.Roles.AutoTitle.Enabled = false
			cfg.ModelCatalog.Sources.ModelsDev.Enabled = new(false)
			cfg.ProviderUsage.Enabled = false
			for name, provider := range compozyconfig.BuiltinProviders() {
				provider.Models.Discovery.Enabled = new(false)
				cfg.Providers[name] = provider
			}
		},
	})
	d, err := New(WithHomePaths(home), WithConfig(&cfg), WithLogger(discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.boot(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := d.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown: %v", err)
		}
	})
	phase := os.Getenv("COMPOZY_RESTART_PHASE")
	if phase == "first" {
		workspace := resolveDaemonWorkspace(t, d.workspaceResolver, filepath.Join(root, "project"))
		for i := range 3 {
			active, err := d.sessions.Create(t.Context(), new(session.CreateOpts{
				AgentName: "restart-lab", Workspace: workspace.ID, Type: session.SessionTypeUser,
				DesiredSessionID: fmt.Sprintf("sess-restart-lab-%d", i),
			}))
			if err != nil {
				t.Fatal(err)
			}
			stream, err := d.sessions.Prompt(t.Context(), active.ID, "restart lab work")
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				for range stream {
				}
			}()
		}
	}
	report := awaitRestartLabReport(t, d, phase)
	verifyRestartLabPublicState(t, d, report)
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, phase+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	<-signals
	if os.Getenv("COMPOZY_RESTART_MODE") == "restart" {
		d.startDetached = func(context.Context, detachedStartRequest) (restartProcess, error) {
			return restartProcessStub{pid: os.Getpid()}, nil
		}
		d.signalProcess = func(int, syscall.Signal) error { return nil }
		if _, err := d.RequestRestart(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
}

func awaitRestartLabReport(t *testing.T, d *Daemon, phase string) []restartLabSession {
	t.Helper()
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		var report []restartLabSession
		ready := true
		for i := range 3 {
			id := fmt.Sprintf("sess-restart-lab-%d", i)
			info, err := d.sessions.Status(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := d.sessions.Events(t.Context(), id, store.EventQuery{})
			if err != nil {
				t.Fatal(err)
			}
			item := restartLabSession{
				ID:         id,
				ACPID:      info.ACPSessionID,
				State:      string(info.State),
				StopReason: string(info.StopReason),
				Detail:     info.StopDetail, Transition: string(info.RuntimeTransition), Failure: info.Failure,
			}
			for _, row := range rows {
				if row.Type == events.SessionRestartResumed {
					item.Continuations++
				}
				event, err := transcript.UnmarshalAgentEvent(row.Content)
				if err != nil {
					t.Fatal(err)
				}
				if event.Type == acp.EventTypeAgentMessage {
					item.Partial = item.Partial || strings.Contains(event.Text, "Partial work preserved.")
					item.Finished = item.Finished || strings.Contains(event.Text, "Restart work finished.")
				}
			}
			prompting := d.sessions.(sessionPromptStateReader).IsPrompting(id)
			if !item.Partial ||
				(phase != "first" && info.State == session.StateActive && (!item.Finished || prompting)) {
				ready = false
			}
			report = append(report, item)
		}
		if ready {
			return report
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatalf("restart work did not settle: %#v", report)
		}
	}
}

// verifyRestartLabPublicState checks the projections consumed by the Web and CLI after each boot.
func verifyRestartLabPublicState(t *testing.T, d *Daemon, report []restartLabSession) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	httpClient := &http.Client{Transport: &http.Transport{}}
	udsClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", d.homePaths.DaemonSocket)
		},
	}}
	defer httpClient.CloseIdleConnections()
	defer udsClient.CloseIdleConnections()
	harness := &e2etest.RuntimeHarness{
		HTTPBaseURL: fmt.Sprintf("http://%s:%d", d.config.HTTP.Host, d.config.HTTP.Port),
		HTTPClient:  httpClient, UDSBaseURL: "http://unix", UDSClient: udsClient,
	}
	for i := range report {
		item := &report[i]
		info, err := d.sessions.Status(ctx, item.ID)
		if err != nil {
			t.Fatal(err)
		}
		path := "/api/workspaces/" + info.WorkspaceID + "/sessions/" + item.ID
		for _, read := range []func(context.Context, string, string, any, any) error{
			harness.HTTPJSON, harness.UDSJSON,
		} {
			var detail compozycontract.SessionResponse
			if err := read(ctx, http.MethodGet, path, nil, &detail); err != nil {
				t.Fatal(err)
			}
			got := detail.Session
			if got.ID != item.ID || string(got.State) != item.State ||
				string(got.StopReason) != item.StopReason || got.Badge == session.BadgeFailed {
				t.Fatalf("public state differs: got=%+v internal=%+v", got, item)
			}
			if got.StopReason == store.StopUserCanceled {
				if got.Failure == nil || got.Failure.Kind != store.FailureCanceled {
					t.Fatalf("explicit stop lost cancellation diagnostic: %+v", got.Failure)
				}
			} else if got.Failure != nil {
				t.Fatalf("restart projected a failure: %+v", got.Failure)
			}
			if item.Badge != "" && item.Badge != string(got.Badge) {
				t.Fatalf("public transports disagree: got=%s previous=%s", got.Badge, item.Badge)
			}
			item.Badge = string(got.Badge)
			var history compozycontract.SessionEventsResponse
			if err := read(ctx, http.MethodGet, path+"/events", nil, &history); err != nil {
				t.Fatal(err)
			}
			partial, finished, continuations := false, false, 0
			for _, row := range history.Events {
				if row.Type == events.SessionRestartResumed {
					continuations++
				}
				partial = partial || strings.Contains(string(row.Content), "Partial work preserved.")
				finished = finished || strings.Contains(string(row.Content), "Restart work finished.")
			}
			if partial != item.Partial || finished != item.Finished || continuations != item.Continuations {
				t.Fatalf("public history differs: partial=%t finished=%t continuations=%d expected=%+v",
					partial, finished, continuations, item)
			}
			item.PublicReads++
		}
	}
}

// TestBootMarksRestartOperationReadyAfterFreshDaemonInfo verifies real boot completes before readiness and reconciles abandoned restarts.
func TestBootMarksRestartOperationReadyAfterFreshDaemonInfo(t *testing.T) {
	t.Parallel()

	t.Run("Should persist the replacement process identity before marking restart ready", func(t *testing.T) {
		t.Parallel()

		homeDir := t.TempDir()
		homePaths, err := compozyconfig.ResolveHomePathsFrom(homeDir)
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		homePaths.DaemonSocket = shortSocketPath(t)
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

		abandoned := operation
		abandoned.OperationID = "restart-abandoned-observer"
		if _, err := store.Create(abandoned); err != nil {
			t.Fatalf("create abandoned observation: %v", err)
		}
		// Unreadable historical metadata must not prevent recovery of valid observations or boot.
		if err := os.WriteFile(
			filepath.Join(homePaths.RestartsDir, "corrupt-history.json"),
			[]byte("{"),
			0o600,
		); err != nil {
			t.Fatal(err)
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
		replacementStartedAt := time.Date(2026, 4, 17, 12, 1, 0, 0, time.UTC)
		d.processStartedAt = func(int) (time.Time, error) {
			return replacementStartedAt, nil
		}
		d.getenv = func(key string) string {
			if key == RestartOperationEnvKey {
				return operation.OperationID
			}
			if key == "HOME" {
				return homePaths.HomeDir
			}
			return os.Getenv(key)
		}

		helper := newRelaunchHelper(&RelaunchHelperConfig{HomePaths: homePaths, OperationID: operation.OperationID,
			PollInterval: time.Millisecond, ReadyTimeout: time.Millisecond, ExitDrainWait: time.Millisecond})
		childExit := make(chan struct{})
		observed := make(chan error, 1)
		go func() {
			observed <- helper.waitForReady(t.Context(), store, operation.OperationID, restartProcessStub{
				pid: 9393, wait: func() error { <-childExit; return nil },
			})
		}()
		t.Cleanup(func() { close(childExit) })
		timer := time.NewTimer(15 * time.Millisecond)
		defer timer.Stop()
		select {
		case err := <-observed:
			t.Fatalf("restart stopped observing a live boot: %v", err)
		case <-timer.C:
		}
		pending, err := store.Get(operation.OperationID)
		if err != nil || pending.Status != RestartStatusStarting {
			t.Fatalf("pre-boot status = %#v, %v", pending, err)
		}
		if err := d.boot(testutil.Context(t)); err != nil {
			t.Fatalf("boot() error = %v", err)
		}
		t.Cleanup(func() {
			if err := d.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		completion := time.NewTimer(10 * time.Second)
		defer completion.Stop()
		select {
		case err := <-observed:
			if err != nil {
				t.Fatalf("observe completed boot: %v", err)
			}
		case <-completion.C:
			t.Fatal("restart observer did not complete")
		}
		reconciled, err := store.Get(abandoned.OperationID)
		if err != nil || reconciled.Status != RestartStatusFailed || reconciled.CompletedAt == nil {
			t.Fatalf("superseded restart = %#v, %v", reconciled, err)
		}

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
		info, err := ReadInfo(homePaths.DaemonInfo)
		if err != nil {
			t.Fatalf("ReadInfo(daemon.json) error = %v", err)
		}
		if !info.StartedAt.Equal(replacementStartedAt) {
			t.Fatalf("replacement started_at = %v, want %v", info.StartedAt, replacementStartedAt)
		}
	})
}
