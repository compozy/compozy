package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestCreateAcceptedInitialPromptLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should stage the prompt before acceptance and dispatch it once after activation", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionCatalog(queueStore), WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)

		startEntered := make(chan struct{})
		releaseStart := make(chan struct{})
		promptEntered := make(chan struct{})
		var acceptedID string
		var releaseOnce sync.Once
		t.Cleanup(func() { releaseOnce.Do(func() { close(releaseStart) }) })
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			close(startEntered)
			<-releaseStart
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-initial-prompt"), nil
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			close(promptEntered)
			events := make(chan acp.AgentEvent, 2)
			emitDonePromptEvents(events, acceptedID, req.TurnID)
			close(events)
			return events, nil
		}

		accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				AgentName: "coder",
				Workspace: h.workspaceID,
			},
			InitialPrompt: "  Investigate the failing build  ",
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		acceptedID = accepted.ID
		select {
		case <-startEntered:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for provider startup to begin")
		}
		if accepted.State != StateStarting {
			t.Fatalf("CreateAccepted() State = %q, want %q", accepted.State, StateStarting)
		}
		summary, err := queueStore.SessionInputQueueSummary(testutil.Context(t), accepted.ID)
		if err != nil {
			t.Fatalf("SessionInputQueueSummary(starting) error = %v", err)
		}
		if summary.PendingActive != 1 || summary.PendingQueued != 1 {
			t.Fatalf("SessionInputQueueSummary(starting) = %#v, want one queued prompt", summary)
		}
		queued, found, err := queueStore.PeekNextSessionInput(testutil.Context(t), accepted.ID)
		if err != nil {
			t.Fatalf("PeekNextSessionInput(starting) error = %v", err)
		}
		if !found || queued.Text != "Investigate the failing build" {
			t.Fatalf("PeekNextSessionInput(starting) = %#v/%t, want trimmed prompt", queued, found)
		}
		select {
		case <-promptEntered:
			t.Fatal("initial prompt reached the driver while the session was starting")
		default:
		}

		releaseOnce.Do(func() { close(releaseStart) })
		select {
		case <-promptEntered:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for the staged prompt to reach the driver")
		}
		waitForCondition(t, "initial prompt completion", func() bool {
			live, ok := h.manager.Get(accepted.ID)
			return ok && live.Info().State == StateActive && !live.IsPrompting()
		})
		promptCalls := managerPromptCalls(h)
		if len(promptCalls) != 1 || promptCalls[0].Message != "Investigate the failing build" {
			t.Fatalf("prompt calls = %#v, want one trimmed initial prompt", promptCalls)
		}
		if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should preserve create-only behavior for a whitespace-only prompt", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionCatalog(queueStore), WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)

		accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session:       CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
			InitialPrompt: "  \n\t  ",
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		if accepted.State != StateStarting {
			t.Fatalf("CreateAccepted() State = %q, want %q", accepted.State, StateStarting)
		}
		waitForCondition(t, "whitespace-only create activation", func() bool {
			live, ok := h.manager.Get(accepted.ID)
			return ok && live.Info().State == StateActive
		})
		summary, err := queueStore.SessionInputQueueSummary(testutil.Context(t), accepted.ID)
		if err != nil {
			t.Fatalf("SessionInputQueueSummary() error = %v", err)
		}
		if summary.PendingActive != 0 || summary.PendingQueued != 0 || len(managerPromptCalls(h)) != 0 {
			t.Fatalf("whitespace-only prompt state = %#v, calls=%#v", summary, managerPromptCalls(h))
		}
		if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should retain a staged prompt after startup failure and dispatch it once on resume", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionCatalog(queueStore), WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		startErr := errors.New("provider unavailable")
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if sequence == 1 {
				return nil, startErr
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-resumed-prompt"), nil
		}

		accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session:       CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
			InitialPrompt: "Continue after recovery",
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		waitForCondition(t, "durable startup failure with queued prompt", func() bool {
			info, statusErr := h.manager.Status(testutil.Context(t), accepted.ID)
			_, stillRegistered := h.manager.Get(accepted.ID)
			return statusErr == nil && info.State == StateStopped && !stillRegistered
		})
		summary, err := queueStore.SessionInputQueueSummary(testutil.Context(t), accepted.ID)
		if err != nil {
			t.Fatalf("SessionInputQueueSummary(stopped) error = %v", err)
		}
		if summary.PendingQueued != 1 || len(managerPromptCalls(h)) != 0 {
			t.Fatalf("stopped prompt state = %#v, calls=%#v", summary, managerPromptCalls(h))
		}

		resumed, err := h.manager.Resume(testutil.Context(t), accepted.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		waitForCondition(t, "resumed initial prompt dispatch", func() bool {
			matches := 0
			for _, call := range managerPromptCalls(h) {
				if strings.HasSuffix(call.Message, "User request:\n\nContinue after recovery") {
					matches++
				}
			}
			return matches == 1
		})
		waitForCondition(t, "resumed initial prompt completion", func() bool {
			return !resumed.IsPrompting()
		})
		promptCalls := managerPromptCalls(h)
		matches := 0
		for _, call := range promptCalls {
			if strings.HasSuffix(call.Message, "User request:\n\nContinue after recovery") {
				matches++
			}
		}
		if matches != 1 {
			t.Fatalf("prompt calls = %#v, want initial prompt exactly once after resume", promptCalls)
		}
		if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
			t.Fatalf("Stop() cleanup error = %v", err)
		}
	})

	t.Run("Should discard every accepted-session artifact when prompt queue admission fails", func(t *testing.T) {
		t.Parallel()

		catalog := openManagerInputQueueStore(t)
		h := newHarness(
			t,
			WithSessionCatalog(catalog),
			WithSessionInputQueueStore(catalog),
			WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
				DefaultMode:  string(BusyInputModeQueue),
				QueueCap:     10,
				MaxTextBytes: 512,
			}),
		)
		registerManagerInputQueueWorkspace(t, catalog, h)
		const sessionID = "sess-prompt-admission-failure"

		_, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				DesiredSessionID: sessionID,
				AgentName:        "coder",
				Workspace:        h.workspaceID,
			},
			InitialPrompt: strings.Repeat("x", 513),
		})
		if err == nil || !strings.Contains(err.Error(), "max_text_bytes (512)") {
			t.Fatalf("CreateAccepted() error = %v, want queue admission failure", err)
		}
		if _, ok := h.manager.Get(sessionID); ok {
			t.Fatalf("Get(%q) found residue after prompt admission failure", sessionID)
		}
		sessionDir := filepath.Join(h.homePaths.SessionsDir, sessionID)
		if _, statErr := os.Stat(sessionDir); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("session directory stat error = %v, want not exist", statErr)
		}
		listed, err := catalog.ListSessions(testutil.Context(t), store.SessionListQuery{WorkspaceID: h.workspaceID})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(listed) != 0 {
			t.Fatalf("ListSessions() = %#v, want no catalog residue", listed)
		}

		retry, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				DesiredSessionID: sessionID,
				AgentName:        "coder",
				Workspace:        h.workspaceID,
			},
		})
		if err != nil {
			t.Fatalf("CreateAccepted(retry) error = %v", err)
		}
		waitForCondition(t, "retry activation after prompt admission rollback", func() bool {
			live, ok := h.manager.Get(retry.ID)
			return ok && live.Info().State == StateActive
		})
		if err := h.manager.Stop(testutil.Context(t), retry.ID); err != nil {
			t.Fatalf("Stop(retry) cleanup error = %v", err)
		}
	})

	t.Run("Should retain a recoverable stopped session when catalog rollback cannot delete", func(t *testing.T) {
		t.Parallel()

		catalog := openManagerInputQueueStore(t)
		deleteErr := errors.New("catalog delete unavailable")
		h := newHarness(
			t,
			WithSessionCatalog(deleteFailingSessionCatalog{SessionCatalog: catalog, err: deleteErr}),
			WithSessionInputQueueStore(catalog),
			WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
				DefaultMode:  string(BusyInputModeQueue),
				QueueCap:     10,
				MaxTextBytes: 512,
			}),
		)
		registerManagerInputQueueWorkspace(t, catalog, h)
		const sessionID = "sess-prompt-admission-catalog-failure"

		_, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{
				DesiredSessionID: sessionID,
				AgentName:        "coder",
				Workspace:        h.workspaceID,
			},
			InitialPrompt: strings.Repeat("x", 513),
		})
		if !errors.Is(err, deleteErr) {
			t.Fatalf("CreateAccepted() error = %v, want catalog delete failure", err)
		}
		if _, ok := h.manager.Get(sessionID); ok {
			t.Fatalf("Get(%q) found a stranded starting session", sessionID)
		}
		if _, statErr := os.Stat(filepath.Join(h.homePaths.SessionsDir, sessionID)); statErr != nil {
			t.Fatalf("session directory was not restored after catalog delete failure: %v", statErr)
		}
		info, statusErr := h.manager.Status(testutil.Context(t), sessionID)
		if statusErr != nil {
			t.Fatalf("Status(%q) error = %v", sessionID, statusErr)
		}
		if info.State != StateStopped || info.Failure == nil {
			t.Fatalf("Status(%q) = %#v, want recoverable stopped failure", sessionID, info)
		}
	})
}

type deleteFailingSessionCatalog struct {
	store.SessionCatalog
	err error
}

func (c deleteFailingSessionCatalog) DeleteSession(context.Context, string) error {
	return c.err
}

func TestCreateAcceptedPersistsStartingBeforeProviderStartupCompletes(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should persist starting and expose an empty transcript before provider activation",
		testCreateAcceptedPersistsStarting,
	)
	t.Run(
		"Should remain externally starting until the committed startup run completes",
		testCreateAcceptedRemainsStartingUntilCommitted,
	)
}

func testCreateAcceptedPersistsStarting(t *testing.T) {
	t.Helper()
	h := newHarness(t)
	startEntered := make(chan struct{})
	releaseStart := make(chan struct{})
	h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
		close(startEntered)
		<-releaseStart
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-accepted"), nil
	}

	startedAt := time.Now()
	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		},
	})
	acceptanceLatency := time.Since(startedAt)
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	if acceptanceLatency >= 250*time.Millisecond {
		t.Fatalf("CreateAccepted() latency = %s, want < 250ms while provider startup is blocked", acceptanceLatency)
	}
	if accepted.State != StateStarting {
		t.Fatalf("CreateAccepted() State = %q, want %q", accepted.State, StateStarting)
	}
	live, ok := h.manager.Get(accepted.ID)
	if !ok || live.Info().State != StateStarting {
		t.Fatalf("Get(%q) = %#v/%t, want durable starting session", accepted.ID, live, ok)
	}
	if meta := readMeta(t, live.MetaPath()); meta.State != string(StateStarting) {
		t.Fatalf("accepted meta State = %q, want %q", meta.State, StateStarting)
	}
	page, err := h.manager.TranscriptPage(testutil.Context(t), accepted.ID, transcript.PageQuery{})
	if err != nil {
		t.Fatalf("TranscriptPage(starting) error = %v", err)
	}
	if len(page.Entries) != 0 {
		t.Fatalf("TranscriptPage(starting) entries = %#v, want empty", page.Entries)
	}
	<-startEntered
	close(releaseStart)
	waitForCondition(t, "accepted session activation", func() bool {
		live, ok := h.manager.Get(accepted.ID)
		return ok && live.Info().State == StateActive
	})
	if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
		t.Fatalf("Stop() cleanup error = %v", err)
	}
}

func testCreateAcceptedRemainsStartingUntilCommitted(t *testing.T) {
	t.Helper()
	created := make(chan struct{})
	release := make(chan struct{})
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	h := newHarness(t, WithNotifier(&blockingCreatedNotifier{created: created, release: release}))

	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		},
	})
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	<-created

	status, err := h.manager.Status(testutil.Context(t), accepted.ID)
	if err != nil {
		t.Fatalf("Status(starting commit) error = %v", err)
	}
	if status.State != StateStarting {
		t.Fatalf("Status(starting commit) State = %q, want %q", status.State, StateStarting)
	}
	listed := h.manager.List()
	if len(listed) != 1 || listed[0].ID != accepted.ID || listed[0].State != StateStarting {
		t.Fatalf("List(starting commit) = %#v, want one starting session", listed)
	}
	if _, err := h.manager.SendPrompt(testutil.Context(t), accepted.ID, SendPromptOpts{
		Message: "too early",
	}); !errors.Is(err, ErrSessionNotActive) {
		t.Fatalf("SendPrompt(starting commit) error = %v, want %v", err, ErrSessionNotActive)
	}

	close(release)
	released = true
	waitForCondition(t, "committed session activation", func() bool {
		current, statusErr := h.manager.Status(testutil.Context(t), accepted.ID)
		return statusErr == nil && current.State == StateActive
	})
	if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
		t.Fatalf("Stop() cleanup error = %v", err)
	}
}

type blockingCreatedNotifier struct {
	created chan struct{}
	release chan struct{}
}

func (n *blockingCreatedNotifier) OnSessionCreated(context.Context, *Session) {
	close(n.created)
	<-n.release
}

func (*blockingCreatedNotifier) OnSessionStopped(context.Context, *Session) {}

func (*blockingCreatedNotifier) OnAgentEvent(context.Context, string, any) {}

func TestCreateAcceptedPersistsBackgroundStartupFailure(t *testing.T) {
	t.Parallel()
	t.Run("Should persist background startup failure after durable acceptance", testCreateAcceptedPersistsFailure)
}

func testCreateAcceptedPersistsFailure(t *testing.T) {
	t.Helper()
	h := newHarness(t)
	startErr := errors.New("provider unavailable")
	h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
		return nil, startErr
	}

	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		},
	})
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	if accepted.State != StateStarting {
		t.Fatalf("CreateAccepted() State = %q, want %q", accepted.State, StateStarting)
	}
	waitForCondition(t, "durable startup failure", func() bool {
		info, statusErr := h.manager.Status(testutil.Context(t), accepted.ID)
		return statusErr == nil && info.State == StateStopped && info.Failure != nil
	})
	if run := h.manager.sessionStartRun(accepted.ID); run != nil {
		waitCtx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
		defer cancel()
		if waitErr := waitForSessionStartRun(waitCtx, run); !errors.Is(waitErr, startErr) {
			t.Fatalf("waitForSessionStartRun() error = %v, want provider failure", waitErr)
		}
	}
	info, err := h.manager.Status(testutil.Context(t), accepted.ID)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if info.Failure.Kind != store.FailureStartup || !strings.Contains(info.Failure.Summary, startErr.Error()) {
		t.Fatalf("startup failure = %#v, want startup kind with provider cause", info.Failure)
	}
}

func TestStopJoinsAcceptedStartupAndPreventsLateActivation(t *testing.T) {
	t.Parallel()
	t.Run("Should join accepted startup and prevent late activation", testStopJoinsAcceptedStartup)
	t.Run("Should wait for the accepted recorder before canceling startup", testAcceptedStartupRecorderReady)
	t.Run("Should preserve creation identity when stop overlaps its commit", testAcceptedStartupIdentityCommit)
}

func testStopJoinsAcceptedStartup(t *testing.T) {
	t.Helper()
	entered := make(chan struct{})
	h := newHarness(t, WithPromptAssembler(promptAssemblerFunc(func(
		ctx context.Context,
		_ compozyconfig.AgentDef,
		_ *workspacepkg.ResolvedWorkspace,
	) (string, error) {
		close(entered)
		<-ctx.Done()
		return "", ctx.Err()
	})))
	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		},
	})
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	<-entered
	if err := h.manager.Stop(testutil.Context(t), accepted.ID); err != nil {
		t.Fatalf("Stop(starting) error = %v", err)
	}
	if _, ok := h.manager.Get(accepted.ID); ok {
		t.Fatalf("Get(%q) found session after joined startup stop", accepted.ID)
	}
	info, err := h.manager.Status(testutil.Context(t), accepted.ID)
	if err != nil {
		t.Fatalf("Status(stopped start) error = %v", err)
	}
	if info.State != StateStopped {
		t.Fatalf("Status(stopped start).State = %q, want %q", info.State, StateStopped)
	}
	if got := len(h.driver.startCalls); got != 0 {
		t.Fatalf("driver start calls = %d, want 0 after startup cancellation", got)
	}
}

func testAcceptedStartupRecorderReady(t *testing.T) {
	t.Helper()
	t.Parallel()

	recorderOpenEntered := make(chan struct{})
	releaseRecorderOpen := make(chan struct{})
	var recorderOpenOnce sync.Once
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseRecorderOpen) }) })

	h := newHarness(t)
	h.manager = newManagerWithHarness(t, h, WithStore(func(
		ctx context.Context,
		sessionID string,
		path string,
	) (EventRecorder, error) {
		recorderOpenOnce.Do(func() { close(recorderOpenEntered) })
		select {
		case <-releaseRecorderOpen:
			return sessiondb.OpenSessionDB(ctx, sessionID, path)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}))

	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
	})
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	<-recorderOpenEntered
	active, ok := h.manager.Get(accepted.ID)
	if !ok {
		t.Fatalf("Get(%q) did not find accepted session", accepted.ID)
	}
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(testutil.Context(t), accepted.ID)
	}()

	select {
	case stopErr := <-stopDone:
		releaseOnce.Do(func() { close(releaseRecorderOpen) })
		t.Fatalf("Stop(starting) completed before recorder readiness with error = %v", stopErr)
	case <-time.After(50 * time.Millisecond):
	}

	releaseOnce.Do(func() { close(releaseRecorderOpen) })
	if err := <-stopDone; err != nil {
		t.Fatalf("Stop(accepted) error = %v", err)
	}
	meta := readMeta(t, active.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("stopped meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.StopReason == nil || *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("stopped meta reason = %v, want %q", meta.StopReason, store.StopUserCanceled)
	}
	if got := countEventType(readStoredEvents(t, active), EventTypeSessionStopped); got != 1 {
		t.Fatalf("session_stopped events = %d, want 1", got)
	}
}

type blockingSessionCreationCatalog struct {
	store.SessionCatalog
	store.SessionCreationStore
	registerEntered chan struct{}
	releaseRegister chan struct{}
	stateUpdated    chan struct{}
	registerOnce    sync.Once
	stateUpdateOnce sync.Once
}

func (c *blockingSessionCreationCatalog) RegisterSessionWithCreationIdentity(
	ctx context.Context,
	info store.SessionInfo,
	identity store.SessionCreationIdentity,
) (store.SessionCreationRegistration, error) {
	c.registerOnce.Do(func() { close(c.registerEntered) })
	select {
	case <-c.releaseRegister:
		return c.SessionCreationStore.RegisterSessionWithCreationIdentity(ctx, info, identity)
	case <-ctx.Done():
		return store.SessionCreationRegistration{}, ctx.Err()
	}
}

func (c *blockingSessionCreationCatalog) UpdateSessionState(
	ctx context.Context,
	update store.SessionStateUpdate,
) error {
	err := c.SessionCatalog.UpdateSessionState(ctx, update)
	c.stateUpdateOnce.Do(func() { close(c.stateUpdated) })
	return err
}

func testAcceptedStartupIdentityCommit(t *testing.T) {
	t.Helper()
	t.Parallel()

	catalog := openManagerInputQueueStore(t)
	registerEntered := make(chan struct{})
	releaseRegister := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseRegister) }) })
	blockingCatalog := &blockingSessionCreationCatalog{
		SessionCatalog:       catalog,
		SessionCreationStore: catalog,
		registerEntered:      registerEntered,
		releaseRegister:      releaseRegister,
		stateUpdated:         make(chan struct{}),
	}
	h := newHarness(t, WithSessionCatalog(blockingCatalog))
	registerManagerInputQueueWorkspace(t, catalog, h)

	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
	})
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	<-registerEntered
	active, ok := h.manager.Get(accepted.ID)
	if !ok {
		t.Fatalf("Get(%q) did not find accepted session", accepted.ID)
	}
	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(testutil.Context(t), accepted.ID)
	}()

	select {
	case <-blockingCatalog.stateUpdated:
		t.Fatal("Stop(starting) persisted terminal state while creation identity commit was pending")
	case <-time.After(50 * time.Millisecond):
	}
	listed, err := catalog.ListSessions(testutil.Context(t), store.SessionListQuery{ID: accepted.ID})
	if err != nil {
		t.Fatalf("ListSessions(pending identity) error = %v", err)
	}
	if len(listed) != 1 || listed[0].State != string(StateStarting) {
		t.Fatalf("catalog state during identity commit = %#v, want one starting session", listed)
	}

	releaseOnce.Do(func() { close(releaseRegister) })
	if err := <-stopDone; err != nil {
		t.Fatalf("Stop(accepted) error = %v", err)
	}
	meta := readMeta(t, active.MetaPath())
	metaIdentity := creationIdentityFromMeta(meta)
	if metaIdentity == nil {
		t.Fatal("stopped meta creation identity = nil")
	}
	storedIdentity, err := catalog.GetSessionCreationIdentity(testutil.Context(t), accepted.ID)
	if err != nil {
		t.Fatalf("GetSessionCreationIdentity() error = %v", err)
	}
	if storedIdentity != *metaIdentity {
		t.Fatalf("stored creation identity = %#v, want %#v", storedIdentity, *metaIdentity)
	}
	listed, err = catalog.ListSessions(testutil.Context(t), store.SessionListQuery{ID: accepted.ID})
	if err != nil {
		t.Fatalf("ListSessions(stopped) error = %v", err)
	}
	if len(listed) != 1 || listed[0].State != string(StateStopped) {
		t.Fatalf("catalog state after stop = %#v, want one stopped session", listed)
	}
	if got := countEventType(readStoredEvents(t, active), EventTypeSessionStopped); got != 1 {
		t.Fatalf("session_stopped events = %d, want 1", got)
	}
}

func TestDeleteJoinsAcceptedStartupAndRemovesDurableSession(t *testing.T) {
	t.Parallel()
	t.Run("Should join accepted startup and remove its durable session", testDeleteJoinsAcceptedStartup)
}

func testDeleteJoinsAcceptedStartup(t *testing.T) {
	t.Helper()
	entered := make(chan struct{})
	h := newHarness(t, WithPromptAssembler(promptAssemblerFunc(func(
		ctx context.Context,
		_ compozyconfig.AgentDef,
		_ *workspacepkg.ResolvedWorkspace,
	) (string, error) {
		close(entered)
		<-ctx.Done()
		return "", ctx.Err()
	})))
	accepted, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
		},
	})
	if err != nil {
		t.Fatalf("CreateAccepted() error = %v", err)
	}
	<-entered
	if err := h.manager.Delete(testutil.Context(t), accepted.ID); err != nil {
		t.Fatalf("Delete(starting) error = %v", err)
	}
	if _, ok := h.manager.Get(accepted.ID); ok {
		t.Fatalf("Get(%q) found session after delete joined startup", accepted.ID)
	}
	if _, err := h.manager.Status(testutil.Context(t), accepted.ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Status(deleted start) error = %v, want ErrSessionNotFound", err)
	}
	if got := len(h.driver.startCalls); got != 0 {
		t.Fatalf("driver start calls = %d, want 0 after startup deletion", got)
	}
}

func TestSessionStartEnvFiltersDaemonSecrets(t *testing.T) {
	t.Parallel()

	t.Run("Should remove credential shaped daemon variables and keep Compozy session context", func(t *testing.T) {
		t.Parallel()

		env := sessionStartEnv(
			[]string{
				"PATH=/usr/bin",
				"OPENAI_API_KEY=sk-secret",
				"GITHUB_TOKEN=ghp-secret",
				"PROVIDER_HOME=/tmp/provider",
			},
			&Session{
				ID:                   "sess-1",
				AgentName:            "coder",
				NetworkParticipation: testLiveParticipation("ws-test", "ops"),
			},
		)

		if got := envValue(env, "OPENAI_API_KEY"); got != "" {
			t.Fatalf("OPENAI_API_KEY = %q, want filtered", got)
		}
		if got := envValue(env, "GITHUB_TOKEN"); got != "" {
			t.Fatalf("GITHUB_TOKEN = %q, want filtered", got)
		}
		if got := envValue(env, "PROVIDER_HOME"); got != "/tmp/provider" {
			t.Fatalf("PROVIDER_HOME = %q, want %q", got, "/tmp/provider")
		}
		if got := envValue(env, "COMPOZY_SESSION_ID"); got != "sess-1" {
			t.Fatalf("COMPOZY_SESSION_ID = %q, want %q", got, "sess-1")
		}
		if got := envValue(env, "COMPOZY_PEER_ID"); got == "" {
			t.Fatal("COMPOZY_PEER_ID = empty, want network peer id")
		}
	})
}

func TestSessionStartEnvForProviderSupportsIsolatedPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should keep only operational env before adding session context", func(t *testing.T) {
		t.Parallel()

		env := sessionStartEnvForProvider(
			[]string{
				"PATH=/usr/bin",
				"HOME=/Users/operator",
				"OPENAI_API_KEY=sk-secret",
				"FEATURE_FLAG=enabled",
				"PROVIDER_HOME=/tmp/provider",
			},
			&Session{
				ID:                   "sess-1",
				AgentName:            "coder",
				NetworkParticipation: testLiveParticipation("ws-test", "ops"),
			},
			compozyconfig.ProviderEnvPolicyIsolated,
		)

		if got := envValue(env, "OPENAI_API_KEY"); got != "" {
			t.Fatalf("OPENAI_API_KEY = %q, want isolated env to drop secrets", got)
		}
		if got := envValue(env, "FEATURE_FLAG"); got != "" {
			t.Fatalf("FEATURE_FLAG = %q, want isolated env to drop non-allowlisted variables", got)
		}
		if got := envValue(env, "PATH"); got != "/usr/bin" {
			t.Fatalf("PATH = %q, want %q", got, "/usr/bin")
		}
		if got := envValue(env, "COMPOZY_SESSION_ID"); got != "sess-1" {
			t.Fatalf("COMPOZY_SESSION_ID = %q, want %q", got, "sess-1")
		}
	})
}
