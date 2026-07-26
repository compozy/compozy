//go:build integration && !windows

package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox/local"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/testutil/acpmock"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/kballard/go-shellquote"
)

const (
	testSessionStopHelperEnvKey   = "AGH_TEST_SESSION_STOP_HELPER"
	testSessionStopWrapperEnvKey  = "AGH_TEST_SESSION_STOP_WRAPPER"
	testSessionStopWrapperPIDFile = "AGH_TEST_SESSION_STOP_WRAPPER_PID_FILE"
)

func TestSessionStopACPHelperProcess(t *testing.T) {
	if os.Getenv(testSessionStopHelperEnvKey) != "1" {
		return
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
	driver := acp.New(
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
		Agents: []aghconfig.AgentDef{{
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
	waitForCondition(t, "stopped session metadata", func() bool {
		meta := readMeta(t, session.MetaPath())
		return meta.State == string(StateStopped)
	})

	meta := readMeta(t, session.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}
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

		h := newHarness(t)
		h.cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(driverPath)
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", h.workspaceID, err)
		}
		resolved.Config = h.cfg
		resolved.Agents = []aghconfig.AgentDef{{
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
		h.manager = newManagerWithHarness(
			t,
			h,
			WithDriver(NewACPDriverAdapter(acp.New(acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))))),
		)

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
	resolved.Agents = []aghconfig.AgentDef{{
		Name:     "resume-replay",
		Provider: acpmock.ProviderName,
		Command:  command,
		Prompt:   "You are a deterministic resume replay agent.",
	}}
	h.resolver.upsert(&resolved)
	newRuntimeDriver := func() AgentDriver {
		return NewACPDriverAdapter(acp.New(acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))))
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
	checkpoint := "<agh_checkpoint_summary>\n## Goal\nPreserve the cobalt decision.\n</agh_checkpoint_summary>"
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
	prompts := acpmock.PromptDiagnostics(acpmock.DiagnosticsForAGHSession(records, resumed.ID))
	if got, want := len(prompts), 2; got != want {
		t.Fatalf("resume replay prompt diagnostics = %#v, want %d prompt records", prompts, want)
	}
	checkpointIndex := strings.Index(prompts[1].Prompt, "<agh_checkpoint_summary>")
	replayIndex := strings.Index(prompts[1].Prompt, resumeReplayOpenTag)
	if checkpointIndex < 0 || replayIndex < 0 || checkpointIndex >= replayIndex {
		t.Fatalf("resume replay checkpoint ordering invalid:\n%s", prompts[1].Prompt)
	}
	assertResumeReplayEqualsPrunedEvents(t, prompts[1].Prompt, eventsBeforeResume)
	assertContextRebuiltMarkerCount(t, readStoredEvents(t, resumed), 1)
}

func TestManagerIntegrationKillProcessPersistsAgentCrashedStopReason(t *testing.T) {
	h := newHarness(t)
	driver := acp.New(
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
		Agents: []aghconfig.AgentDef{{
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

	waitForCondition(t, "agent crash metadata", func() bool {
		meta := readMeta(t, session.MetaPath())
		return meta.State == string(StateStopped) && meta.StopReason != nil
	})

	meta := readMeta(t, session.MetaPath())
	if *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopAgentCrashed)
	}
}

func TestManagerIntegrationCreateAndResumeWithWorkspaceResolver(t *testing.T) {
	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace root) error = %v", err)
	}

	command := sessionStopHelperCommand(t)
	writeSessionIntegrationAgentDef(t, homePaths, "coder", command)

	registry, err := globaldb.OpenGlobalDB(context.Background(), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(context.Background()); err != nil {
			t.Fatalf("registry.Close() error = %v", err)
		}
	})

	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Providers[acpmock.ProviderName] = acpmock.ProviderConfig(command)

	resolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		workspacepkg.WithConfigLoader(func(string) (aghconfig.Config, error) { return cfg, nil }),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}

	driver := acp.New(acp.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
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
}

func TestManagerIntegrationResumeClassifiesCrashAndActivates(t *testing.T) {
	h := newRealACPIntegrationHarness(t, sessionStopHelperCommand(t))

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	waitForStoppedSession(t, h.manager, session)

	meta := readMeta(t, session.MetaPath())
	meta.State = string(StateActive)
	meta.StopReason = nil
	meta.StopDetail = ""
	if err := store.WriteSessionMeta(session.MetaPath(), meta); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), resumed.ID); err != nil {
			t.Fatalf("cleanup Stop() error = %v", err)
		}
	})

	if got := resumed.Info().State; got != StateActive {
		t.Fatalf("resumed state = %q, want %q", got, StateActive)
	}
	if got := resumed.Info().StopReason; got != store.StopAgentCrashed {
		t.Fatalf("resumed stop reason = %q, want %q", got, store.StopAgentCrashed)
	}
	if got := resumed.Info().StopDetail; got != resumeStopDetailAgentCrashed {
		t.Fatalf("resumed stop detail = %q, want %q", got, resumeStopDetailAgentCrashed)
	}

	meta = readMeta(t, resumed.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopAgentCrashed)
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
		Agents: []aghconfig.AgentDef{{
			Name:     aghconfig.DefaultAgentName,
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

	waitForCondition(t, "two stop events after resume flow", func() bool {
		events := readStoredEvents(t, resumed)
		return countEventType(events, EventTypeSessionStopped) == 2
	})

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
	driver := acp.New(
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
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: acpmock.ProviderName,
			Command:  command,
			Prompt:   "You are a coding assistant.",
		}},
	})
	h.manager = newManagerWithHarness(t, h, WithDriver(NewACPDriverAdapter(driver)))
	return h
}

func waitForStoppedSession(t *testing.T, manager *Manager, sess *Session) {
	t.Helper()

	waitForCondition(t, "stopped session state", func() bool {
		if _, ok := manager.Get(sess.ID); ok {
			return false
		}
		meta := readMeta(t, sess.MetaPath())
		return meta.State == string(StateStopped)
	})
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

func writeSessionIntegrationAgentDef(t *testing.T, homePaths aghconfig.HomePaths, name string, command string) {
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
