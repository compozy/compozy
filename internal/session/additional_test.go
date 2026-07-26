package session

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCreateCleansUpOnStartFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	recorder := &stubRecorder{}
	h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
		return nil, errors.New("start failed token=super-secret")
	}
	h.manager = newManagerWithHarness(t, h,
		WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return recorder, nil
		}),
	)

	_, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err == nil {
		t.Fatal("Create() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "start failed") {
		t.Fatalf("Create() error = %v, want start failure", err)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder close calls = %d, want 1", recorder.closeCalls)
	}
	meta := readMeta(t, store.SessionMetaFile(filepath.Join(h.homePaths.SessionsDir, "sess-1")))
	if meta.State != string(StateStopped) {
		t.Fatalf("failed start meta state = %q, want stopped", meta.State)
	}
	if meta.Failure == nil {
		t.Fatal("failed start meta Failure = nil, want startup failure")
	}
	if got, want := meta.Failure.Kind, store.FailureStartup; got != want {
		t.Fatalf("failed start failure kind = %q, want %q", got, want)
	}
	if !strings.Contains(meta.Failure.Summary, "start failed") ||
		strings.Contains(meta.Failure.Summary, "super-secret") ||
		!strings.Contains(meta.Failure.Summary, "[REDACTED]") {
		t.Fatalf("failed start summary = %q, want redacted start failure", meta.Failure.Summary)
	}
	if meta.Failure.CrashBundlePath == "" {
		t.Fatal("failed start crash bundle path = empty, want persisted bundle")
	}
	if _, statErr := os.Stat(meta.Failure.CrashBundlePath); statErr != nil {
		t.Fatalf("stat crash bundle %q error = %v", meta.Failure.CrashBundlePath, statErr)
	}
	bundle, err := os.ReadFile(meta.Failure.CrashBundlePath)
	if err != nil {
		t.Fatalf("ReadFile(crash bundle) error = %v", err)
	}
	if strings.Contains(string(bundle), "super-secret") || !strings.Contains(string(bundle), "[REDACTED]") {
		t.Fatalf("crash bundle = %q, want redacted secret", string(bundle))
	}
	if len(h.manager.List()) != 0 {
		t.Fatalf("List() after failed create = %d sessions, want 0", len(h.manager.List()))
	}
	all, err := h.manager.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("ListAll() after failed create error = %v", err)
	}
	if len(all) != 1 || all[0].Failure == nil || all[0].Failure.Kind != store.FailureStartup {
		t.Fatalf("ListAll() after failed create = %#v, want one startup failure", all)
	}
	if got := h.notifier.createdCount(); got != 1 {
		t.Fatalf("created notifications after failed create = %d, want 1", got)
	}
	if got := h.notifier.stoppedCount(); got != 1 {
		t.Fatalf("stopped notifications after failed create = %d, want 1", got)
	}
	wantOrder := []string{"created:sess-1", "stopped:sess-1"}
	if got := h.notifier.notificationOrder(); !slices.Equal(got, wantOrder) {
		t.Fatalf("notification order after failed create = %#v, want %#v", got, wantOrder)
	}
}

func TestCreateErrorBranches(t *testing.T) {
	t.Parallel()

	t.Run("Should blank agent name uses config default", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{Workspace: h.workspaceID})
		if err != nil {
			t.Fatalf("Create(blank agent) error = %v", err)
		}
		t.Cleanup(func() {
			_ = h.manager.Stop(testutil.Context(t), session.ID)
		})
		if got, want := session.Info().AgentName, aghconfig.DefaultAgentName; got != want {
			t.Fatalf("Create(blank agent) AgentName = %q, want %q", got, want)
		}
	})

	t.Run("Should blank agent name without config default", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.cfg.Defaults.Agent = ""
		h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{
				ID:      h.workspaceID,
				RootDir: h.workspace,
				Name:    h.workspaceName,
			},
			Config: h.cfg,
			Agents: []aghconfig.AgentDef{{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			}},
		})
		h.manager = newManagerWithHarness(t, h)
		if _, err := h.manager.Create(testutil.Context(t), CreateOpts{Workspace: h.workspaceID}); err == nil {
			t.Fatal("Create(blank agent with empty defaults) error = nil, want non-nil")
		}
	})

	t.Run("Should empty generated session id", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithSessionIDGenerator(func() string { return "" }))
		if _, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		}); err == nil {
			t.Fatal("Create(empty session id) error = nil, want non-nil")
		}
	})

	t.Run("Should store open failure", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.manager = newManagerWithHarness(t, h, WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return nil, errors.New("open failed")
		}))
		if _, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		}); err == nil {
			t.Fatal("Create(store open failure) error = nil, want non-nil")
		}
	})
}

func TestCreateWithNilPromptAssemblerIsSafe(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptAssembler(nil))

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got := session.Info().Type; got != SessionTypeUser {
		t.Fatalf("session type = %q, want %q", got, SessionTypeUser)
	}
	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant." {
		t.Fatalf("start system prompt = %q, want raw agent prompt", got)
	}
}

func TestResumeCleansUpOnStartFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	recorder := &stubRecorder{}
	h.driver.startHook = func(_ acp.StartOpts, _ int) (*fakeProcess, error) {
		return nil, errors.New("resume failed")
	}
	h.manager = newManagerWithHarness(t, h,
		WithStore(func(context.Context, string, string) (EventRecorder, error) {
			return recorder, nil
		}),
	)

	_, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err == nil {
		t.Fatal("Resume() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "resume failed") {
		t.Fatalf("Resume() error = %v, want resume failure", err)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder close calls = %d, want 1", recorder.closeCalls)
	}
}

func TestCreatePassesResolvedAdditionalDirsToDriver(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	additionalOne := filepath.Join(h.homePaths.HomeDir, "shared-one")
	additionalTwo := filepath.Join(h.homePaths.HomeDir, "shared-two")
	for _, dir := range []string{additionalOne, additionalTwo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
		}
	}

	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:             h.workspaceID,
			RootDir:        h.workspace,
			AdditionalDirs: []string{additionalOne, additionalTwo},
			Name:           h.workspaceName,
		},
		Config: h.cfg,
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are a coding assistant.",
		}},
	})

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	if got, want := h.driver.startCalls[0].AdditionalDirs, []string{
		additionalOne,
		additionalTwo,
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("start AdditionalDirs = %#v, want %#v", got, want)
	}
}

func TestResumePassesResolvedAdditionalDirsToDriver(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	additionalOne := filepath.Join(h.homePaths.HomeDir, "shared-one")
	additionalTwo := filepath.Join(h.homePaths.HomeDir, "shared-two")
	for _, dir := range []string{additionalOne, additionalTwo} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
		}
	}

	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:             h.workspaceID,
			RootDir:        h.workspace,
			AdditionalDirs: []string{additionalOne, additionalTwo},
			Name:           h.workspaceName,
		},
		Config: h.cfg,
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "You are a coding assistant.",
		}},
	})

	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), resumed.ID)
	})

	if got, want := h.driver.startCalls[1].AdditionalDirs, []string{
		additionalOne,
		additionalTwo,
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("resume start AdditionalDirs = %#v, want %#v", got, want)
	}
}

func TestResumeErrorBranches(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if _, err := h.manager.Resume(testutil.Context(t), ""); err == nil {
		t.Fatal("Resume(blank id) error = nil, want non-nil")
	}
	if _, err := h.manager.Resume(testutil.Context(t), "missing"); err == nil {
		t.Fatal("Resume(missing meta) error = nil, want non-nil")
	}
}

func TestPromptErrorPaths(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "   "); err == nil {
		t.Fatal("Prompt(empty) error = nil, want non-nil")
	}
	if _, err := h.manager.Prompt(testutil.Context(t), "missing", "hello"); err == nil {
		t.Fatal("Prompt(missing) error = nil, want non-nil")
	}
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "after-stop"); err == nil {
		t.Fatal("Prompt(stopped) error = nil, want non-nil")
	} else {
		if !errors.Is(err, ErrSessionNotActive) {
			t.Fatalf("Prompt(stopped) error = %v, want ErrSessionNotActive", err)
		}
		if !strings.Contains(err.Error(), string(StateStopped)) {
			t.Fatalf("Prompt(stopped) error = %v, want stopped state context", err)
		}
	}

	h = newHarness(t)
	session = createSession(t, h)
	session.clearProcess(time.Now().UTC())
	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "missing-process"); err == nil {
		t.Fatal("Prompt(missing process) error = nil, want non-nil")
	}
}

func TestResumeReturnsExistingActiveSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		_ = h.manager.Stop(testutil.Context(t), session.ID)
	})

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(active) error = %v", err)
	}
	if resumed != session {
		t.Fatalf("Resume(active) returned %p, want %p", resumed, session)
	}
}

func TestNewManagerOptionsAndValidation(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	now := time.Date(2026, 4, 3, 15, 0, 0, 0, time.UTC)
	cfg := aghconfig.DefaultWithHome(homePaths)
	resolver := newFakeWorkspaceResolver(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      "ws-options",
			RootDir: "/tmp/workspace",
			Name:    "workspace",
		},
		Config: cfg,
		Agents: []aghconfig.AgentDef{{
			Name:     aghconfig.DefaultAgentName,
			Provider: "claude",
			Prompt:   "hi",
		}},
	})
	manager, err := NewManager(
		WithHomePaths(homePaths),
		WithDriver(newFakeDriver()),
		WithNotifier(newFakeNotifier()),
		WithWorkspaceResolver(resolver),
		WithNow(func() time.Time { return now }),
		WithPromptBufferSize(7),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if got := manager.now(); !got.Equal(now) {
		t.Fatalf("now() = %s, want %s", got, now)
	}
	if got := manager.promptBufSize; got != 7 {
		t.Fatalf("promptBufSize = %d, want 7", got)
	}
	if manager.workspace != resolver {
		t.Fatal("workspace resolver override was not applied")
	}

	defaultManager, err := NewManager(WithHomePaths(homePaths))
	if err != nil {
		t.Fatalf("NewManager(defaults) error = %v", err)
	}
	if defaultManager.driver == nil {
		t.Fatal("defaultManager.driver = nil, want default ACP adapter")
	}

	_, err = NewManager(
		WithHomePaths(homePaths),
		WithDriver(nil),
	)
	if err == nil {
		t.Fatal("NewManager(nil driver) error = nil, want non-nil")
	}

	manager, err = NewManager(
		WithHomePaths(homePaths),
		WithDriver(newFakeDriver()),
		WithLogger(nil),
		WithNotifier(nil),
		WithWorkspaceResolver(resolver),
		WithStore(func(context.Context, string, string) (EventRecorder, error) { return &stubRecorder{}, nil }),
		WithNow(nil),
		WithSessionIDGenerator(nil),
		WithTurnIDGenerator(nil),
		WithPromptBufferSize(0),
	)
	if err != nil {
		t.Fatalf("NewManager(normalized defaults) error = %v", err)
	}
	if manager.logger == nil || manager.now == nil || manager.newSessionID == nil || manager.newTurnID == nil {
		t.Fatal("NewManager() failed to restore default dependencies after nil overrides")
	}
	if manager.notifier != nil {
		t.Fatal("NewManager() restored a notifier default, want nil")
	}
	if manager.promptBufSize != defaultPromptBufferSize {
		t.Fatalf("promptBufSize = %d, want default %d", manager.promptBufSize, defaultPromptBufferSize)
	}

	testCases := []struct {
		name string
		opts []Option
	}{
		{
			name: "missing store opener",
			opts: []Option{
				WithHomePaths(homePaths),
				WithDriver(newFakeDriver()),
				WithWorkspaceResolver(resolver),
				WithStore(nil),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewManager(tc.opts...); err == nil {
				t.Fatalf("NewManager(%s) error = nil, want non-nil", tc.name)
			}
		})
	}

	t.Run("ShouldWrapInvalidSupervisionConfigWithSessionContext", func(t *testing.T) {
		t.Parallel()

		_, err := NewManager(
			WithHomePaths(homePaths),
			WithDriver(newFakeDriver()),
			WithWorkspaceResolver(resolver),
			WithSessionSupervision(aghconfig.SessionSupervisionConfig{
				ActivityHeartbeatInterval: -time.Second,
				ProgressNotifyInterval:    time.Minute,
				InactivityWarningAfter:    time.Minute,
				InactivityTimeout:         2 * time.Minute,
				TimeoutCancelGrace:        time.Second,
			}),
		)
		if err == nil {
			t.Fatal("NewManager(invalid supervision) error = nil, want non-nil")
		}
		if !strings.Contains(err.Error(), "session: session.supervision.activity_heartbeat_interval") {
			t.Fatalf("NewManager(invalid supervision) error = %v, want session supervision context", err)
		}
	})
}

func TestHelperFunctionsAndUtilities(t *testing.T) {
	t.Parallel()

	resolved := workspacepkg.ResolvedWorkspace{
		Agents: []aghconfig.AgentDef{{
			Name:     "coder",
			Provider: "claude",
			Prompt:   "hi",
		}},
	}
	got, err := resolveWorkspaceAgent("coder", &resolved)
	if err != nil {
		t.Fatalf("resolveWorkspaceAgent(coder) error = %v", err)
	}
	if got.Name != "coder" {
		t.Fatalf("resolveWorkspaceAgent(coder) name = %q, want coder", got.Name)
	}
	if _, err := resolveWorkspaceAgent("missing", &resolved); !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		t.Fatalf("resolveWorkspaceAgent(missing) error = %v, want ErrAgentNotAvailable", err)
	}

	done := make(chan struct{})
	close(done)
	if !isProcessDone(&AgentProcess{done: done}) {
		t.Fatal("isProcessDone(closed) = false, want true")
	}
	if !isProcessDone(nil) {
		t.Fatal("isProcessDone(nil) = false, want true")
	}

	if got := derefString(nil); got != "" {
		t.Fatalf("derefString(nil) = %q, want empty", got)
	}
	if got := *stringPointer("value"); got != "value" {
		t.Fatalf("stringPointer(value) = %q, want value", got)
	}
	if stringPointer("   ") != nil {
		t.Fatal("stringPointer(blank) = non-nil, want nil")
	}
	if got := newID("sess"); !strings.HasPrefix(got, "sess-") {
		t.Fatalf("newID(sess) = %q, want prefixed id", got)
	}
	if got := newID(""); got == "" {
		t.Fatal("newID(\"\") = empty, want non-empty")
	}
}

func TestCreateWithBlankWorkspaceReturnsValidationError(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	if _, err := h.manager.Create(testutil.Context(t), CreateOpts{AgentName: "coder"}); err == nil {
		t.Fatal("Create(blank workspace) error = nil, want non-nil")
	}
	if _, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:     "coder",
		Workspace:     h.workspaceID,
		WorkspacePath: h.workspace,
	}); err == nil {
		t.Fatal("Create(workspace + workspacePath) error = nil, want non-nil")
	}
}

func TestCreateAndResumeRequireWorkspaceResolver(t *testing.T) {
	t.Parallel()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}

	manager, err := NewManager(
		WithHomePaths(homePaths),
		WithDriver(newFakeDriver()),
		WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if _, err := manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: "ws-missing",
	}); err == nil {
		t.Fatal("Create(without resolver) error = nil, want non-nil")
	}

	sessionDir := filepath.Join(homePaths.SessionsDir, "sess-stored")
	if err := store.WriteSessionMeta(store.SessionMetaFile(sessionDir), store.SessionMeta{
		ID:                   "sess-stored",
		AgentName:            "coder",
		WorkspaceID:          "ws-stored",
		NetworkParticipation: testLocalParticipationPtr(),
		State:                string(StateStopped),
		CreatedAt:            time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteSessionMeta() error = %v", err)
	}

	if _, err := manager.Resume(testutil.Context(t), "sess-stored"); err == nil {
		t.Fatal("Resume(without resolver) error = nil, want non-nil")
	}
}

func TestMarshalAgentEvent(t *testing.T) {
	t.Parallel()

	rawPayload := json.RawMessage(
		`{"sessionUpdate":"tool_call_update","status":"completed","rawOutput":"hello","_meta":{"claudeCode":{"toolName":"Bash"}}}`,
	)
	raw, err := marshalAgentEvent(acp.AgentEvent{Raw: rawPayload})
	if err != nil {
		t.Fatalf("marshalAgentEvent(raw) error = %v", err)
	}

	totalTokens := int64(4)
	payload, err := marshalAgentEvent(acp.AgentEvent{
		Type:      acp.EventTypeDone,
		SessionID: "acp-1",
		TurnID:    "turn-1",
		Timestamp: time.Now().UTC(),
		Text:      "done",
		Error:     "none",
		Usage: &acp.TokenUsage{
			TurnID:      "turn-1",
			TotalTokens: &totalTokens,
			Timestamp:   time.Now().UTC(),
		},
	})
	if err != nil {
		t.Fatalf("marshalAgentEvent(structured) error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(payload) error = %v", err)
	}
	if decoded["schema"] != transcript.CanonicalSchema {
		t.Fatalf("decoded[schema] = %v, want %q", decoded["schema"], transcript.CanonicalSchema)
	}
	if decoded["type"] != acp.EventTypeDone {
		t.Fatalf("decoded[type] = %v, want %q", decoded["type"], acp.EventTypeDone)
	}
	if decoded["text"] != "done" {
		t.Fatalf("decoded[text] = %v, want %q", decoded["text"], "done")
	}

	var rawDecoded map[string]any
	if err := json.Unmarshal([]byte(raw), &rawDecoded); err != nil {
		t.Fatalf("json.Unmarshal(raw payload) error = %v", err)
	}
	if rawDecoded["schema"] != transcript.CanonicalSchema {
		t.Fatalf("rawDecoded[schema] = %v, want %q", rawDecoded["schema"], transcript.CanonicalSchema)
	}
	if rawDecoded["tool_name"] != "Bash" {
		t.Fatalf("rawDecoded[tool_name] = %v, want %q", rawDecoded["tool_name"], "Bash")
	}
	if _, ok := rawDecoded["raw"]; ok {
		t.Fatalf("rawDecoded unexpectedly retained nested raw payload: %#v", rawDecoded)
	}
}

func TestAgentProcessHelpersAndAdapterUtilities(t *testing.T) {
	t.Parallel()

	wrapped := wrapACPProcess(&acp.AgentProcess{
		PID:       99,
		AgentName: "coder",
		Command:   "fake",
		Cwd:       "/tmp",
		SessionID: "acp-1",
		Caps:      acp.Caps{SupportsLoadSession: true},
		StartedAt: time.Now().UTC(),
	})
	if wrapped == nil || wrapped.PID != 99 || wrapped.SessionID != "acp-1" {
		t.Fatalf("wrapACPProcess() = %#v", wrapped)
	}
	if wrapACPProcess(nil) != nil {
		t.Fatal("wrapACPProcess(nil) = non-nil, want nil")
	}

	manager := &Manager{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if got := manager.sessionLogger(nil); got == nil {
		t.Fatal("sessionLogger(nil) = nil, want logger")
	}
	if got := manager.sessionLogger(&Session{ID: "sess-1", AgentName: "coder"}); got == nil {
		t.Fatal("sessionLogger(session) = nil, want logger")
	}

	adapter := NewACPDriverAdapter(acp.New())
	if _, err := adapter.nativeProcess(&AgentProcess{}); err == nil {
		t.Fatal("nativeProcess(unsupported process) error = nil, want non-nil")
	}

	if _, err := adapter.Start(context.Background(), acp.StartOpts{}); err == nil {
		t.Fatal("Start(invalid opts) error = nil, want non-nil")
	}
	if err := adapter.Cancel(context.Background(), wrapACPProcess(&acp.AgentProcess{})); err == nil {
		t.Fatal("Cancel(empty session id) error = nil, want non-nil")
	}
}

func TestSessionDirAccessorAndStopWithoutProcess(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if session.SessionDir() == "" {
		t.Fatal("SessionDir() = empty, want path")
	}

	session.clearProcess(time.Now().UTC())
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop(no process) error = %v", err)
	}
	if readMeta(t, session.MetaPath()).State != string(StateStopped) {
		t.Fatalf("meta state after no-process stop = %q, want %q", readMeta(t, session.MetaPath()).State, StateStopped)
	}
}

func TestTransitionToSameStateOnlyUpdatesTimestamp(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	session := &Session{
		ID:        "sess-1",
		AgentName: "coder",
		Workspace: t.TempDir(),
		State:     StateActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	later := now.Add(time.Second)
	if err := session.transition(StateActive, later); err != nil {
		t.Fatalf("transition(same state) error = %v", err)
	}
	if got := session.Info().UpdatedAt; !got.Equal(later) {
		t.Fatalf("UpdatedAt = %s, want %s", got, later)
	}
}

type stubRecorder struct {
	closeCalls int
}

func (s *stubRecorder) Record(context.Context, store.SessionEvent) error {
	return nil
}

func (s *stubRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (s *stubRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return nil, nil
}

func (s *stubRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (s *stubRecorder) Close(context.Context) error {
	s.closeCalls++
	return nil
}
