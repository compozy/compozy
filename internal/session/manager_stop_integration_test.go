//go:build integration && !windows

package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/providers"
	"github.com/compozy/compozy/internal/sandbox/local"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/kballard/go-shellquote"
)

const (
	testSessionStopHelperEnvKey   = "COMPOZY_TEST_SESSION_STOP_HELPER"
	testSessionStopWrapperEnvKey  = "COMPOZY_TEST_SESSION_STOP_WRAPPER"
	testSessionStopWrapperPIDFile = "COMPOZY_TEST_SESSION_STOP_WRAPPER_PID_FILE"
)

func TestSessionStopACPHelperProcess(t *testing.T) {
	if os.Getenv(testSessionStopHelperEnvKey) != "1" {
		return
	}

	if path := os.Getenv("COMPOZY_TEST_STUBBORN_PROMPTS"); path != "" {
		signal.Ignore(syscall.SIGTERM)
		conn := acpsdk.NewAgentSideConnection(stubbornSessionACPAgent{path: path}, os.Stdout, os.Stdin)
		<-conn.Done()
		select {} // This fixture requires verified process killing even after stdin closes.
	}
	conn := acpsdk.NewAgentSideConnection(sessionStopACPAgent{}, os.Stdout, os.Stdin)
	<-conn.Done()
	os.Exit(0)
}

func TestSessionStopACPWrapperProcess(t *testing.T) {
	if os.Getenv(testSessionStopWrapperEnvKey) != "1" {
		return
	}

	bin, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}

	cmd := exec.Command(bin, "-test.run=TestSessionStopACPHelperProcess")
	cmd.Env = append([]string(nil), os.Environ()...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		os.Exit(1)
	}

	if pidFile := strings.TrimSpace(os.Getenv(testSessionStopWrapperPIDFile)); pidFile != "" {
		if writeErr := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644); writeErr != nil {
			if killErr := cmd.Process.Kill(); killErr != nil {
				os.Exit(1)
			}
			os.Exit(1)
		}
	}

	if err := cmd.Wait(); err != nil {
		os.Exit(1)
	}

	os.Exit(0)
}

func TestManagerIntegrationStopFinalizesWrappedACPProcess(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "wrapped-helper.pid")

	h := newHarness(t)
	command := sessionStopWrapperCommand(t, pidFile)
	h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
	driver := newIntegrationACPDriver(
		acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		acp.WithStopTimeout(100*time.Millisecond),
	)
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: acpmock.ProviderName,
			Command:  command,
			Prompt:   "You are a coding assistant.",
		}},
	})
	h.manager = newManagerWithHarness(t, h, WithDriver(NewACPDriverAdapter(driver)))

	session := createSession(t, h)
	childPID := waitForSessionStopWrapperChildPID(t, pidFile)

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := h.manager.Stop(stopCtx, session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	waitForSessionStopProcessExit(t, childPID, time.Second)
	h.notifier.waitForStopped(t, session.ID)
	if got := readMeta(t, session.MetaPath()).State; got != string(StateStopped) {
		t.Fatalf("meta.State = %q, want %q", got, StateStopped)
	}

	meta := readMeta(t, session.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}
}

func TestManagerIntegrationSupervisedWorkRecovery(t *testing.T) {
	t.Parallel()
	t.Run("Should recover owned work only after the frozen process tree exits", func(t *testing.T) {
		t.Parallel()
		base := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
		var clock atomic.Int64
		clock.Store(base.UnixNano())
		pidFile := filepath.Join(t.TempDir(), "child.pid")
		h := newRealACPIntegrationHarness(t, sessionStopWrapperCommand(t, pidFile))
		config := testSupervisionConfig()
		config.QuietAfter, config.StopGrace = time.Second, time.Second
		h.manager = newManagerWithHarness(t, h, WithDriver(h.manager.driver),
			WithNow(func() time.Time { return time.Unix(0, clock.Load()).UTC() }),
			WithSessionSupervision(config),
			WithSessionStopConfig(compozyconfig.SessionStopConfig{CooperativeGrace: 20 * time.Millisecond}))
		db := openManagerInputQueueStore(t)
		registerManagerInputQueueWorkspace(t, db, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, db, h, sess)
		proc := sess.processHandle()
		childPID := waitForSessionStopWrapperChildPID(t, pidFile)
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), sess.ID); err != nil {
				t.Errorf("cleanup Stop: %v", err)
			}
		})
		tasks, source := seedSupervisedTaskForSession(t, db, sess, base)
		artifact := filepath.Join(h.workspace, "committed-output.txt")
		if err := os.WriteFile(artifact, []byte("part-1\npart-2\npart-3\npart-4\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		h.manager.SetSupervisedWorkRecovery(func(ctx context.Context, info *Info, eventID string) error {
			if sessionStopProcessAlive(proc.PID) || sessionStopProcessAlive(childPID) {
				return fmt.Errorf("recovery ran before process tree exit")
			}
			return tasks.RecoverSupervisedWork(ctx, taskpkg.SupervisedStop{
				SessionID: info.ID, WorkspaceID: info.WorkspaceID, ProfileID: info.ProfileID, EventID: eventID,
			})
		})
		installAbsentWorkSources(h.manager)
		if err := syscall.Kill(-proc.PID, syscall.SIGSTOP); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		for tick := range 3 {
			at := base.Add(time.Duration(10+tick) * time.Second)
			clock.Store(at.UnixNano())
			if err := h.manager.Supervise(ctx, at); err != nil {
				t.Fatal(err)
			}
		}
		outcome, err := h.manager.AwaitStopped(ctx, sess.ID)
		if err != nil || !outcome.Verified || outcome.Cause != CauseInactivity {
			t.Fatalf("supervised stop = %#v, %v", outcome, err)
		}
		runs, err := db.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: source.TaskID})
		if err != nil || len(runs) != 2 {
			t.Fatalf("recovered runs = %#v, %v", runs, err)
		}
		for _, run := range runs {
			if run.ID != source.ID && (run.PreviousRunID != source.ID || run.Attempt != 2 ||
				run.Status != taskpkg.TaskRunStatusQueued || run.SessionID != "") {
				t.Fatalf("successor = %#v", run)
			}
		}
		content, err := os.ReadFile(artifact)
		if err != nil || string(content) != "part-1\npart-2\npart-3\npart-4\n" {
			t.Fatalf("committed output = %q, %v", content, err)
		}
		assertSupervisionEventCorrelation(t, h.manager, sess, "session.supervision_stopped")
	})
}

func seedSupervisedTaskForSession(
	t *testing.T, db *globaldb.GlobalDB, sess *Session, at time.Time,
) (*taskpkg.Service, taskpkg.Run) {
	t.Helper()
	tasks, err := taskpkg.NewManager(taskpkg.WithStore(db), taskpkg.WithManagerNow(func() time.Time { return at }))
	if err != nil {
		t.Fatal(err)
	}
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "test")
	if err != nil {
		t.Fatal(err)
	}
	record, err := tasks.CreateTask(t.Context(), taskpkg.CreateTask{
		ProfileID: sess.Info().ProfileID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: sess.WorkspaceID,
		Title: "Recover committed work", MaxAttempts: new(3),
	}, actor)
	if err != nil {
		t.Fatal(err)
	}
	run, err := tasks.EnqueueRun(t.Context(), taskpkg.EnqueueRun{TaskID: record.ID}, actor)
	if err != nil {
		t.Fatal(err)
	}
	agent, err := taskpkg.DeriveAgentSessionActorContext(sess.ID, sess.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	claim, err := tasks.ClaimNextRun(t.Context(), taskpkg.ClaimCriteria{
		RunID: run.ID, Scope: taskpkg.ScopeWorkspace, WorkspaceID: sess.WorkspaceID,
		ClaimerSessionID: sess.ID, LeaseDuration: time.Second, Now: at.Add(-time.Minute),
	}, agent)
	if err != nil {
		t.Fatal(err)
	}
	return tasks, claim.Run
}

func TestManagerIntegrationAllowedToolsOverrideNarrowsAcpmockSession(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and expose the narrowed allowed-tools policy", func(t *testing.T) {
		t.Parallel()

		driverPath := acpmock.RequireDriver(t)
		fixturePath, err := filepath.Abs(filepath.Join(
			"..",
			"testutil",
			"acpmock",
			"testdata",
			"hosted_native_tools_fixture.json",
		))
		if err != nil {
			t.Fatalf("Abs(fixture) error = %v", err)
		}
		diagnosticsPath := filepath.Join(t.TempDir(), "acpmock-diagnostics.jsonl")
		command := acpmock.BuildCommand(driverPath, fixturePath, "hosted-native", diagnosticsPath)

		h := newHostedMCPHarness(
			t,
			WithDriver(NewACPDriverAdapter(newIntegrationACPDriver(
				acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
			))),
		)
		h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(driverPath)
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		resolved.Config = h.cfg
		resolved.Agents = []compozyconfig.AgentDef{{
			Name:     "acpmock-tools",
			Provider: acpmock.ProviderName,
			Command:  command,
			Prompt:   "You are an acpmock tool policy agent.",
			Tools: []string{
				toolspkg.ToolIDTaskRead.String(),
				toolspkg.ToolIDTaskUpdate.String(),
			},
		}}
		h.resolver.upsert(&resolved)

		sess, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "acpmock-tools",
			Workspace: h.workspaceID,
			AllowedToolsOverride: []string{
				toolspkg.ToolIDTaskRead.String(),
			},
		})
		if err != nil {
			t.Fatalf("Create(acpmock narrowed tools) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Fatalf("Stop(acpmock session) error = %v", err)
			}
		})

		info := sess.Info()
		if info.Lineage == nil {
			t.Fatal("session lineage = nil, want narrowed tool policy")
		}
		if got, want := info.Lineage.PermissionPolicy.Tools, []string{
			toolspkg.ToolIDTaskRead.String(),
		}; !testutil.EqualStringSlices(
			got,
			want,
		) {
			t.Fatalf("lineage policy tools = %#v, want %#v", got, want)
		}
		meta := readMeta(t, sess.MetaPath())
		if meta.Lineage == nil {
			t.Fatal("persisted lineage = nil, want narrowed tool policy")
		}
		if got, want := meta.Lineage.PermissionPolicy.Tools, []string{
			toolspkg.ToolIDTaskRead.String(),
		}; !testutil.EqualStringSlices(
			got,
			want,
		) {
			t.Fatalf("persisted policy tools = %#v, want %#v", got, want)
		}
	})
}

func TestManagerIntegrationResumeReplayRestoresLoadUnsupportedContext(t *testing.T) {
	t.Parallel()

	driverPath := acpmock.RequireDriver(t)
	fixturePath, err := filepath.Abs(filepath.Join(
		"..",
		"testutil",
		"acpmock",
		"testdata",
		"resume_replay_fixture.json",
	))
	if err != nil {
		t.Fatalf("Abs(fixture) error = %v", err)
	}
	diagnosticsPath := filepath.Join(t.TempDir(), "resume-replay-diagnostics.jsonl")
	command := acpmock.BuildCommand(driverPath, fixturePath, "resume-replay", diagnosticsPath)

	h := newHarness(t)
	h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(driverPath)
	resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
	}
	resolved.Config = h.cfg
	resolved.Agents = []compozyconfig.AgentDef{{
		Name:     "resume-replay",
		Provider: acpmock.ProviderName,
		Command:  command,
		Prompt:   "You are a deterministic resume replay agent.",
	}}
	h.resolver.upsert(&resolved)
	newRuntimeDriver := func() AgentDriver {
		return NewACPDriverAdapter(newIntegrationACPDriver(
			acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		))
	}
	h.manager = newManagerWithHarness(t, h, WithDriver(newRuntimeDriver()))

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "resume-replay",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	firstPrompt, err := h.manager.Prompt(
		testutil.Context(t),
		session.ID,
		"Remember that the recovery code is cobalt.",
	)
	if err != nil {
		t.Fatalf("Prompt(before restart) error = %v", err)
	}
	collectEvents(t, firstPrompt)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop(before restart) error = %v", err)
	}
	eventsBeforeResume := readStoredEvents(t, session)
	if err := h.manager.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown(before manager restart) error = %v", err)
	}
	checkpoint := "<compozy_checkpoint_summary>\n## Goal\nPreserve the cobalt decision.\n</compozy_checkpoint_summary>"
	h.manager = newManagerWithHarness(
		t,
		h,
		WithDriver(newRuntimeDriver()),
		WithPromptAssembler(&resumeContextPromptAssembler{checkpoint: checkpoint}),
	)

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(load unsupported) error = %v", err)
	}
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil &&
			!errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("Stop(resumed) error = %v", err)
		}
	})

	secondPrompt, err := h.manager.Prompt(
		testutil.Context(t),
		resumed.ID,
		"What was the recovery code?",
	)
	if err != nil {
		t.Fatalf("Prompt(after restart) error = %v", err)
	}
	secondEvents := collectEvents(t, secondPrompt)
	if !slices.ContainsFunc(secondEvents, func(event acp.AgentEvent) bool {
		return event.Type == acp.EventTypeAgentMessage && strings.Contains(event.Text, "cobalt")
	}) {
		t.Fatalf("resumed prompt events = %#v, want pre-restart recovery code", secondEvents)
	}

	records, err := acpmock.ReadDiagnostics(diagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics() error = %v", err)
	}
	prompts := acpmock.PromptDiagnostics(acpmock.DiagnosticsForCompozySession(records, resumed.ID))
	if got, want := len(prompts), 2; got != want {
		t.Fatalf("resume replay prompt diagnostics = %#v, want %d prompt records", prompts, want)
	}
	checkpointIndex := strings.Index(prompts[1].Prompt, "<compozy_checkpoint_summary>")
	replayIndex := strings.Index(prompts[1].Prompt, resumeReplayOpenTag)
	if checkpointIndex < 0 || replayIndex < 0 || checkpointIndex >= replayIndex {
		t.Fatalf("resume replay checkpoint ordering invalid:\n%s", prompts[1].Prompt)
	}
	assertResumeReplayEqualsPrunedEvents(t, prompts[1].Prompt, eventsBeforeResume)
	assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 1)
}

func TestManagerIntegrationKillProcessPersistsAgentCrashedStopReason(t *testing.T) {
	h := newHarness(t)
	driver := newIntegrationACPDriver(
		acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		acp.WithStopTimeout(100*time.Millisecond),
	)
	command := sessionStopHelperCommand(t)
	h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: acpmock.ProviderName,
			Command:  command,
			Prompt:   "You are a coding assistant.",
		}},
	})
	h.manager = newManagerWithHarness(t, h, WithDriver(NewACPDriverAdapter(driver)))

	session := createSession(t, h)
	proc := session.processHandle()
	if proc == nil {
		t.Fatal("session process = nil, want ACP process")
	}
	if err := syscall.Kill(proc.PID, syscall.SIGKILL); err != nil {
		t.Fatalf("syscall.Kill(%d, SIGKILL) error = %v", proc.PID, err)
	}

	h.notifier.waitForStopped(t, session.ID)

	meta := readMeta(t, session.MetaPath())
	if *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopAgentCrashed)
	}
}

func TestManagerIntegrationCreateAndResumeWithWorkspaceResolver(t *testing.T) {
	t.Run("Should create and resume with a resolved workspace", func(t *testing.T) {
		t.Parallel()

		homePaths, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}

		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspace root) error = %v", err)
		}

		command := sessionStopHelperCommand(t)
		writeSessionIntegrationAgentDef(t, homePaths, "coder", command)

		registry, err := openSessionTestGlobalDB(context.Background(), homePaths.DatabaseFile)
		if err != nil {
			t.Fatalf("OpenGlobalDB() error = %v", err)
		}
		t.Cleanup(func() {
			if err := registry.Close(context.Background()); err != nil {
				t.Fatalf("registry.Close() error = %v", err)
			}
		})

		cfg := compozyconfig.DefaultWithHome(homePaths)
		cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)

		resolver, err := workspacepkg.NewResolver(
			registry,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
			workspacepkg.WithConfigLoader(func(string) (compozyconfig.Config, error) { return cfg, nil }),
			workspacepkg.WithProfileConfigLoader(
				func(string, string) (compozyconfig.Config, error) { return cfg, nil },
			),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}

		driver := newIntegrationACPDriver(acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
		sandboxRegistry, err := local.NewRegistry(local.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
		if err != nil {
			t.Fatalf("local.NewRegistry() error = %v", err)
		}
		manager, err := NewManager(
			WithHomePaths(homePaths),
			WithWorkspaceResolver(resolver),
			WithDriver(NewACPDriverAdapter(driver)),
			WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
			WithSandboxRegistry(sandboxRegistry),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		cleanupTestManager(t, manager)

		session, err := manager.Create(testutil.Context(t), CreateOpts{
			AgentName:     "coder",
			WorkspacePath: workspaceRoot,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		workspaceID := session.Info().WorkspaceID
		if workspaceID == "" {
			t.Fatal("Create() workspace id = empty, want resolved workspace id")
		}
		canonicalWorkspaceRoot := resolveIntegrationWorkspaceRoot(t, workspaceRoot)
		if got, want := session.Info().Workspace, canonicalWorkspaceRoot; got != want {
			t.Fatalf("Create() workspace root = %q, want %q", got, want)
		}
		events, err := manager.Prompt(testutil.Context(t), session.ID, "integration prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		for range events {
		}

		if err := manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		waitForStoppedSession(t, manager, session)

		resumed, err := manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Stop(testutil.Context(t), resumed.ID); err != nil {
				t.Fatalf("cleanup Stop() error = %v", err)
			}
			waitForStoppedSession(t, manager, resumed)
		})

		if got := resumed.Info().WorkspaceID; got != workspaceID {
			t.Fatalf("Resume() workspace id = %q, want %q", got, workspaceID)
		}
		if got, want := resumed.Info().Workspace, canonicalWorkspaceRoot; got != want {
			t.Fatalf("Resume() workspace root = %q, want %q", got, want)
		}
		if got := readMeta(t, resumed.MetaPath()).WorkspaceID; got != workspaceID {
			t.Fatalf("meta workspace id = %q, want %q", got, workspaceID)
		}
	})
}

func TestManagerIntegrationCrashRecoveryRejectsDeadRuntimeAttachment(t *testing.T) {
	h := newRealACPIntegrationHarness(t, sessionStopHelperCommand(t))

	session := createSession(t, h)
	events, err := h.manager.Prompt(testutil.Context(t), session.ID, "bind before crash")
	if err != nil {
		t.Fatalf("Prompt(): %v", err)
	}
	_ = collectEvents(t, events)
	meta := readMeta(t, session.MetaPath())
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, session)

	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	h.manager = newManagerWithHarness(t, h, WithDriver(NewACPDriverAdapter(newIntegrationACPDriver(
		acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		acp.WithStopTimeout(100*time.Millisecond),
	))))
	cleanupTestManager(t, h.manager)
	if err := h.manager.RecoverPendingStops(testutil.Context(t)); err != nil {
		t.Fatalf("RecoverPendingStops(): %v", err)
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); !errors.Is(err, store.ErrSessionNotAttachable) {
		t.Fatalf("Resume(crashed runtime) error = %v, want ErrSessionNotAttachable", err)
	}
	if _, exists := h.manager.Get(session.ID); exists {
		t.Fatal("Resume materialized a dead runtime")
	}
	meta = readMeta(t, session.MetaPath())
	if meta.StopReason == nil || *meta.StopReason != store.StopAgentCrashed ||
		meta.StopDetail != resumeStopDetailAgentCrashed {
		t.Fatalf(
			"crash classification = %v/%q, want agent_crashed/%q",
			meta.StopReason,
			meta.StopDetail,
			resumeStopDetailAgentCrashed,
		)
	}
}

func TestManagerIntegrationResumeFailsWhenWorkspaceDirectoryMissing(t *testing.T) {
	h := newRealACPIntegrationHarness(t, sessionStopHelperCommand(t))

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, session)
	if err := os.RemoveAll(h.workspace); err != nil {
		t.Fatalf("os.RemoveAll(%q) error = %v", h.workspace, err)
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(missing workspace dir) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), h.workspace) {
		t.Fatalf("Resume(missing workspace dir) error = %v, want workspace path %q", err, h.workspace)
	}
}

func TestManagerIntegrationResumeFailsWhenAgentRemoved(t *testing.T) {
	h := newRealACPIntegrationHarness(t, sessionStopHelperCommand(t))

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, session)

	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     compozyconfig.DefaultAgentName,
			Provider: acpmock.ProviderName,
			Command:  sessionStopHelperCommand(t),
			Prompt:   "You are a coding assistant.",
		}},
	})

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(missing agent) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), "coder") {
		t.Fatalf("Resume(missing agent) error = %v, want agent name", err)
	}
}

func TestManagerIntegrationResumeFailsWhenEventStoreIsEmpty(t *testing.T) {
	h := newRealACPIntegrationHarness(t, sessionStopHelperCommand(t))

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, session)
	if err := os.WriteFile(session.DBPath(), nil, 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", session.DBPath(), err)
	}

	if _, err := h.manager.Resume(testutil.Context(t), session.ID); err == nil {
		t.Fatal("Resume(empty event store) error = nil, want non-nil")
	} else if !strings.Contains(err.Error(), session.DBPath()) || !strings.Contains(err.Error(), "file is empty") {
		t.Fatalf("Resume(empty event store) error = %v, want db path and empty-file detail", err)
	}
}

func TestManagerIntegrationFullStopResumeStopPersistsStopReasons(t *testing.T) {
	h := newRealACPIntegrationHarness(t, sessionStopHelperCommand(t))

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, session)

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, resumed)

	meta := readMeta(t, resumed.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}

	events := readStoredEvents(t, resumed)
	if got := countEventType(events, EventTypeSessionStopped); got != 2 {
		t.Fatalf("session_stopped events = %d, want 2", got)
	}
	stopReasons := make([]string, 0, 2)
	for _, event := range events {
		if event.Type != EventTypeSessionStopped {
			continue
		}
		payload := decodeStoredEventPayload(t, event)
		stopReasons = append(stopReasons, payload["stop_reason"].(string))
	}
	if got, want := len(stopReasons), 2; got != want {
		t.Fatalf("stop reason payload count = %d, want %d", got, want)
	}
	for index, reason := range stopReasons {
		if reason != string(store.StopUserCanceled) {
			t.Fatalf("stop reason payload %d = %q, want %q", index, reason, store.StopUserCanceled)
		}
	}
}

func newRealACPIntegrationHarness(t *testing.T, command string) *harness {
	t.Helper()

	h := newHarness(t)
	h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)
	driver := newIntegrationACPDriver(
		acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		acp.WithStopTimeout(100*time.Millisecond),
	)
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{{
			Name:     "coder",
			Provider: acpmock.ProviderName,
			Command:  command,
			Prompt:   "You are a coding assistant.",
		}},
	})
	h.manager = newManagerWithHarness(t, h, WithDriver(NewACPDriverAdapter(driver)))
	return h
}

func newIntegrationACPDriver(options ...acp.Option) *acp.Driver {
	base := []acp.Option{acp.WithProviderPreStarter(providers.NewPreStarter())}
	return acp.New(append(base, options...)...)
}

func waitForStoppedSession(t *testing.T, manager *Manager, sess *Session) {
	t.Helper()
	if _, ok := manager.Get(sess.ID); ok {
		t.Fatalf("Get(%q) found session after Stop returned", sess.ID)
	}
	if got := readMeta(t, sess.MetaPath()).State; got != string(StateStopped) {
		t.Fatalf("meta.State = %q, want %q", got, StateStopped)
	}
}

func sessionStopWrapperCommand(t *testing.T, pidFile string) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	return shellquote.Join(
		"env",
		testSessionStopHelperEnvKey+"=1",
		testSessionStopWrapperEnvKey+"=1",
		testSessionStopWrapperPIDFile+"="+pidFile,
		bin,
		"-test.run=TestSessionStopACPWrapperProcess",
	)
}

func sessionStopHelperCommand(t *testing.T) string {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	return shellquote.Join(
		"env",
		testSessionStopHelperEnvKey+"=1",
		bin,
		"-test.run=TestSessionStopACPHelperProcess",
	)
}

func writeSessionIntegrationAgentDef(t *testing.T, homePaths compozyconfig.HomePaths, name string, command string) {
	t.Helper()

	path := filepath.Join(homePaths.AgentsDir, name, "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(agent dir) error = %v", err)
	}
	contents := strings.Join([]string{
		"---",
		"name: " + name,
		"provider: " + acpmock.ProviderName,
		"command: " + command,
		"---",
		"You are a coding assistant.",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(agent def) error = %v", err)
	}
}

func resolveIntegrationWorkspaceRoot(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return normalizeResolverPath(path)
	}
	return normalizeResolverPath(resolved)
}

func waitForSessionStopWrapperChildPID(t *testing.T, path string) int {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			text := strings.TrimSpace(string(data))
			if text == "" {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			pid, convErr := strconv.Atoi(text)
			if convErr != nil {
				t.Fatalf("strconv.Atoi(%q) error = %v", string(data), convErr)
			}
			if pid <= 0 {
				t.Fatalf("wrapper child pid = %d, want > 0", pid)
			}
			return pid
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.ReadFile(%q) error = %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for wrapper child pid file %q", path)
	return 0
}

func waitForSessionStopProcessExit(t *testing.T, pid int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !sessionStopProcessAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("process %d is still alive after %v", pid, timeout)
}

func sessionStopProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

type sessionStopACPAgent struct{}

func (sessionStopACPAgent) Authenticate(
	context.Context,
	acpsdk.AuthenticateRequest,
) (acpsdk.AuthenticateResponse, error) {
	return acpsdk.AuthenticateResponse{}, nil
}

func (sessionStopACPAgent) Logout(context.Context, acpsdk.LogoutRequest) (acpsdk.LogoutResponse, error) {
	return acpsdk.LogoutResponse{}, nil
}

func (sessionStopACPAgent) Initialize(context.Context, acpsdk.InitializeRequest) (acpsdk.InitializeResponse, error) {
	return acpsdk.InitializeResponse{
		ProtocolVersion: acpsdk.ProtocolVersionNumber,
		AgentCapabilities: acpsdk.AgentCapabilities{
			LoadSession: true,
		},
		AuthMethods: []acpsdk.AuthMethod{},
	}, nil
}

func (sessionStopACPAgent) Cancel(context.Context, acpsdk.CancelNotification) error {
	return nil
}

func (sessionStopACPAgent) CloseSession(
	context.Context,
	acpsdk.CloseSessionRequest,
) (acpsdk.CloseSessionResponse, error) {
	return acpsdk.CloseSessionResponse{}, nil
}

func (sessionStopACPAgent) ListSessions(
	context.Context,
	acpsdk.ListSessionsRequest,
) (acpsdk.ListSessionsResponse, error) {
	return acpsdk.ListSessionsResponse{Sessions: []acpsdk.SessionInfo{}}, nil
}

func (sessionStopACPAgent) NewSession(context.Context, acpsdk.NewSessionRequest) (acpsdk.NewSessionResponse, error) {
	return acpsdk.NewSessionResponse{
		SessionId: "sess-stop-helper",
	}, nil
}

func (sessionStopACPAgent) LoadSession(context.Context, acpsdk.LoadSessionRequest) (acpsdk.LoadSessionResponse, error) {
	return acpsdk.LoadSessionResponse{}, nil
}

func (sessionStopACPAgent) ResumeSession(
	context.Context,
	acpsdk.ResumeSessionRequest,
) (acpsdk.ResumeSessionResponse, error) {
	return acpsdk.ResumeSessionResponse{}, nil
}

func (sessionStopACPAgent) Prompt(context.Context, acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	return acpsdk.PromptResponse{
		StopReason: acpsdk.StopReasonEndTurn,
	}, nil
}

func (sessionStopACPAgent) SetSessionMode(
	context.Context,
	acpsdk.SetSessionModeRequest,
) (acpsdk.SetSessionModeResponse, error) {
	return acpsdk.SetSessionModeResponse{}, nil
}

func (sessionStopACPAgent) SetSessionConfigOption(
	context.Context,
	acpsdk.SetSessionConfigOptionRequest,
) (acpsdk.SetSessionConfigOptionResponse, error) {
	return acpsdk.SetSessionConfigOptionResponse{ConfigOptions: []acpsdk.SessionConfigOption{}}, nil
}

// stubbornSessionACPAgent blocks only the first durable prompt across process restarts.
// Routing uses the admission count, independent of rendered prompt text.
type stubbornSessionACPAgent struct {
	sessionStopACPAgent
	path string
}

type stubbornSessionPrompt struct {
	PID  int    `json:"pid"`
	Text string `json:"text"`
}

func (a stubbornSessionACPAgent) Prompt(_ context.Context, req acpsdk.PromptRequest) (acpsdk.PromptResponse, error) {
	file, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0o600)
	if err != nil {
		return acpsdk.PromptResponse{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return acpsdk.PromptResponse{}, errors.Join(err, file.Close())
	}
	var text strings.Builder
	for _, block := range req.Prompt {
		if block.Text != nil {
			text.WriteString(block.Text.Text)
		}
	}
	err = json.NewEncoder(file).Encode(stubbornSessionPrompt{PID: os.Getpid(), Text: text.String()})
	if err := errors.Join(err, file.Close()); err != nil {
		return acpsdk.PromptResponse{}, err
	}
	if info.Size() == 0 {
		// This process fixture must ignore request cancellation as well as SIGTERM.
		// Only verified process killing may release its first prompt.
		select {}
	}
	return acpsdk.PromptResponse{StopReason: acpsdk.StopReasonEndTurn}, nil
}

func TestManagerIntegrationSteerFallback(t *testing.T) {
	t.Run("Should escalate an ignored cancellation and rebind before ordered replacement dispatch", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), "prompts.jsonl")
		command := shellquote.Join("env", "COMPOZY_TEST_STUBBORN_PROMPTS="+path) + " " + sessionStopHelperCommand(t)
		h := newRealACPIntegrationHarness(t, command)
		queue := openManagerInputQueueStore(t)
		h.manager = newManagerWithHarness(t, h, WithDriver(h.manager.driver), WithSessionInputQueueStore(queue),
			WithSessionStopConfig(compozyconfig.SessionStopConfig{CooperativeGrace: 20 * time.Millisecond}))
		registerManagerInputQueueWorkspace(t, queue, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queue, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(context.Background(), sess.ID); err != nil {
				t.Errorf("cleanup Stop: %v", err)
			}
		})
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		active, err := h.manager.SendPrompt(ctx, sess.ID, SendPromptOpts{Message: "active prompt"})
		if err != nil {
			t.Fatal(err)
		}
		first := waitForStubbornSessionPrompts(ctx, t, path, 1)
		turnID := sess.CurrentTurnID()
		if _, err := h.manager.SendPrompt(
			ctx,
			sess.ID,
			SendPromptOpts{Message: "queued later", Mode: BusyInputModeQueue},
		); err != nil {
			t.Fatal(err)
		}
		result, err := h.manager.SendPrompt(
			ctx,
			sess.ID,
			SendPromptOpts{Message: "change direction", Mode: BusyInputModeSteer},
		)
		if err != nil || result.SteerDelivery != store.SteerDeliveryInterruptFallback {
			t.Fatalf("fallback = %#v, %v", result, err)
		}
		outcome, err := h.manager.AwaitTurnQuiesced(ctx, sess.ID, turnID)
		if err != nil || !outcome.Quiesced || !outcome.Escalated {
			t.Fatalf("cancel outcome = %#v, %v", outcome, err)
		}
		prompts := waitForStubbornSessionPrompts(ctx, t, path, 3)
		collectEvents(t, active.Events)
		if err := h.manager.WaitForPromptDrains(ctx); err != nil {
			t.Fatal(err)
		}
		if len(prompts) != 3 || !promptRequestEndsWith(prompts[1].Text, "change direction") ||
			!promptRequestEndsWith(prompts[2].Text, "queued later") {
			t.Fatalf("prompt order = %#v", prompts)
		}
		if prompts[1].PID == first[0].PID || prompts[2].PID != prompts[1].PID {
			t.Fatalf("process rebind = %#v", prompts)
		}
		waitForSessionStopProcessExit(t, first[0].PID, time.Second)
		if sess.Info().State != StateActive || sess.IsPrompting() {
			t.Fatalf("session after replacement = %#v", sess.Info())
		}
		if got := readMeta(t, sess.MetaPath()).State; got != string(StateActive) {
			t.Fatalf("persisted state = %s", got)
		}
	})
}

func waitForStubbornSessionPrompts(ctx context.Context, t *testing.T, path string, count int) []stubbornSessionPrompt {
	t.Helper()
	for {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		var prompts []stubbornSessionPrompt
		for line := range strings.SplitSeq(string(data), "\n") {
			if line == "" {
				continue
			}
			var prompt stubbornSessionPrompt
			if err := json.Unmarshal([]byte(line), &prompt); err != nil {
				break
			} // Writer may still be appending the final line.
			prompts = append(prompts, prompt)
		}
		if len(prompts) >= count {
			return prompts
		}
		select {
		case <-ctx.Done():
			t.Fatalf("waiting for %d subprocess prompts: got %d: %v", count, len(prompts), ctx.Err())
			return nil
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestManagerIntegrationSpawnProviderCommandRouting(t *testing.T) {
	t.Parallel()
	t.Run("Should launch inherited and explicit native routes and preserve routing on resume", func(t *testing.T) {
		t.Parallel()
		routes := t.TempDir()
		commands := map[string]string{}
		for _, account := range []string{"a", "b", "c"} {
			directory := filepath.Join(routes, account)
			if err := os.MkdirAll(directory, 0o700); err != nil {
				t.Fatal(err)
			}
			wrapper := filepath.Join(directory, "provider")
			script := "#!/bin/sh\n" + "touch " + shellquote.Join(
				directory,
			) + "/\"$COMPOZY_SESSION_ID\"\nexec " + sessionStopHelperCommand(
				t,
			) + "\n"
			if err := os.WriteFile(wrapper, []byte(script), 0o700); err != nil {
				t.Fatal(err)
			}
			commands[account] = shellquote.Join(wrapper)
		}
		h := newRealACPIntegrationHarness(t, commands["a"])
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		provider := acpmock.ProviderConfig(commands["a"])
		provider.AuthMode = compozyconfig.ProviderAuthModeNativeCLI
		provider.NoneSecurity = ""
		workspace.Config.Providers[acpmock.ProviderName] = provider
		workspace.Agents = []compozyconfig.AgentDef{
			{Name: "parent", Provider: acpmock.ProviderName, Command: commands["b"], Prompt: "Delegate."},
			{Name: "child", Provider: acpmock.ProviderName, Prompt: "Review."},
			{Name: "explicit", Provider: acpmock.ProviderName, Command: commands["c"], Prompt: "Review."},
		}
		h.resolver.upsert(&workspace)
		parent, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "parent", Workspace: h.workspaceID})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, parent.ID) })
		workspace.Agents[0].Command = commands["c"]
		h.resolver.upsert(&workspace)
		child, err := h.manager.Spawn(
			t.Context(),
			SpawnOpts{ParentSessionID: parent.ID, AgentName: "child", TTL: time.Minute},
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, child.ID) })
		explicit, err := h.manager.Spawn(
			t.Context(),
			SpawnOpts{ParentSessionID: parent.ID, AgentName: "explicit", TTL: time.Minute},
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { stopActiveIntegrationSession(t, h, explicit.ID) })
		for _, route := range []struct{ sessionID, account string }{{parent.ID, "b"}, {child.ID, "b"}, {explicit.ID, "c"}} {
			if _, err := os.Stat(filepath.Join(routes, route.account, route.sessionID)); err != nil {
				t.Fatalf("provider route %s/%s was not executed: %v", route.account, route.sessionID, err)
			}
			if _, err := os.Stat(filepath.Join(routes, "a", route.sessionID)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("global route unexpectedly executed for %s: %v", route.sessionID, err)
			}
		}
		if err := h.manager.Stop(t.Context(), child.ID); err != nil {
			t.Fatal(err)
		}
		resumed, err := h.manager.Resume(t.Context(), child.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := resumed.providerRoutingSnapshot().Command; got != commands["b"] {
			t.Fatalf("resumed command = %q, want creator launch snapshot", got)
		}
		if err := h.manager.Stop(t.Context(), child.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Stop(t.Context(), parent.ID); err != nil {
			t.Fatal(err)
		}
		workspace.Agents[0].Command = commands["b"]
		h.resolver.upsert(&workspace)
		resumed, err = h.manager.Resume(t.Context(), child.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := resumed.providerRoutingSnapshot().Command; got != commands["b"] {
			t.Fatalf("resumed command = %q, want persisted creator route resolution", got)
		}
	})
}
