package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/daemon"
	"github.com/spf13/cobra"
)

type stubDaemonCommandClient struct {
	target       apiclient.Target
	health       apicore.DaemonHealth
	healthErr    error
	healthCalls  int
	startCalls   int
	startSlug    string
	startRequest apicore.TaskRunRequest
	startRun     apicore.Run
	startErr     error
}

func (c *stubDaemonCommandClient) Target() apiclient.Target {
	if c == nil {
		return apiclient.Target{}
	}
	return c.target
}

func (c *stubDaemonCommandClient) Health(context.Context) (apicore.DaemonHealth, error) {
	if c == nil {
		return apicore.DaemonHealth{}, errors.New("stub daemon client is required")
	}
	c.healthCalls++
	if c.healthErr != nil {
		return apicore.DaemonHealth{}, c.healthErr
	}
	return c.health, nil
}

func (c *stubDaemonCommandClient) StartTaskRun(
	_ context.Context,
	slug string,
	req apicore.TaskRunRequest,
) (apicore.Run, error) {
	if c == nil {
		return apicore.Run{}, errors.New("stub daemon client is required")
	}
	c.startCalls++
	c.startSlug = slug
	c.startRequest = req
	if c.startErr != nil {
		return apicore.Run{}, c.startErr
	}
	return c.startRun, nil
}

func installTestCLIDaemonBootstrap(t *testing.T, bootstrap cliDaemonBootstrap) {
	t.Helper()

	original := newCLIDaemonBootstrap
	newCLIDaemonBootstrap = func() cliDaemonBootstrap { return bootstrap }
	t.Cleanup(func() {
		newCLIDaemonBootstrap = original
	})
}

func newTaskRunPresentationCommand(state *commandState) *cobra.Command {
	cmd := &cobra.Command{Use: "compozy tasks run"}
	cmd.Flags().StringVar(&state.attachMode, "attach", attachModeAuto, "attach mode")
	cmd.Flags().Bool("ui", false, "ui mode")
	cmd.Flags().Bool("stream", false, "stream mode")
	cmd.Flags().Bool("detach", false, "detach mode")
	return cmd
}

func decodeTaskRunOverrides(t *testing.T, raw json.RawMessage) daemonRuntimeOverrides {
	t.Helper()

	if len(raw) == 0 {
		return daemonRuntimeOverrides{}
	}
	var payload daemonRuntimeOverrides
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode task run overrides: %v", err)
	}
	return payload
}

func TestResolveTaskPresentationModeUsesInjectedInteractiveCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		interactive   bool
		wantMode      string
		wantErrSubstr string
		configure     func(*testing.T, *commandState, *cobra.Command)
	}{
		{
			name:        "auto resolves to ui on interactive terminals",
			interactive: true,
			wantMode:    attachModeUI,
		},
		{
			name:        "auto resolves to stream on non-interactive terminals",
			interactive: false,
			wantMode:    attachModeStream,
		},
		{
			name:          "explicit ui requires an interactive terminal",
			interactive:   false,
			wantErrSubstr: "requires an interactive terminal for ui mode",
			configure: func(t *testing.T, state *commandState, cmd *cobra.Command) {
				t.Helper()
				if err := cmd.Flags().Set("attach", attachModeUI); err != nil {
					t.Fatalf("set attach: %v", err)
				}
				state.attachMode = attachModeUI
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindStart, "")
			state.isInteractive = func() bool { return tt.interactive }
			cmd := newTaskRunPresentationCommand(state)
			if tt.configure != nil {
				tt.configure(t, state, cmd)
			}

			got, err := state.resolveTaskPresentationMode(cmd)
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected resolveTaskPresentationMode error, got mode %q", got)
				}
				if got != "" {
					t.Fatalf("expected no resolved mode on error, got %q", got)
				}
				if gotErr := err.Error(); !containsAll(gotErr, tt.wantErrSubstr) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveTaskPresentationMode: %v", err)
			}
			if got != tt.wantMode {
				t.Fatalf("unexpected presentation mode: got %q want %q", got, tt.wantMode)
			}
		})
	}
}

func TestNewDefaultCLIDaemonBootstrapProvidesRuntimeDependencies(t *testing.T) {
	t.Parallel()

	bootstrap := newDefaultCLIDaemonBootstrap()
	if bootstrap.resolveHomePaths == nil || bootstrap.readInfo == nil || bootstrap.newClient == nil ||
		bootstrap.launch == nil {
		t.Fatalf("expected daemon bootstrap dependencies to be wired: %#v", bootstrap)
	}
	if bootstrap.sleep == nil || bootstrap.now == nil {
		t.Fatalf("expected daemon bootstrap timing hooks to be wired: %#v", bootstrap)
	}
	if bootstrap.startupTimeout != defaultDaemonStartupTimeout {
		t.Fatalf("unexpected startup timeout: %s", bootstrap.startupTimeout)
	}
	if bootstrap.pollInterval != defaultDaemonPollInterval {
		t.Fatalf("unexpected poll interval: %s", bootstrap.pollInterval)
	}

	client, err := bootstrap.newClient(apiclient.Target{HTTPPort: 43123})
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	if client.Target().HTTPPort != 43123 {
		t.Fatalf("unexpected bootstrap client target: %#v", client.Target())
	}
}

func TestCLIDaemonBootstrapEnsureReusesHealthyDaemon(t *testing.T) {
	t.Parallel()

	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-ready.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}
	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	var launchCalls int
	var newClientTargets []apiclient.Target

	bootstrap := cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(path string) (daemon.Info, error) {
			if path != "/tmp/compozy-home/daemon.json" {
				t.Fatalf("unexpected daemon info path: %q", path)
			}
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-ready.sock",
				StartedAt:  now,
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(target apiclient.Target) (daemonCommandClient, error) {
			newClientTargets = append(newClientTargets, target)
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			launchCalls++
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            func() time.Time { return now },
		startupTimeout: time.Second,
		pollInterval:   time.Millisecond,
	}

	gotClient, err := bootstrap.ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if gotClient != readyClient {
		t.Fatalf("ensure returned unexpected client: %#v", gotClient)
	}
	if launchCalls != 0 {
		t.Fatalf("expected healthy daemon reuse without launch, got %d launch attempts", launchCalls)
	}
	if readyClient.healthCalls != 1 {
		t.Fatalf("expected one health probe, got %d", readyClient.healthCalls)
	}
	if len(newClientTargets) != 1 || newClientTargets[0].SocketPath != "/tmp/compozy-ready.sock" {
		t.Fatalf("unexpected bootstrap client target sequence: %#v", newClientTargets)
	}
}

func TestCLIDaemonBootstrapProbeReportsNotReadyHealth(t *testing.T) {
	t.Parallel()

	bootstrap := cliDaemonBootstrap{
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-health.sock",
				StartedAt:  time.Date(2026, 4, 17, 12, 30, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(target apiclient.Target) (daemonCommandClient, error) {
			return &stubDaemonCommandClient{
				target: target,
				health: apicore.DaemonHealth{
					Ready: false,
					Details: []apicore.HealthDetail{{
						Code:    "db_unavailable",
						Message: "global.db is not ready",
					}},
				},
			}, nil
		},
	}

	_, err := bootstrap.probe(context.Background(), "/tmp/compozy-home/daemon.json")
	if err == nil {
		t.Fatal("expected probe failure for not-ready daemon health")
	}
	if got := err.Error(); !containsAll(
		got,
		"probe daemon health via unix:///tmp/compozy-health.sock",
		"global.db is not ready",
	) {
		t.Fatalf("unexpected probe error: %v", err)
	}
}

func TestCLIDaemonBootstrapProbeWrapsReadInfoAndClientErrors(t *testing.T) {
	t.Parallel()

	readInfoErrBootstrap := cliDaemonBootstrap{
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{}, errors.New("daemon info missing")
		},
	}
	_, err := readInfoErrBootstrap.probe(context.Background(), "/tmp/compozy-home/daemon.json")
	if err == nil || !containsAll(err.Error(), "read daemon info", "daemon info missing") {
		t.Fatalf("expected wrapped readInfo error, got %v", err)
	}

	newClientErrBootstrap := cliDaemonBootstrap{
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-health.sock",
				StartedAt:  time.Date(2026, 4, 17, 12, 45, 0, 0, time.UTC),
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			return nil, errors.New("target rejected")
		},
	}
	_, err = newClientErrBootstrap.probe(context.Background(), "/tmp/compozy-home/daemon.json")
	if err == nil || !containsAll(err.Error(), "build daemon client", "target rejected") {
		t.Fatalf("expected wrapped newClient error, got %v", err)
	}
}

func TestCLIDaemonBootstrapEnsureRepairsStaleTransportAfterLaunch(t *testing.T) {
	t.Parallel()

	nowSequence := []time.Time{
		time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 4, 17, 12, 0, 0, 500000000, time.UTC),
	}
	nowIndex := 0
	nextNow := func() time.Time {
		if nowIndex >= len(nowSequence) {
			return nowSequence[len(nowSequence)-1]
		}
		value := nowSequence[nowIndex]
		nowIndex++
		return value
	}

	staleClient := &stubDaemonCommandClient{
		target:    apiclient.Target{SocketPath: "/tmp/compozy-stale.sock"},
		healthErr: errors.New("dial unix /tmp/compozy-stale.sock: connect: no such file or directory"),
	}
	readyClient := &stubDaemonCommandClient{
		target: apiclient.Target{SocketPath: "/tmp/compozy-stale.sock"},
		health: apicore.DaemonHealth{Ready: true},
	}

	var launchCalls int
	var clientCalls int

	bootstrap := cliDaemonBootstrap{
		resolveHomePaths: func() (compozyconfig.HomePaths, error) {
			return compozyconfig.HomePaths{InfoPath: "/tmp/compozy-home/daemon.json"}, nil
		},
		readInfo: func(string) (daemon.Info, error) {
			return daemon.Info{
				PID:        1234,
				SocketPath: "/tmp/compozy-stale.sock",
				StartedAt:  nowSequence[0],
				State:      daemon.ReadyStateReady,
			}, nil
		},
		newClient: func(apiclient.Target) (daemonCommandClient, error) {
			clientCalls++
			if clientCalls == 1 {
				return staleClient, nil
			}
			return readyClient, nil
		},
		launch: func(compozyconfig.HomePaths) error {
			launchCalls++
			return nil
		},
		sleep:          func(time.Duration) {},
		now:            nextNow,
		startupTimeout: 2 * time.Second,
		pollInterval:   time.Millisecond,
	}

	gotClient, err := bootstrap.ensure(context.Background())
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if gotClient != readyClient {
		t.Fatalf("ensure returned unexpected repaired client: %#v", gotClient)
	}
	if launchCalls != 1 {
		t.Fatalf("expected one daemon launch to repair stale transport, got %d", launchCalls)
	}
	if staleClient.healthCalls != 1 {
		t.Fatalf("expected one stale health probe, got %d", staleClient.healthCalls)
	}
	if readyClient.healthCalls != 1 {
		t.Fatalf("expected one repaired health probe, got %d", readyClient.healthCalls)
	}
}

func TestResolveTaskWorkflowName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		flagName      string
		wantName      string
		wantErrSubstr string
	}{
		{
			name:     "positional slug wins when flag is empty",
			args:     []string{"demo"},
			wantName: "demo",
		},
		{
			name:     "name flag works without positional slug",
			flagName: "demo",
			wantName: "demo",
		},
		{
			name:          "positional mismatch is rejected",
			args:          []string{"demo"},
			flagName:      "other",
			wantErrSubstr: "workflow slug mismatch",
		},
		{
			name:          "missing slug is rejected",
			wantErrSubstr: "workflow slug is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindStart, "")
			state.name = tt.flagName

			err := state.resolveTaskWorkflowName(tt.args)
			if tt.wantErrSubstr != "" {
				if err == nil {
					t.Fatal("expected resolveTaskWorkflowName error")
				}
				if !containsAll(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveTaskWorkflowName: %v", err)
			}
			if state.name != tt.wantName {
				t.Fatalf("unexpected resolved workflow name: got %q want %q", state.name, tt.wantName)
			}
		})
	}
}

func TestResolveTaskPresentationModeRejectsConflictsAndInvalidModes(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, "")
	state.isInteractive = func() bool { return true }
	cmd := newTaskRunPresentationCommand(state)
	if err := cmd.Flags().Set("attach", attachModeStream); err != nil {
		t.Fatalf("set attach: %v", err)
	}
	state.attachMode = attachModeStream
	if err := cmd.Flags().Set("ui", "true"); err != nil {
		t.Fatalf("set ui: %v", err)
	}
	if _, err := state.resolveTaskPresentationMode(cmd); err == nil || !containsAll(err.Error(), "choose only one") {
		t.Fatalf("expected conflicting attach mode error, got %v", err)
	}

	state = newCommandState(commandKindStart, "")
	state.isInteractive = func() bool { return true }
	cmd = newTaskRunPresentationCommand(state)
	state.attachMode = "bogus"
	if _, err := state.resolveTaskPresentationMode(cmd); err == nil ||
		!containsAll(err.Error(), "attach mode must be one of auto, ui, stream, or detach") {
		t.Fatalf("expected invalid attach mode error, got %v", err)
	}
}

func TestBuildTaskRunRuntimeOverridesIncludesOnlyExplicitFlags(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, "")
	cmd := newTaskRunPresentationCommand(state)
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "include completed")

	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}
	state.dryRun = true
	if err := cmd.Flags().Set("include-completed", "true"); err != nil {
		t.Fatalf("set include-completed: %v", err)
	}
	state.includeCompleted = true

	raw, err := state.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
	}
	overrides := decodeTaskRunOverrides(t, raw)
	if overrides.DryRun == nil || !*overrides.DryRun {
		t.Fatalf("expected explicit dry-run override, got %#v", overrides)
	}
	if overrides.IncludeCompleted == nil || !*overrides.IncludeCompleted {
		t.Fatalf("expected explicit include-completed override, got %#v", overrides)
	}
	if overrides.AutoCommit != nil || overrides.Model != nil || overrides.Timeout != nil {
		t.Fatalf("expected unset flags to remain absent, got %#v", overrides)
	}
}

func TestBuildTaskRunRuntimeOverridesIncludesAllExplicitRuntimeFlags(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, "")
	cmd := newTaskRunPresentationCommand(state)
	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "include completed")
	cmd.Flags().Var(
		newTaskRuntimeFlagValue(&state.executionTaskRuntimeRules),
		"task-runtime",
		"task runtime",
	)

	mustSetFlag := func(name string, value string) {
		t.Helper()
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("set %s: %v", name, err)
		}
	}

	mustSetFlag("auto-commit", "true")
	state.autoCommit = true
	mustSetFlag("ide", "claude")
	state.ide = "claude"
	mustSetFlag("model", "gpt-5.4")
	state.model = "gpt-5.4"
	mustSetFlag("add-dir", "../shared")
	state.addDirs = []string{"../shared"}
	mustSetFlag("tail-lines", "42")
	state.tailLines = 42
	mustSetFlag("reasoning-effort", "high")
	state.reasoningEffort = "high"
	mustSetFlag("access-mode", "default")
	state.accessMode = "default"
	mustSetFlag("timeout", "2m")
	state.timeout = "2m"
	mustSetFlag("max-retries", "3")
	state.maxRetries = 3
	mustSetFlag("retry-backoff-multiplier", "2.5")
	state.retryBackoffMultiplier = 2.5
	mustSetFlag("task-runtime", "id=task_01,model=gpt-5.4-mini")

	raw, err := state.buildTaskRunRuntimeOverrides(cmd)
	if err != nil {
		t.Fatalf("buildTaskRunRuntimeOverrides: %v", err)
	}
	overrides := decodeTaskRunOverrides(t, raw)

	if overrides.AutoCommit == nil || !*overrides.AutoCommit {
		t.Fatalf("expected auto-commit override, got %#v", overrides)
	}
	if overrides.IDE == nil || *overrides.IDE != "claude" {
		t.Fatalf("expected ide override, got %#v", overrides)
	}
	if overrides.Model == nil || *overrides.Model != "gpt-5.4" {
		t.Fatalf("expected model override, got %#v", overrides)
	}
	if overrides.AddDirs == nil || len(*overrides.AddDirs) != 1 || (*overrides.AddDirs)[0] != "../shared" {
		t.Fatalf("expected add-dir override, got %#v", overrides)
	}
	if overrides.TailLines == nil || *overrides.TailLines != 42 {
		t.Fatalf("expected tail-lines override, got %#v", overrides)
	}
	if overrides.ReasoningEffort == nil || *overrides.ReasoningEffort != "high" {
		t.Fatalf("expected reasoning-effort override, got %#v", overrides)
	}
	if overrides.AccessMode == nil || *overrides.AccessMode != "default" {
		t.Fatalf("expected access-mode override, got %#v", overrides)
	}
	if overrides.Timeout == nil || *overrides.Timeout != "2m" {
		t.Fatalf("expected timeout override, got %#v", overrides)
	}
	if overrides.MaxRetries == nil || *overrides.MaxRetries != 3 {
		t.Fatalf("expected max-retries override, got %#v", overrides)
	}
	if overrides.RetryBackoffMultiplier == nil || *overrides.RetryBackoffMultiplier != 2.5 {
		t.Fatalf("expected retry-backoff-multiplier override, got %#v", overrides)
	}
	if overrides.TaskRuntimeRules == nil || len(*overrides.TaskRuntimeRules) != 1 {
		t.Fatalf("expected task-runtime override, got %#v", overrides)
	}
}

func TestHelpOnlyDaemonCommandRootsReturnHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  *cobra.Command
		want string
	}{
		{
			name: "workspaces",
			cmd:  newWorkspacesCommand(),
			want: "Manage daemon workspace registrations",
		},
		{
			name: "reviews",
			cmd:  newReviewsCommand(),
			want: "Inspect and remediate review workflows",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, err := executeCommandCombinedOutput(tt.cmd, nil)
			if err != nil {
				t.Fatalf("execute %s help root: %v", tt.name, err)
			}
			if !containsAll(output, tt.want) {
				t.Fatalf("unexpected %s help output:\n%s", tt.name, output)
			}
		})
	}
}

func TestMapDaemonCommandErrorUsesStableExitCodes(t *testing.T) {
	t.Parallel()

	assertExitCode := func(t *testing.T, err error, want int) {
		t.Helper()

		var exitErr *commandExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected commandExitError, got %T", err)
		}
		if exitErr.ExitCode() != want {
			t.Fatalf("unexpected exit code: got %d want %d", exitErr.ExitCode(), want)
		}
	}

	conflictErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: 409,
		Envelope: apicore.TransportError{
			RequestID: "req-conflict",
			Code:      "workflow_conflict",
			Message:   "workflow already active",
		},
	})
	assertExitCode(t, conflictErr, 1)

	validationErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: 422,
		Envelope: apicore.TransportError{
			RequestID: "req-invalid",
			Code:      "invalid_request",
			Message:   "invalid workflow request",
		},
	})
	assertExitCode(t, validationErr, 1)

	transportErr := mapDaemonCommandError(fmt.Errorf("dial daemon: %w", errors.New("connection refused")))
	assertExitCode(t, transportErr, 2)

	remoteErr := mapDaemonCommandError(&apiclient.RemoteError{
		StatusCode: 503,
		Envelope: apicore.TransportError{
			RequestID: "req-unavailable",
			Code:      "daemon_unavailable",
			Message:   "daemon unavailable",
		},
	})
	assertExitCode(t, remoteErr, 2)
}
