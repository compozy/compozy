package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestStopTransitionsToStoppedAndNotifies(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) after Stop() = found, want missing", session.ID)
	}
	if got := h.notifier.stoppedCount(); got != 1 {
		t.Fatalf("stopped notifications = %d, want 1", got)
	}
	meta := readMeta(t, session.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}
	if got := session.Info().StopReason; got != store.StopUserCanceled {
		t.Fatalf("session.Info().StopReason = %q, want %q", got, store.StopUserCanceled)
	}

	events := readStoredEvents(t, session)
	stopEvent := storedEventByType(t, events, EventTypeSessionStopped)
	stopPayload := decodeStoredEventPayload(t, stopEvent)
	if got, want := stopPayload["stop_reason"], string(store.StopUserCanceled); got != want {
		t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
	}
}

func TestCompletedSpawnReapKeepsCleanTerminalProjection(t *testing.T) {
	t.Parallel()
	t.Run("Should keep a settled TTL reap clean across lifecycle projections", func(t *testing.T) {
		t.Parallel()

		wakeNotifier := &recordingSpawnWakeNotifier{}
		h := newHarness(t, WithSpawnWakeNotifier(wakeNotifier), WithSessionCatalog(newRecordingSessionCatalog()))
		parent := createSession(t, h)
		child, err := h.manager.Spawn(testutil.Context(t), SpawnOpts{
			ParentSessionID: parent.ID,
			AgentName:       "coder",
			Name:            "spawned-child",
			TTL:             time.Minute,
		})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}

		if err := h.manager.StopWithSpawnTTL(
			testutil.Context(t),
			child.ID,
			"spawn_reaper:ttl_expired",
		); err != nil {
			t.Fatalf("StopWithSpawnTTL() error = %v", err)
		}

		meta := readMeta(t, child.MetaPath())
		if meta.StopReason == nil || *meta.StopReason != store.StopCompleted {
			t.Fatalf("meta.StopReason = %#v, want %q", meta.StopReason, store.StopCompleted)
		}
		if got, want := meta.StopDetail, "spawn_reaper:ttl_expired"; got != want {
			t.Fatalf("meta.StopDetail = %q, want %q", got, want)
		}
		if meta.Failure != nil {
			t.Fatalf("meta.Failure = %#v, want nil for clean TTL reap", meta.Failure)
		}
		if got := countTranscriptMarkers(t, h.manager, child.ID, transcript.MarkerPromptTimeout); got != 0 {
			t.Fatalf("prompt timeout markers = %d, want 0 for clean TTL reap", got)
		}
		if got := h.notifier.stoppedCount(); got != 1 {
			t.Fatalf("stopped notifications = %d, want 1", got)
		}
		parents, events := wakeNotifier.calls()
		if len(events) != 1 || len(parents) != 1 {
			t.Fatalf("spawn wakes = parents %#v events %#v, want one clean wake", parents, events)
		}
		if parents[0] != parent.ID || events[0].Reason != SpawnWakeReasonStopped ||
			events[0].Badge != BadgeStopped {
			t.Fatalf("spawn wake = parents %#v events %#v, want stopped wake", parents, events)
		}
	})
}

func TestActivateAndWatchUpdatesStateAndStartsWatcher(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	sessionDir := filepath.Join(h.homePaths.SessionsDir, "sess-helper")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}

	dbPath := store.SessionDBFile(sessionDir)
	recorder, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner("sess-helper", h.workspaceID),
		dbPath,
	)
	if err != nil {
		t.Fatalf("OpenSessionDB() error = %v", err)
	}

	session := &Session{
		ID:                   "sess-helper",
		Name:                 "helper",
		AgentName:            "coder",
		WorkspaceID:          h.workspaceID,
		Workspace:            h.workspace,
		NetworkParticipation: testLocalParticipation(),
		Type:                 SessionTypeUser,
		State:                StateStarting,
		CreatedAt:            time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		UpdatedAt:            time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		sessionDir:           sessionDir,
		metaPath:             store.SessionMetaFile(sessionDir),
		dbPath:               dbPath,
		recorder:             recorder,
	}

	if err := h.manager.reserveStart(testutil.Context(t), session.ID, h.workspaceID); err != nil {
		t.Fatalf("reserveStart() error = %v", err)
	}

	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName: "coder",
		Command:   "fake-agent",
		Cwd:       h.workspace,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := h.manager.activateAndWatch(
		testutil.Context(t),
		session,
		proc,
		false,
		compozyconfig.ResolvedAgent{Name: "coder"},
		[]NetworkPeerCapability{},
		hookspkg.HookSessionPostCreate,
		false,
	); err != nil {
		t.Fatalf("activateAndWatch() error = %v", err)
	}

	if got := session.Info().State; got != StateActive {
		t.Fatalf("session state = %q, want %q", got, StateActive)
	}
	if got := session.Info().ACPSessionID; got != proc.SessionID {
		t.Fatalf("session ACPSessionID = %q, want %q", got, proc.SessionID)
	}
	if got, ok := h.manager.Get(session.ID); !ok || got != session {
		t.Fatalf("Get(%q) = (%v, %v), want active session", session.ID, got, ok)
	}
	if got := h.notifier.createdCount(); got != 1 {
		t.Fatalf("created notifications = %d, want 1", got)
	}
	if meta := readMeta(t, session.MetaPath()); meta.State != string(StateActive) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateActive)
	}

	h.driver.lastProcess().exit()
	h.notifier.waitForStopped(t, session.ID)
	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) found session after stopped notification", session.ID)
	}
}

func TestActivateAndWatchRollsBackOnMetaWriteFailure(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	sessionDir := filepath.Join(t.TempDir(), "session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}
	blockingPath := filepath.Join(sessionDir, "blocked-parent")
	if err := os.WriteFile(blockingPath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("WriteFile(blockingPath) error = %v", err)
	}

	recorder, err := h.manager.openStore(
		testutil.Context(t),
		testSessionDBOwner("sess-rollback", h.workspaceID),
		filepath.Join(sessionDir, "events.db"),
	)
	if err != nil {
		t.Fatalf("openStore() error = %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if closeErr := recorder.Close(closeCtx); closeErr != nil {
			t.Errorf("Close(rollback recorder) cleanup error = %v", closeErr)
		}
	})

	session := &Session{
		ID:          "sess-rollback",
		AgentName:   "coder",
		WorkspaceID: h.workspaceID,
		Workspace:   h.workspace,
		Type:        SessionTypeUser,
		State:       StateStarting,
		CreatedAt:   time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 6, 23, 0, 0, 0, time.UTC),
		sessionDir:  sessionDir,
		metaPath:    filepath.Join(blockingPath, "session.json"),
		dbPath:      filepath.Join(sessionDir, "events.db"),
		recorder:    recorder,
	}

	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{
		AgentName: "coder",
		Command:   "fake-agent",
		Cwd:       h.workspace,
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := h.manager.reserveStart(testutil.Context(t), session.ID, session.WorkspaceID); err != nil {
		t.Fatalf("reserveStart() error = %v", err)
	}

	if err := h.manager.activateAndWatch(
		testutil.Context(t),
		session,
		proc,
		false,
		compozyconfig.ResolvedAgent{Name: "coder"},
		[]NetworkPeerCapability{},
		hookspkg.HookSessionPostCreate,
		false,
	); err == nil {
		t.Fatal("activateAndWatch() error = nil, want non-nil")
	}
	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) = active session, want rollback", session.ID)
	}
	if got := session.Info().State; got != StateStarting {
		t.Fatalf("session state after rollback = %q, want %q", got, StateStarting)
	}
	if got := session.processHandle(); got != nil {
		t.Fatalf("session process after rollback = %#v, want nil", got)
	}
	if h.driver.stopCalls != 1 {
		t.Fatalf("driver stop calls = %d, want 1", h.driver.stopCalls)
	}
}

func TestCleanupFailedStartRemovesSessionDir(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	recorder := &fakeEventRecorder{}
	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{AgentName: "coder"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	sessionDir := filepath.Join(t.TempDir(), "failed-session")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sessionDir) error = %v", err)
	}

	if err := h.manager.cleanupFailedStart(sessionDir, recorder, proc); err != nil {
		t.Fatalf("cleanupFailedStart(with dir) error = %v", err)
	}
	if h.driver.stopCalls != 1 {
		t.Fatalf("driver stop calls = %d, want 1", h.driver.stopCalls)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder close calls = %d, want 1", recorder.closeCalls)
	}
	if _, err := os.Stat(sessionDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Stat(sessionDir) error = %v, want os.ErrNotExist", err)
	}
}

func TestCleanupFailedStartWithoutSessionDirSkipsRemoval(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	recorder := &fakeEventRecorder{}
	proc, err := h.driver.Start(testutil.Context(t), acp.StartOpts{AgentName: "coder"})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if err := h.manager.cleanupFailedStart("", recorder, proc); err != nil {
		t.Fatalf("cleanupFailedStart(without dir) error = %v", err)
	}
	if h.driver.stopCalls != 1 {
		t.Fatalf("driver stop calls = %d, want 1", h.driver.stopCalls)
	}
	if recorder.closeCalls != 1 {
		t.Fatalf("recorder close calls = %d, want 1", recorder.closeCalls)
	}
}

func TestAgentCrashTransitionsToStoppedAndNotifies(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	h.driver.lastProcess().crash(errors.New("boom"), "stderr trace")
	h.notifier.waitForStopped(t, session.ID)
	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) found session after stopped notification", session.ID)
	}

	meta := readMeta(t, session.MetaPath())
	if meta.State != string(StateStopped) {
		t.Fatalf("meta state = %q, want %q", meta.State, StateStopped)
	}
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopAgentCrashed)
	}

	events := readStoredEvents(t, session)
	if !containsEventType(events, acp.EventTypeError) {
		t.Fatalf("stored events missing crash error: %#v", events)
	}
	stopEvent := storedEventByType(t, events, EventTypeSessionStopped)
	stopPayload := decodeStoredEventPayload(t, stopEvent)
	if got, want := stopPayload["stop_reason"], string(store.StopAgentCrashed); got != want {
		t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
	}
}

func TestStopAndProcessExitFinalizeOnlyOnce(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	proceed := make(chan struct{})
	h.driver.stopHook = func(proc *fakeProcess) error {
		proc.crash(errors.New("boom"), "stderr trace")
		<-proceed
		return nil
	}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(testutil.Context(t), session.ID)
	}()

	h.notifier.waitForStopped(t, session.ID)
	close(proceed)

	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if got := h.notifier.stoppedCount(); got != 1 {
		t.Fatalf("stopped notifications = %d, want 1", got)
	}

	reopened, err := sessiondb.OpenSessionDB(
		testutil.Context(t),
		testSessionDBOwner(session.ID, session.WorkspaceID),
		session.DBPath(),
	)
	if err != nil {
		t.Fatalf("OpenSessionDB(reopen) error = %v", err)
	}
	defer func() {
		if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
			t.Errorf("Close(reopened session DB) cleanup error = %v", closeErr)
		}
	}()

	events, err := reopened.Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(reopened) error = %v", err)
	}
	if got := countEventType(events, EventTypeSessionStopped); got != 1 {
		t.Fatalf("countEventType(session_stopped) = %d, want 1", got)
	}
	meta := readMeta(t, session.MetaPath())
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopUserCanceled {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopUserCanceled)
	}
}

func TestStopWaitsForProcessDoneAfterSuccessfulDriverStop(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	release := make(chan struct{})
	h.driver.stopHook = func(proc *fakeProcess) error {
		go func() {
			<-release
			proc.exit()
		}()
		return nil
	}

	stopCtx, cancel := context.WithTimeout(testutil.Context(t), defaultLifecycleTimeout)
	defer cancel()

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(stopCtx, session.ID)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("Stop() completed early with %v, want it blocked on process exit", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)

	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	h.notifier.waitForStopped(t, session.ID)
	if _, ok := h.manager.Get(session.ID); ok {
		t.Fatalf("Get(%q) found session after stopped notification", session.ID)
	}
	if got := readMeta(t, session.MetaPath()).State; got != string(StateStopped) {
		t.Fatalf("meta.State = %q, want %q", got, StateStopped)
	}
}

func TestConcurrentCreateStopGet(t *testing.T) {
	h := newHarness(t)

	done := make(chan struct{})
	var readers sync.WaitGroup
	readers.Go(func() {
		for {
			select {
			case <-done:
				return
			default:
				_ = h.manager.List()
				for _, info := range h.manager.List() {
					h.manager.Get(info.ID)
				}
			}
		}
	})

	const total = 8
	var workers sync.WaitGroup
	for i := range total {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()

			session, err := h.manager.Create(testutil.Context(t), CreateOpts{
				AgentName: "coder",
				Name:      fmt.Sprintf("session-%d", index),
				Workspace: h.workspaceID,
			})
			if err != nil {
				t.Errorf("Create(%d) error = %v", index, err)
				return
			}
			if _, ok := h.manager.Get(session.ID); !ok {
				t.Errorf("Get(%q) = missing after Create()", session.ID)
			}
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop(%q) error = %v", session.ID, err)
			}
		}(i)
	}

	workers.Wait()
	close(done)
	readers.Wait()

	if list := h.manager.List(); len(list) != 0 {
		t.Fatalf("List() after concurrent stop = %d, want 0", len(list))
	}
}
