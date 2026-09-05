package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/sandbox"
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

func TestStoppedSessionDiscardsLatePromptEvents(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ batched, postRecord bool }{{false, false}, {true, false}, {false, true}, {true, true}} {
		batched := tc.batched
		t.Run(
			fmt.Sprintf(
				"Should discard delayed delivery when stop completes during a hook with batch %t and post-record %t",
				batched,
				tc.postRecord,
			),
			func(t *testing.T) {
				t.Parallel()
				entered, release := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				t.Cleanup(unblock)
				var enterOnce sync.Once
				waitForRelease := func(ctx context.Context, recordType string) error {
					if recordType != acp.EventTypeAgentMessage {
						return nil
					}
					enterOnce.Do(func() { close(entered) })
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				dispatcher := &spyHookDispatcher{}
				if tc.postRecord {
					dispatcher.dispatchEventPostRecordFn = func(ctx context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
						return payload, waitForRelease(ctx, payload.RecordType)
					}
				} else {
					dispatcher.dispatchEventPreRecordFn = func(ctx context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
						return payload, waitForRelease(ctx, payload.RecordType)
					}
				}
				h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
				active := createSession(t, h)
				ctx := testutil.Context(t)
				out := make(chan acp.AgentEvent, 2)
				finished := make(chan error, 1)
				go func() {
					turn := &promptTurnDispatchState{session: active, turnID: "overlap-turn"}
					loop := &promptPumpLoopState{}
					event := acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "delayed output"}
					var failure *store.SessionFailure
					var errorText string
					var stopPump bool
					if batched {
						failure, errorText, stopPump = h.manager.handlePromptPumpChunkBatch(
							ctx, ctx, active, turn, out, loop, []acp.AgentEvent{event}, &promptPumpFatal{},
						)
					} else {
						failure, errorText, stopPump = h.manager.handlePromptPumpEvent(
							ctx, ctx, active, turn, out, loop, event, false, &promptPumpFatal{},
						)
					}
					if failure != nil || errorText != "" || stopPump {
						finished <- fmt.Errorf("discard became prompt failure: %v %q stop=%t", failure, errorText, stopPump)
						return
					}
					finished <- nil
				}()
				select {
				case <-entered:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				if err := h.manager.Stop(ctx, active.ID); err != nil {
					t.Fatal(err)
				}
				before := active.Info()
				unblock()
				select {
				case err := <-finished:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				after := active.Info()
				if len(out) != 0 || after.State != StateStopped || after.StopReason != before.StopReason ||
					!reflect.DeepEqual(after.Failure, before.Failure) {
					t.Fatalf("post-stop event changed consumer or terminal state: %#v", after)
				}
				if got := countAgentEvents(
					h.notifier.eventsForSession(active.ID),
					acp.EventTypeAgentMessage,
				); got != 0 {
					t.Fatalf("post-stop agent message notifications = %d, want 0", got)
				}
				if got := countTranscriptMarkers(t, h.manager, active.ID, transcript.MarkerPostStop); got != 1 {
					t.Fatalf("post-stop markers = %d, want 1", got)
				}
				terminal := storedEventByType(t, readStoredEvents(t, active), EventTypeSessionStopped)
				for _, stored := range readStoredEvents(t, active) {
					if stored.Type == acp.EventTypeAgentMessage &&
						(!tc.postRecord || stored.Sequence >= terminal.Sequence) {
						t.Fatal("post-stop event persisted as agent output")
					}
				}
			},
		)
	}
	for _, eventType := range []string{acp.EventTypeAgentMessage, acp.EventTypeError, "batch"} {
		batched := eventType == "batch"
		name := "Should discard a late " + eventType + " without changing the stopped session"
		if batched {
			name = "Should discard a late chunk batch without changing the stopped session"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			active := createSession(t, h)
			ctx := testutil.Context(t)
			if err := h.manager.Stop(ctx, active.ID); err != nil {
				t.Fatalf("Stop() error = %v", err)
			}
			catalog, cancelCatalog, err := h.manager.SubscribeSessionCatalogEvents(ctx,
				CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: h.workspaceID})
			if err != nil {
				t.Fatal(err)
			}
			defer cancelCatalog()
			before := active.Info()
			turn := &promptTurnDispatchState{session: active, turnID: "late-turn"}
			loop := &promptPumpLoopState{}
			out := make(chan acp.AgentEvent, 2)
			event := acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "late output"}
			if eventType == acp.EventTypeError {
				event = acp.AgentEvent{
					Type:    eventType,
					Error:   "late failure",
					Failure: &store.SessionFailure{Kind: store.FailureProcess, Summary: "late failure"},
				}
			}
			var failure *store.SessionFailure
			var errorText string
			var stopPump bool
			if batched {
				failure, errorText, stopPump = h.manager.handlePromptPumpChunkBatch(
					ctx, ctx, active, turn, out, loop, []acp.AgentEvent{event, event}, &promptPumpFatal{},
				)
			} else {
				failure, errorText, stopPump = h.manager.handlePromptPumpEvent(
					ctx, ctx, active, turn, out, loop, event, false, &promptPumpFatal{},
				)
			}
			if failure != nil || errorText != "" || stopPump {
				t.Fatalf("late event became a prompt failure: %#v, %q", failure, errorText)
			}
			if len(out) != 0 {
				t.Fatal("late output reached the consumer")
			}
			after := active.Info()
			if after.State != StateStopped || after.StopReason != before.StopReason ||
				!reflect.DeepEqual(after.Failure, before.Failure) ||
				!after.UpdatedAt.Equal(before.UpdatedAt) ||
				h.notifier.stoppedCount() != 1 {
				t.Fatalf("late event changed terminal state: before %#v, after %#v", before, after)
			}
			if got := countTranscriptMarkers(t, h.manager, active.ID, transcript.MarkerPostStop); got != 1 {
				t.Fatalf("post-stop markers = %d, want 1", got)
			}
			select {
			case wake := <-catalog:
				if wake.Kind != CatalogEventUpserted || wake.SessionID != active.ID ||
					wake.WorkspaceID != h.workspaceID {
					t.Fatalf("post-stop catalog wake = %#v", wake)
				}
			default:
				t.Fatal("durable post-stop marker did not wake catalog consumers")
			}
			for _, stored := range readStoredEvents(t, active) {
				if stored.Type == acp.EventTypeAgentMessage {
					t.Fatal("late output persisted as an agent message")
				}
			}
		})
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

func TestStopRequiresVerifiedProcessExit(t *testing.T) {
	t.Parallel()
	t.Run("Should retain stopping until process death is verified", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		h.driver.mu.Lock()
		h.driver.verifyExitHook = func(*AgentProcess) (bool, error) { return false, nil }
		h.driver.mu.Unlock()
		t.Cleanup(func() { h.driver.mu.Lock(); h.driver.verifyExitHook = nil; h.driver.mu.Unlock() })
		h.driver.stopHook = func(*fakeProcess) error {
			if !active.Info().StopEscalated || !readMeta(t, active.MetaPath()).StopEscalated {
				t.Error("forced signal preceded durable escalation")
			}
			if got := countEventType(readStoredEvents(t, active), eventspkg.SessionStopEscalated); got != 1 {
				t.Errorf("escalation events before forced signal = %d", got)
			}
			return errors.New("forced stop remains unverified")
		}
		err := h.manager.Stop(t.Context(), active.ID)
		if !errors.Is(err, ErrStopVerificationFailed) {
			t.Fatalf("Stop() = %v, want verification failure", err)
		}
		if active.Info().State != StateStopping {
			t.Fatalf("state = %s, want stopping", active.Info().State)
		}
		if !active.Info().StopEscalated || !readMeta(t, active.MetaPath()).StopEscalated {
			t.Fatal("exhausted stop escalation was not persisted")
		}
		if BadgeForInfo(active.Info()) != BadgeNeedsAttention ||
			ClassForBadge(BadgeForInfo(active.Info())) != AttentionNeedsYou {
			t.Fatal("unverified stop did not require nonterminal attention")
		}
		if !active.Info().StopVerificationFailed || !readMeta(t, active.MetaPath()).StopVerificationFailed {
			t.Fatal("exhausted stop attention was not persisted")
		}
		if meta := readMeta(t, active.MetaPath()); meta.State != string(StateStopping) {
			t.Fatalf("persisted state = %s, want stopping", meta.State)
		}
		if got := h.notifier.stoppedCount(); got != 0 {
			t.Fatalf("stopped notifications = %d", got)
		}
		events, err := active.recorderHandle().Query(t.Context(), store.EventQuery{})
		if err != nil {
			t.Fatal(err)
		}
		if got := countEventType(events, EventTypeSessionStopped); got != 0 {
			t.Fatalf("terminal events = %d", got)
		}
		if countEventType(events, eventspkg.SessionStopEscalated) != 2 ||
			countEventType(events, eventspkg.SessionStopVerificationFailed) != 1 {
			t.Fatal("exhausted stop did not persist both escalation phases and verification failure")
		}
		for _, stored := range events {
			if stored.Type != eventspkg.SessionStopEscalated && stored.Type != eventspkg.SessionStopVerificationFailed {
				continue
			}
			event, err := transcript.UnmarshalAgentEvent(stored.Content)
			if err != nil {
				t.Fatal(err)
			}
			var payload stopEventPayload
			if err := json.Unmarshal(event.Raw, &payload); err != nil {
				t.Fatal(err)
			}
			if payload.WorkspaceID != active.WorkspaceID || payload.SessionID != active.ID ||
				payload.TurnID != stored.TurnID || payload.Scope != "session" || payload.Cause != CauseUserRequested {
				t.Fatalf("stop correlation = %#v", payload)
			}
			if stored.Type == eventspkg.SessionStopVerificationFailed &&
				payload.ReasonCode != "stop_verification_failed" {
				t.Fatalf("verification diagnostic = %#v", payload)
			}
		}

		if err := h.manager.WaitForFinalizations(t.Context()); err != nil {
			t.Fatal(err)
		}
		h.driver.mu.Lock()
		h.driver.verifyExitHook = nil
		h.driver.mu.Unlock()
		if err := h.manager.Stop(t.Context(), active.ID); err != nil {
			t.Fatalf("verified retry: %v", err)
		}
		if active.Info().StopVerificationFailed || readMeta(t, active.MetaPath()).StopVerificationFailed {
			t.Fatal("verified retry retained stop attention")
		}
		if got := h.notifier.stoppedCount(); got != 1 {
			t.Fatalf("verified stopped notifications = %d, want 1", got)
		}
	})
}

func TestSharedSessionStopOperation(t *testing.T) {
	t.Run("Should defer the terminal event until its recovery receipt is durable", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		path := h.manager.recoveredStopReceiptPath(active.ID)
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		removeObstruction := sync.OnceFunc(func() {
			if err := os.Remove(path); err != nil {
				t.Error(err)
			}
		})
		t.Cleanup(removeObstruction)
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		first, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if !errors.Is(err, ErrRecoveryPersistence) || !first.Verified || first.FinalState != StateStopping {
			t.Fatalf("undurable receipt outcome = %#v, %v", first, err)
		}
		if countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 0 || h.notifier.stoppedCount() != 0 {
			t.Fatal("terminal publication bypassed durable recovery receipt")
		}
		removeObstruction()
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		settled, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if err != nil || !settled.Verified || settled.FinalState != StateStopped || settled.Phase != first.Phase {
			t.Fatalf("receipt retry = %#v, %v", settled, err)
		}
		if countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 1 ||
			h.notifier.stoppedCount() != 1 {
			t.Fatal("receipt retry omitted or duplicated terminal publication")
		}
	})

	for _, stage := range []string{"event", "ack", "state"} {
		t.Run("Should recover active stop after manager restart at "+stage+" persistence", func(t *testing.T) {
			t.Parallel()
			catalog := newRecordingSessionCatalog()
			h := newHarness(t, WithSessionCatalog(catalog))
			active := createSession(t, h)
			var fail atomic.Bool
			fail.Store(true)
			catalog.mu.Lock()
			catalog.updateHook = func(update store.SessionStateUpdate) error {
				if stage == "state" && update.State == string(StateStopped) && fail.Load() {
					return errors.New("catalog unavailable")
				}
				return nil
			}
			catalog.mu.Unlock()
			if stage != "state" {
				writeErr := errors.New("terminal writer unavailable")
				active.setRecorder(&terminalWriteFailingRecorder{
					EventRecorder: active.recorderHandle(),
					fail:          &fail,
					writeErr:      writeErr,
					afterCommit:   stage == "ack",
				})
				openStore := h.manager.openStore
				h.manager.openStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
					if fail.Load() {
						return nil, writeErr
					}
					return openStore(ctx, owner, path)
				}
			}
			if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
				t.Fatal(err)
			}
			first, err := h.manager.AwaitStopped(t.Context(), active.ID)
			if !errors.Is(err, ErrRecoveryPersistence) || !first.Verified {
				t.Fatalf("pending stop = %#v, %v", first, err)
			}
			fail.Store(false)
			if err := h.manager.closeSessionRecorder(active); err != nil {
				t.Fatal(err)
			}
			h.manager.removeActive(active.ID)
			manager := newManagerWithHarness(t, h)
			cleanupTestManager(t, manager)
			if err := manager.RecoverPendingStops(t.Context()); err != nil {
				t.Fatal(err)
			}
			settled, err := manager.AwaitStopped(t.Context(), active.ID)
			if err != nil || !settled.Verified || settled.FinalState != StateStopped || settled.Phase != first.Phase ||
				settled.Cause != first.Cause || settled.Elapsed != first.Elapsed {
				t.Fatalf("restarted settlement = %#v, %v", settled, err)
			}
			if countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 1 ||
				h.notifier.stoppedCount() != 1 {
				t.Fatal("restart duplicated or omitted terminal settlement")
			}
			if _, err := os.Stat(manager.recoveredStopReceiptPath(active.ID)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("settled receipt retained: %v", err)
			}
		})
	}

	for _, metadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should retry final stop state with metadata failure %t", metadata), func(t *testing.T) {
			t.Parallel()
			catalog := newRecordingSessionCatalog()
			h := newHarness(t, WithSessionCatalog(catalog))
			active := createSession(t, h)
			var fail atomic.Bool
			fail.Store(true)
			persistErr := errors.New("final stop state unavailable")
			backup := active.MetaPath() + ".saved"
			var metadataMoved atomic.Bool
			restoreMetadata := sync.OnceFunc(func() {
				if !metadataMoved.Load() {
					return
				}
				if err := os.Remove(active.MetaPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
					t.Error(err)
				}
				if err := os.Rename(backup, active.MetaPath()); err != nil {
					t.Error(err)
				}
			})
			t.Cleanup(restoreMetadata)
			catalog.mu.Lock()
			catalog.updateHook = func(update store.SessionStateUpdate) error {
				if !metadata && update.State == string(StateStopped) && fail.Load() {
					return persistErr
				}
				return nil
			}
			catalog.mu.Unlock()
			var cleanupCalls atomic.Int32
			h.notifier.mu.Lock()
			h.notifier.finalizingHook = func(context.Context, *Session) {
				cleanupCalls.Add(1)
				if metadata {
					if err := os.Rename(active.MetaPath(), backup); err != nil {
						t.Error(err)
						return
					}
					metadataMoved.Store(true)
					if err := os.Mkdir(active.MetaPath(), 0o700); err != nil {
						t.Error(err)
					}
				}
			}
			h.notifier.mu.Unlock()
			if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
				t.Fatal(err)
			}
			first, err := h.manager.AwaitStopped(t.Context(), active.ID)
			if !errors.Is(err, ErrRecoveryPersistence) || (!metadata && !errors.Is(err, persistErr)) ||
				!first.Verified || first.FinalState != StateStopping ||
				h.notifier.stoppedCount() != 0 {
				t.Fatalf("pending final state = %#v, %v", first, err)
			}
			if _, err := h.manager.Resume(t.Context(), active.ID); !errors.Is(err, ErrRecoveryPersistence) {
				t.Fatalf("resume bypassed final-state persistence: %v", err)
			}
			if err := h.manager.Delete(t.Context(), active.ID); !errors.Is(err, ErrRecoveryPersistence) {
				t.Fatalf("delete bypassed final-state persistence: %v", err)
			}
			fail.Store(false)
			restoreMetadata()
			if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
				t.Fatal(err)
			}
			settled, err := h.manager.AwaitStopped(t.Context(), active.ID)
			if err != nil || !settled.Verified || settled.FinalState != StateStopped || settled.Phase != first.Phase {
				t.Fatalf("final-state retry = %#v, %v", settled, err)
			}
			if cleanupCalls.Load() != 1 || h.notifier.stoppedCount() != 1 ||
				countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 1 {
				t.Fatal("final-state retry repeated cleanup or terminal publication")
			}
			if readMeta(t, active.MetaPath()).State != string(StateStopped) {
				t.Fatal("final state is not durable")
			}
		})
	}
	t.Run("Should persist the terminal event with a fresh writer when the prompt recorder fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		var fail atomic.Bool
		fail.Store(true)
		writeErr := errors.New("prompt recorder unavailable")
		active.setRecorder(
			&terminalWriteFailingRecorder{EventRecorder: active.recorderHandle(), fail: &fail, writeErr: writeErr},
		)
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		outcome, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if !errors.Is(err, writeErr) || !outcome.Verified || outcome.FinalState != StateStopped {
			t.Fatalf("durable fallback outcome = %#v, %v", outcome, err)
		}
		if h.notifier.stoppedCount() != 1 || countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 1 {
			t.Fatal("fresh writer did not persist and publish one terminal outcome")
		}
	})
	for _, afterCommit := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should retry terminal event persistence with committed failure %t", afterCommit),
			func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				active := createSession(t, h)
				var fail atomic.Bool
				fail.Store(true)
				persistErr := errors.New("terminal event unavailable")
				openStore := h.manager.openStore
				h.manager.openStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
					if fail.Load() {
						return nil, persistErr
					}
					return openStore(ctx, owner, path)
				}
				active.setRecorder(&terminalWriteFailingRecorder{
					EventRecorder: active.recorderHandle(), fail: &fail, writeErr: persistErr, afterCommit: afterCommit,
				})
				if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
					t.Fatal(err)
				}
				first, err := h.manager.AwaitStopped(t.Context(), active.ID)
				if !errors.Is(err, persistErr) || !first.Verified || first.FinalState != StateStopping {
					t.Fatalf("pending terminal write = %#v, %v", first, err)
				}
				if h.notifier.stoppedCount() != 0 {
					t.Fatal("failed write announced stopped")
				}
				fail.Store(false)
				if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
					t.Fatal(err)
				}
				settled, err := h.manager.AwaitStopped(t.Context(), active.ID)
				if err != nil || !settled.Verified || settled.FinalState != StateStopped ||
					settled.Phase != first.Phase {
					t.Fatalf("terminal retry = %#v, %v", settled, err)
				}
				if h.notifier.stoppedCount() != 1 ||
					countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 1 {
					t.Fatal("retry duplicated terminal event or notification")
				}
			},
		)
	}
	t.Run("Should retry active stop classification persistence without repeating termination", func(t *testing.T) {
		t.Parallel()
		catalog := newRecordingSessionCatalog()
		var postStops atomic.Int32
		dispatcher := &spyHookDispatcher{
			dispatchSessionPostStopFn: func(_ context.Context, payload hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error) {
				postStops.Add(1)
				return payload, nil
			},
		}
		h := newHarness(t, WithSessionCatalog(catalog), WithHookSet(fullHookSet(dispatcher)))
		active := createSession(t, h)
		catalogErr := errors.New("stop classification unavailable")
		var fail atomic.Bool
		fail.Store(true)
		catalog.mu.Lock()
		catalog.updateHook = func(update store.SessionStateUpdate) error {
			if update.StopReason != nil && fail.Load() {
				return catalogErr
			}
			return nil
		}
		catalog.mu.Unlock()
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		first, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if !errors.Is(err, catalogErr) || !first.Verified || first.FinalState != StateStopping {
			t.Fatalf("pending classification = %#v, %v", first, err)
		}
		if _, ok := h.manager.Get(active.ID); !ok || h.notifier.stoppedCount() != 0 {
			t.Fatal("failed classification removed or announced the terminal session")
		}
		if countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 0 {
			t.Fatal("terminal event preceded durable classification")
		}
		h.driver.mu.Lock()
		cancels, stops, kills := h.driver.cancelCalls, h.driver.stopCalls, h.driver.killCalls
		h.driver.mu.Unlock()
		fail.Store(false)
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		settled, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if err != nil || !settled.Verified || settled.FinalState != StateStopped || settled.Phase != first.Phase {
			t.Fatalf("settled classification = %#v, %v", settled, err)
		}
		h.driver.mu.Lock()
		repeated := h.driver.cancelCalls != cancels || h.driver.stopCalls != stops || h.driver.killCalls != kills
		h.driver.mu.Unlock()
		if repeated || postStops.Load() != 1 || h.notifier.stoppedCount() != 1 ||
			countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 1 {
			t.Fatal("classification retry repeated process termination or terminal publication")
		}
	})
	for _, persistFailure := range []bool{false, true} {
		name := "Should preserve remote orphan attention while allowing boot after durable diagnosis"
		if persistFailure {
			name = "Should reject boot when remote orphan diagnostics cannot persist"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			active := seedRecoveredRemoteStop(t, h)
			persistErr := errors.New("orphan diagnostic store unavailable")
			if persistFailure {
				h.manager.openStore = func(context.Context, store.SessionDBOwner, string) (EventRecorder, error) {
					return nil, persistErr
				}
			}
			ctx := testutil.Context(t)
			err := h.manager.RecoverPendingStops(ctx)
			if persistFailure {
				if !errors.Is(err, ErrRecoveryPersistence) || !errors.Is(err, persistErr) {
					t.Fatalf("boot hid orphan diagnostic persistence failure: %v", err)
				}
			} else if err != nil {
				t.Fatalf("durable unverified attention blocked boot: %v", err)
			}
			outcome, err := h.manager.AwaitStopped(ctx, active.ID)
			if !errors.Is(err, ErrStopVerificationFailed) || outcome.Verified || outcome.FinalState != StateStopping {
				t.Fatalf("boot manufactured remote exit proof: %#v, %v", outcome, err)
			}
			meta := readMeta(t, active.MetaPath())
			if meta.State != string(StateStopping) || !meta.StopVerificationFailed || h.notifier.stoppedCount() != 1 {
				t.Fatal("boot lost durable attention or notified a false terminal state")
			}
		})
	}

	for _, tc := range []struct {
		name        string
		mode        string
		wantSignals []syscall.Signal
		verified    bool
	}{
		{"Should skip signals for a reused orphan PID", "reused", nil, true},
		{"Should refuse signals when orphan identity lookup fails", "unknown", nil, false},
		{"Should stop after a verified graceful orphan exit", "graceful", []syscall.Signal{syscall.SIGTERM}, true},
		{"Should not kill a replacement orphan identity after TERM", "replacement", []syscall.Signal{syscall.SIGTERM}, true},
		{"Should attempt KILL after orphan TERM fails and retain unverified death", "failed", []syscall.Signal{syscall.SIGTERM, syscall.SIGKILL}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			started := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
			observed, exited := started, false
			if tc.mode == "reused" {
				observed = started.Add(time.Hour)
			}
			identityErr, signalErr := errors.New("identity unavailable"), errors.New("signal failed")
			verify := func(*AgentProcess) (bool, error) {
				if tc.mode == "unknown" {
					return false, identityErr
				}
				return exited || !procutil.MatchesObservedStartTime(observed, started), nil
			}
			var signals []syscall.Signal
			signalProcess := func(pid int, signal syscall.Signal) error {
				if pid != 1001 {
					return fmt.Errorf("unexpected orphan signal target: %d", pid)
				}
				signals = append(signals, signal)
				switch tc.mode {
				case "replacement":
					observed = started.Add(time.Hour)
				case "graceful":
					exited = true
				}
				if tc.mode == "failed" {
					return signalErr
				}
				return nil
			}
			proc := NewAgentProcess(AgentProcessOptions{Done: make(chan struct{})})
			proc.PID, proc.StartedAt = 1001, started
			outcome, err := h.manager.runTerminationLadder(testutil.Context(t), proc, terminationTarget{
				cooperative: func(context.Context, *AgentProcess) error { return ErrSessionNotActive },
				forced:      recoveredProcessSignal(syscall.SIGTERM, verify, signalProcess),
				kill:        recoveredProcessSignal(syscall.SIGKILL, verify, signalProcess),
				verifyExit:  verify,
			})
			if outcome.Verified != tc.verified || (err == nil) != tc.verified ||
				!slices.Equal(signals, tc.wantSignals) {
				t.Fatalf("orphan ladder signals=%v outcome=%#v error=%v", signals, outcome, err)
			}
			if tc.mode == "unknown" && !errors.Is(err, identityErr) {
				t.Fatal("orphan ladder lost identity lookup error")
			}
			if tc.mode == "failed" && (!errors.Is(err, signalErr) || !errors.Is(err, ErrStopVerificationFailed)) {
				t.Fatal("orphan ladder lost failed signal or unverified exit diagnostic")
			}
		})
	}

	for _, tc := range []struct {
		previousState State
		restart       bool
	}{
		{"", false}, {StateStopping, false}, {StateActive, false}, {StateActive, true},
		{StateStarting, false}, {StateStarting, true}, {"starting-live", false}, {"starting-live", true},
	} {
		previousState := tc.previousState
		exitedBeforeBoot := previousState != "" && previousState != "starting-live"
		if previousState == "starting-live" {
			previousState = StateStarting
		}
		t.Run(
			fmt.Sprintf(
				"Should recover an orphan from %s with restart %t through shared settlement and retain crash attribution",
				tc.previousState,
				tc.restart,
			),
			func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				active := seedRecoveredLocalStop(t, h)
				ctx := testutil.Context(t)
				if previousState != "" {
					meta := readMeta(t, active.MetaPath())
					if exitedBeforeBoot {
						if err := procutil.KillProcessGroupIDAndWait(
							meta.Liveness.SubprocessPID,
							time.Second,
						); err != nil {
							t.Fatal(err)
						}
					}
					meta.State = string(previousState)
					if previousState == StateStarting {
						meta.Failure = nil
					}
					if err := store.WriteSessionMeta(active.MetaPath(), meta); err != nil {
						t.Fatal(err)
					}
				}
				manager := h.manager
				if tc.restart {
					if !exitedBeforeBoot {
						if _, err := manager.ListAll(ctx); err != nil {
							t.Fatal(err)
						}
					} else {
						meta := readMeta(t, active.MetaPath())
						classified, changed := ClassifyInactiveMetaForRecovery(manager.now(), meta)
						if !changed {
							t.Fatal("expected interrupted metadata classification")
						}
						if err := manager.prepareClassifiedExitSettlement(&meta, &classified); err != nil {
							t.Fatal(err)
						}
						// Crash boundary: the receipt is durable while session metadata remains active.
					}
					manager = newManagerWithHarness(t, h)
					cleanupTestManager(t, manager)
				}
				before := h.notifier.stoppedCount()
				if err := manager.RecoverPendingStops(ctx); err != nil {
					t.Fatal(err)
				}
				outcome, err := manager.AwaitStopped(ctx, active.ID)
				wantCause, wantReason := CauseProcessExited, store.StopAgentCrashed
				if previousState == StateStarting {
					wantCause, wantReason = CauseFailed, store.StopError
				}
				if err != nil || !outcome.Verified || outcome.Escalated == exitedBeforeBoot ||
					outcome.Cause != wantCause {
					t.Fatalf("orphan recovery did not use verified shared stop: %#v, %v", outcome, err)
				}
				meta := readMeta(t, active.MetaPath())
				if meta.State != string(StateStopped) || meta.StopReason == nil ||
					*meta.StopReason != wantReason {
					t.Fatalf("orphan recovery lost crash attribution: %s, %v", meta.State, meta.StopReason)
				}
				if previousState == StateStarting && (meta.ACPSessionID != nil || meta.Failure == nil ||
					meta.Failure.Kind != store.FailureStartup || meta.StopDetail != resumeStopDetailStartIncomplete) {
					t.Fatalf("startup recovery lost failure or retained incomplete ACP identity: %#v", meta)
				}
				exited, err := procutil.VerifyProcessExit(
					meta.Liveness.SubprocessPID,
					*meta.Liveness.SubprocessStartedAt,
				)
				if err != nil || !exited {
					t.Fatalf("orphan remains alive after recovery: %v", err)
				}
				stored := readStoredEvents(t, active)
				escalations := 2
				if exitedBeforeBoot {
					escalations = 1
				}
				if countEventType(stored, eventspkg.SessionStopEscalated) != escalations ||
					countEventType(stored, EventTypeSessionStopped) != 2 || h.notifier.stoppedCount() != before+1 {
					t.Fatal("orphan recovery omitted canonical escalation, terminal event or notification")
				}
				if err := manager.RecoverPendingStops(ctx); err != nil {
					t.Fatal(err)
				}
				if h.notifier.stoppedCount() != before+1 || len(readStoredEvents(t, active)) != len(stored) {
					t.Fatal("repeated boot recovery duplicated settled events or notification")
				}
			},
		)
	}

	for _, invalid := range []string{"malformed", "version", "workspace", "generation"} {
		t.Run("Should preserve and reject a recovered stop receipt with invalid "+invalid, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			active := createSession(t, h)
			ctx := testutil.Context(t)
			if err := h.manager.Stop(ctx, active.ID); err != nil {
				t.Fatal(err)
			}
			meta := readMeta(t, active.MetaPath())
			receipt := recoveredStopReceipt{
				Version: 1, SessionID: meta.ID, WorkspaceID: meta.WorkspaceID,
				RuntimeGeneration: meta.RuntimeGeneration, CreatedAt: meta.CreatedAt,
				TurnID: "pending-stop", StartedAt: h.manager.now(),
				Outcome: StopOutcome{FinalState: StateStopped, Verified: true, Cause: CauseUserRequested},
			}
			switch invalid {
			case "version":
				receipt.Version++
			case "workspace":
				receipt.WorkspaceID = "another-workspace"
			case "generation":
				receipt.RuntimeGeneration++
			}
			content, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			if invalid == "malformed" {
				content = []byte("{")
			}
			path := h.manager.recoveredStopReceiptPath(active.ID)
			if err := os.WriteFile(path, content, 0o600); err != nil {
				t.Fatal(err)
			}
			restarted := newManagerWithHarness(t, h)
			cleanupTestManager(t, restarted)
			if err := restarted.RecoverPendingStops(ctx); !errors.Is(err, ErrRecoveryPersistence) {
				t.Fatalf("recovery accepted invalid receipt: %v", err)
			}
			if _, err := restarted.Resume(ctx, active.ID); !errors.Is(err, ErrRecoveryPersistence) {
				t.Fatalf("resume accepted invalid receipt: %v", err)
			}
			preserved, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(preserved, content) {
				t.Fatalf("recovery discarded invalid receipt: %v", err)
			}
		})
	}

	t.Run("Should recover verified terminal settlement after manager restart", func(t *testing.T) {
		t.Parallel()
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := seedRecoveredLocalStop(t, h)
		ctx := testutil.Context(t)
		catalogErr := errors.New("terminal catalog unavailable before restart")
		catalog.mu.Lock()
		catalog.updateHook = func(update store.SessionStateUpdate) error {
			if update.State == string(StateStopped) {
				return catalogErr
			}
			return nil
		}
		catalog.mu.Unlock()
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		original, err := h.manager.AwaitStopped(ctx, active.ID)
		if !errors.Is(err, catalogErr) || !original.Verified {
			t.Fatalf("pre-restart terminal settlement = %#v, %v", original, err)
		}
		before := h.notifier.stoppedCount()
		catalog.mu.Lock()
		catalog.updateHook = nil
		catalog.mu.Unlock()
		restarted := newManagerWithHarness(t, h, WithSessionCatalog(catalog))
		cleanupTestManager(t, restarted)
		if _, err := restarted.Resume(ctx, active.ID); !errors.Is(err, ErrRecoveryPersistence) {
			t.Fatalf("direct resume bypassed durable pending settlement: %v", err)
		}
		if err := restarted.RecoverPendingStops(ctx); err != nil {
			t.Fatal(err)
		}
		if err := restarted.RequestStop(ctx, active.ID, CauseShutdown); err != nil {
			t.Fatal(err)
		}
		settled, err := restarted.AwaitStopped(ctx, active.ID)
		if err != nil || settled != original {
			t.Fatalf("restart lost original stop outcome: %#v, %v; want %#v", settled, err, original)
		}
		if h.notifier.stoppedCount() != before+1 ||
			countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 2 {
			t.Fatal("restart did not settle one durable terminal event and notification")
		}
	})

	t.Run("Should settle a committed recovered terminal event after recorder close fails", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := seedRecoveredLocalStop(t, h)
		ctx := testutil.Context(t)
		closeErr := errors.New("terminal recorder close failed after commit")
		var fail atomic.Bool
		fail.Store(true)
		opener := h.manager.openStore
		h.manager.openStore = func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
			recorder, err := opener(ctx, owner, path)
			if err != nil {
				return nil, err
			}
			return &terminalCloseFailingRecorder{EventRecorder: recorder, fail: &fail, closeErr: closeErr}, nil
		}
		before := h.notifier.stoppedCount()
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		original, err := h.manager.AwaitStopped(ctx, active.ID)
		if !errors.Is(err, closeErr) || !original.Verified {
			t.Fatalf("committed terminal event lost close error or exit proof: %#v, %v", original, err)
		}
		committed := readStoredEvents(t, active)
		if countEventType(committed, EventTypeSessionStopped) != 2 || h.notifier.stoppedCount() != before {
			t.Fatal("failed close did not retain the committed event before notification")
		}
		if err := h.manager.RequestStop(ctx, active.ID, CauseShutdown); err != nil {
			t.Fatal(err)
		}
		settled, err := h.manager.AwaitStopped(ctx, active.ID)
		if err != nil || settled != original {
			t.Fatalf("committed terminal retry changed original outcome: %#v, %v", settled, err)
		}
		if got := readStoredEvents(t, active); !reflect.DeepEqual(got, committed) {
			t.Fatal("terminal retry changed or duplicated committed events")
		}
		if h.notifier.stoppedCount() != before+1 {
			t.Fatal("terminal retry did not notify exactly once")
		}
		if err := h.manager.Stop(ctx, active.ID); err != nil {
			t.Fatal(err)
		}
		if h.notifier.stoppedCount() != before+1 {
			t.Fatal("completed retry duplicated terminal notification")
		}
	})

	t.Run("Should retain recovered exit proof while terminal metadata is unavailable", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := seedRecoveredLocalStop(t, h)
		ctx := testutil.Context(t)
		metadataErr := errors.New("terminal metadata unavailable")
		var fail atomic.Bool
		fail.Store(true)
		h.manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			meta, err := store.ReadSessionMeta(path)
			if err != nil {
				return meta, err
			}
			if fail.Load() && meta.State == string(StateStopping) {
				exited, err := procutil.VerifyProcessExit(
					meta.Liveness.SubprocessPID,
					*meta.Liveness.SubprocessStartedAt,
				)
				if err == nil && exited {
					return store.SessionMeta{}, metadataErr
				}
			}
			return meta, nil
		}
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		original, err := h.manager.AwaitStopped(ctx, active.ID)
		if !errors.Is(err, metadataErr) || !original.Verified {
			t.Fatalf("metadata failure lost exit proof: %#v, %v", original, err)
		}
		fail.Store(false)
		restarted := newManagerWithHarness(t, h)
		cleanupTestManager(t, restarted)
		if err := restarted.RequestStop(ctx, active.ID, CauseShutdown); err != nil {
			t.Fatal(err)
		}
		settled, err := restarted.AwaitStopped(ctx, active.ID)
		if err != nil || settled != original {
			t.Fatalf("settlement retry changed original outcome: %#v, %v", settled, err)
		}
		meta := readMeta(t, active.MetaPath())
		if meta.State != string(StateStopped) || meta.StopReason == nil || *meta.StopReason != store.StopUserCanceled {
			t.Fatal("retry reclassified the original verified stop")
		}
	})

	t.Run("Should retry recovered terminal catalog persistence without repeating termination", func(t *testing.T) {
		t.Parallel()
		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		active := seedRecoveredLocalStop(t, h)
		ctx := testutil.Context(t)
		catalogErr := errors.New("terminal catalog write failed")
		var fail atomic.Bool
		fail.Store(true)
		catalog.mu.Lock()
		catalog.updateHook = func(update store.SessionStateUpdate) error {
			if update.State == string(StateStopped) && fail.Load() {
				return catalogErr
			}
			return nil
		}
		catalog.mu.Unlock()
		before := h.notifier.stoppedCount()
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		original, err := h.manager.AwaitStopped(ctx, active.ID)
		if !errors.Is(err, catalogErr) || !original.Verified {
			t.Fatalf("failed terminal persistence = %#v, %v", original, err)
		}
		if _, err := h.manager.Resume(ctx, active.ID); !errors.Is(err, ErrRecoveryPersistence) {
			t.Fatalf("resume bypassed pending terminal persistence: %v", err)
		}
		if _, err := h.manager.ClearConversation(ctx, active.ID); !errors.Is(err, ErrRecoveryPersistence) {
			t.Fatalf("clear bypassed pending terminal persistence: %v", err)
		}
		if err := h.manager.Delete(ctx, active.ID); !errors.Is(err, ErrRecoveryPersistence) {
			t.Fatalf("delete bypassed pending terminal persistence: %v", err)
		}
		preparation, removalErr := h.manager.PrepareWorkspaceRemoval(ctx, h.workspaceID)
		if preparation != nil {
			if err := preparation.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}
		if !errors.Is(removalErr, ErrRecoveryPersistence) {
			t.Fatalf("workspace removal bypassed pending terminal persistence: %v", removalErr)
		}
		beforeRetry := readStoredEvents(t, active)
		fail.Store(false)
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		settled, err := h.manager.AwaitStopped(ctx, active.ID)
		if err != nil || !settled.Verified || settled.FinalState != StateStopped || settled.Phase != original.Phase {
			t.Fatalf("terminal persistence retry = %#v, %v", settled, err)
		}
		catalog.mu.Lock()
		state := catalog.sessions[active.ID].State
		catalog.mu.Unlock()
		if state != string(StateStopped) || h.notifier.stoppedCount() != before+1 {
			t.Fatal("terminal retry did not reconcile catalog and notification")
		}
		afterRetry := readStoredEvents(t, active)
		if countEventType(
			afterRetry,
			eventspkg.SessionStopEscalated,
		) != countEventType(
			beforeRetry,
			eventspkg.SessionStopEscalated,
		) ||
			countEventType(afterRetry, EventTypeSessionStopped) != 2 {
			t.Fatal("terminal retry repeated escalation or terminal events")
		}
	})

	t.Run("Should preserve recovered cleanup errors for concurrent stop waiters", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := seedRecoveredLocalStop(t, h)
		ctx := testutil.Context(t)
		entered, release := make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		t.Cleanup(unblock)
		cleanupErr := errors.New("recovered ledger persistence failed")
		h.manager.ledgerMaterializer = &testLedgerMaterializer{
			materialize: func(context.Context, store.SessionLedgerRecord) error {
				close(entered)
				<-release
				return cleanupErr
			},
		}
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		select {
		case <-entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		observed := &resumeWaitObservedContext{Context: ctx, entered: make(chan struct{})}
		waiter := make(chan error, 1)
		go func() { waiter <- h.manager.Stop(observed, active.ID) }()
		select {
		case <-observed.entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		unblock()
		if err := <-waiter; !errors.Is(err, cleanupErr) {
			t.Fatalf("concurrent recovered waiter lost cleanup error: %v", err)
		}
		outcome, err := h.manager.AwaitStopped(ctx, active.ID)
		if !errors.Is(err, cleanupErr) || !outcome.Verified || outcome.FinalState != StateStopped {
			t.Fatalf("recovered cleanup outcome = %#v, %v", outcome, err)
		}
		warning := storedEventByType(t, readStoredEvents(t, active), acp.EventTypeRuntimeWarning)
		if !strings.Contains(warning.Content, "ledger cleanup") {
			t.Fatalf("recovered cleanup warning = %s", warning.Content)
		}
		if err := h.manager.Stop(ctx, active.ID); err != nil {
			t.Fatalf("already completed recovered stop: %v", err)
		}
	})

	t.Run("Should coalesce recovered requests admitted before metadata preparation completes", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := seedRecoveredRemoteStop(t, h)
		ctx := testutil.Context(t)
		metadataEntered, phaseEntered := make(chan struct{}), make(chan struct{})
		releaseMetadata, releasePhase := make(chan struct{}), make(chan struct{})
		var metadataOnce, phaseOnce sync.Once
		t.Cleanup(func() {
			metadataOnce.Do(func() { close(releaseMetadata) })
			phaseOnce.Do(func() { close(releasePhase) })
		})
		var reads atomic.Int32
		h.manager.readSessionMeta = func(path string) (store.SessionMeta, error) {
			switch reads.Add(1) {
			case 1:
				close(metadataEntered)
				<-releaseMetadata
			case 2:
				close(phaseEntered)
				<-releasePhase
			}
			return store.ReadSessionMeta(path)
		}
		first, second := make(chan error, 1), make(chan error, 1)
		go func() { first <- h.manager.RequestStop(ctx, active.ID, CauseUserRequested) }()
		select {
		case <-metadataEntered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		observed := &resumeWaitObservedContext{Context: ctx, entered: make(chan struct{})}
		go func() { second <- h.manager.RequestStop(observed, active.ID, CauseUserRequested) }()
		select {
		case <-observed.entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		metadataOnce.Do(func() { close(releaseMetadata) })
		select {
		case <-phaseEntered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		for _, accepted := range []<-chan error{first, second} {
			select {
			case err := <-accepted:
				if err != nil {
					t.Fatalf("coalesced request waited for termination: %v", err)
				}
			case <-ctx.Done():
				t.Fatal("coalesced request did not return while the stop phase was blocked")
			}
		}
		resumeObserved := &resumeWaitObservedContext{Context: ctx, entered: make(chan struct{})}
		resumed := make(chan error, 1)
		go func() {
			_, err := h.manager.Resume(resumeObserved, active.ID)
			resumed <- err
		}()
		select {
		case <-resumeObserved.entered:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		select {
		case err := <-resumed:
			t.Fatalf("resume completed during recovered stop: %v", err)
		default:
		}
		phaseOnce.Do(func() { close(releasePhase) })
		if _, err := h.manager.AwaitStopped(ctx, active.ID); !errors.Is(err, ErrStopVerificationFailed) {
			t.Fatalf("recovered stop outcome: %v", err)
		}
		select {
		case err := <-resumed:
			if !errors.Is(err, ErrStopVerificationFailed) {
				t.Fatalf("resume after unverified recovered stop: %v", err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	})

	t.Run("Should stop a recovered local process and notify the verified terminal winner once", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := seedRecoveredLocalStop(t, h)
		ctx := testutil.Context(t)
		before := h.notifier.stoppedCount()
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		outcome, err := h.manager.AwaitStopped(ctx, active.ID)
		if err != nil || !outcome.Verified || outcome.FinalState != StateStopped || outcome.Phase != StopPhaseForced {
			t.Fatalf("recovered local stop = %#v, %v", outcome, err)
		}
		after := readMeta(t, active.MetaPath())
		if after.State != string(StateStopped) || after.StopVerificationFailed ||
			h.notifier.stoppedCount() != before+1 {
			t.Fatal("verified recovered stop did not persist and notify its terminal winner")
		}
		if countEventType(readStoredEvents(t, active), EventTypeSessionStopped) != 2 {
			t.Fatal("verified recovered stop did not append its terminal event")
		}
		if err := h.manager.Stop(ctx, active.ID); err != nil || h.notifier.stoppedCount() != before+1 {
			t.Fatalf("repeated recovered stop: %v", err)
		}
	})

	t.Run("Should retain recovered remote stop attention without signaling a local process", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := seedRecoveredRemoteStop(t, h)
		ctx := testutil.Context(t)
		meta := readMeta(t, active.MetaPath())
		beforeEvents := readStoredEvents(t, active)
		if err := h.manager.RequestStop(ctx, active.ID, CauseUserRequested); err != nil {
			t.Fatalf("RequestStop(recovered) error = %v", err)
		}
		outcome, err := h.manager.AwaitStopped(ctx, active.ID)
		if !errors.Is(err, ErrStopVerificationFailed) || outcome.Verified || outcome.FinalState != StateStopping {
			t.Fatalf("recovered stop = %#v, %v", outcome, err)
		}
		after := readMeta(t, active.MetaPath())
		if after.State != string(StateStopping) || !after.StopVerificationFailed || !after.StopEscalated ||
			after.Name != meta.Name || after.CreationDigest != meta.CreationDigest {
			t.Fatalf("recovered metadata lost stop truth or identity: %#v", after)
		}
		stored := readStoredEvents(t, active)
		if countEventType(
			stored,
			eventspkg.SessionStopEscalated,
		) != countEventType(
			beforeEvents,
			eventspkg.SessionStopEscalated,
		)+2 ||
			countEventType(stored, eventspkg.SessionStopVerificationFailed) != 1 ||
			countEventType(stored, EventTypeSessionStopped) != countEventType(beforeEvents, EventTypeSessionStopped) {
			t.Fatal("recovered stop omitted phase diagnostics or emitted an unverified terminal event")
		}
	})
	t.Run("Should recover remote termination through bounded provider phases and identity proof", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		var exited atomic.Bool
		signals := make(chan sandbox.ProcessSignal, 3)
		provider := &recoveredProcessTestProvider{
			verify: func(ctx context.Context, state sandbox.SessionState) (bool, error) {
				if _, ok := ctx.Deadline(); !ok || state.InstanceID != "remote-instance" {
					return false, errors.New("missing phase deadline or remote identity")
				}
				return exited.Load(), nil
			},
			signal: func(_ context.Context, _ sandbox.SessionState, signal sandbox.ProcessSignal) error {
				signals <- signal
				if signal == sandbox.ProcessSignalKill {
					exited.Store(true)
					return nil
				}
				return errors.New("remote process remains alive")
			},
		}
		registry, err := sandbox.NewRegistry(provider)
		if err != nil {
			t.Fatal(err)
		}
		h.manager.sandbox = registry
		proc, target := h.manager.recoveredTerminationTarget(&store.SessionMeta{
			Sandbox: &store.SessionSandboxMeta{Backend: "daytona", InstanceID: "remote-instance"},
		})
		result, err := h.manager.runTerminationLadder(t.Context(), proc, target)
		if err != nil || !result.Verified || result.Phase != StopPhaseKilled {
			t.Fatalf("recovered remote termination = %+v, %v", result, err)
		}
		close(signals)
		var observed []sandbox.ProcessSignal
		for signal := range signals {
			observed = append(observed, signal)
		}
		if !slices.Equal(observed, []sandbox.ProcessSignal{
			sandbox.ProcessSignalCloseInput, sandbox.ProcessSignalTerminate, sandbox.ProcessSignalKill,
		}) {
			t.Fatalf("remote phases = %v", observed)
		}
	})

	t.Run("Should use recovered process actions and identity proof through the shared ladder", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		proc := NewAgentProcess(AgentProcessOptions{PID: 123, Done: make(chan struct{})})
		var mu sync.Mutex
		var calls []StopPhase
		verified := false
		action := func(phase StopPhase, budget time.Duration) func(context.Context, *AgentProcess) error {
			return func(ctx context.Context, got *AgentProcess) error {
				deadline, ok := ctx.Deadline()
				if got != proc || !ok || time.Until(deadline) > budget {
					return errors.New("invalid recovered process phase context")
				}
				mu.Lock()
				defer mu.Unlock()
				calls = append(calls, phase)
				if phase == StopPhaseKilled {
					verified = true
					return nil
				}
				return errors.New("process remains alive")
			}
		}
		result, err := h.manager.runTerminationLadder(testutil.Context(t), proc, terminationTarget{
			cooperative: action(StopPhaseCooperative, h.manager.stopConfig.CooperativeGrace),
			forced:      action(StopPhaseForced, stopForcedGrace),
			kill:        action(StopPhaseKilled, stopKillGrace),
			verifyExit: func(got *AgentProcess) (bool, error) {
				mu.Lock()
				defer mu.Unlock()
				return got == proc && verified, nil
			},
		})
		if err != nil || !result.Verified || !result.Escalated || result.Phase != StopPhaseKilled {
			t.Fatalf("recovered termination = %#v, %v", result, err)
		}
		mu.Lock()
		defer mu.Unlock()
		if !reflect.DeepEqual(calls, []StopPhase{StopPhaseCooperative, StopPhaseForced, StopPhaseKilled}) {
			t.Fatalf("recovered termination actions = %#v", calls)
		}
	})
	t.Parallel()

	t.Run("Should bound stop while the launcher ignores cancellation and settle after it returns", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		entered, release := make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		t.Cleanup(unblock)
		h.driver.startContextHook = func(ctx context.Context, _ acp.StartOpts, _ int) (*fakeProcess, error) {
			close(entered)
			<-release
			return nil, ctx.Err()
		}
		created := make(chan error, 1)
		go func() {
			_, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "coder", Workspace: h.workspaceID})
			created <- err
		}()
		<-entered
		if err := h.manager.RequestStop(t.Context(), "sess-1", CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		budget := h.manager.stopConfig.CooperativeGrace + stopForcedGrace + stopKillGrace
		ctx, cancel := context.WithTimeout(t.Context(), budget+time.Second)
		defer cancel()
		pending, err := h.manager.AwaitStopped(ctx, "sess-1")
		if !errors.Is(err, ErrStopVerificationFailed) || pending.Verified || pending.FinalState != StateStopping {
			t.Fatalf("hung launcher stop = %#v, %v", pending, err)
		}
		active, ok := h.manager.Get("sess-1")
		if !ok || !readMeta(t, active.MetaPath()).StopVerificationFailed || h.notifier.stoppedCount() != 0 {
			t.Fatal("hung launcher did not retain durable stop attention")
		}
		unblock()
		if err := <-created; err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		settled, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if err != nil || !settled.Verified || settled.FinalState != StateStopped {
			t.Fatalf("late startup settlement = %#v, %v", settled, err)
		}
	})
	t.Run("Should retain an unverified process acquired after startup cancellation", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		entered, release := make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		t.Cleanup(unblock)
		late := newFakeProcess("coder", "fake-agent", h.workspace, "acp-late")
		h.driver.startContextHook = func(ctx context.Context, _ acp.StartOpts, _ int) (*fakeProcess, error) {
			close(entered)
			<-ctx.Done()
			<-release
			return late, nil
		}
		h.driver.stopHook = func(*fakeProcess) error { return errors.New("forced stop refused") }
		h.driver.killHook = func(*fakeProcess) error { return errors.New("kill refused") }
		created := make(chan error, 1)
		go func() {
			_, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "coder", Workspace: h.workspaceID})
			created <- err
		}()
		<-entered
		if err := h.manager.RequestStop(t.Context(), "sess-1", CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		unblock()
		outcome, err := h.manager.AwaitStopped(t.Context(), "sess-1")
		if !errors.Is(err, ErrStopVerificationFailed) || outcome.Verified || outcome.FinalState != StateStopping {
			t.Fatalf("late process outcome = %#v, %v", outcome, err)
		}
		if err := <-created; !errors.Is(err, ErrStopVerificationFailed) {
			t.Fatalf("startup failure = %v", err)
		}
		active, ok := h.manager.Get("sess-1")
		if !ok || active.processHandle() != late.handle ||
			readMeta(t, active.MetaPath()).State != string(StateStopping) {
			t.Fatal("late live process was not retained for a retry")
		}
		h.driver.mu.Lock()
		h.driver.killHook = nil
		h.driver.mu.Unlock()
		if err := h.manager.Stop(t.Context(), active.ID); err != nil {
			t.Fatalf("retry late process stop: %v", err)
		}
	})
	t.Run("Should share an asynchronous stop while the provider is starting", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		t.Cleanup(unblock)
		h.driver.startContextHook = func(ctx context.Context, _ acp.StartOpts, _ int) (*fakeProcess, error) {
			close(entered)
			<-ctx.Done()
			close(canceled)
			<-release
			return nil, ctx.Err()
		}
		created := make(chan error, 1)
		go func() {
			_, err := h.manager.Create(t.Context(), CreateOpts{AgentName: "coder", Workspace: h.workspaceID})
			created <- err
		}()
		<-entered
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := h.manager.RequestStop(ctx, "sess-1", CauseUserRequested); err != nil {
			t.Fatalf("accept startup stop: %v", err)
		}
		<-canceled
		cancel()
		if _, err := h.manager.AwaitStopped(ctx, "sess-1"); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled startup waiter: %v", err)
		}
		unblock()
		outcome, err := h.manager.AwaitStopped(t.Context(), "sess-1")
		if err != nil || !outcome.Verified || outcome.FinalState != StateStopped ||
			outcome.Cause != CauseUserRequested {
			t.Fatalf("startup stop outcome = %#v, %v", outcome, err)
		}
		if err := <-created; err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("startup completion: %v", err)
		}
		replay, err := h.manager.AwaitStopped(t.Context(), "sess-1")
		if err != nil || replay != outcome {
			t.Fatalf("shared startup result = %#v, %v", replay, err)
		}
	})
	t.Run("Should distinguish turn quiescence from session process termination", func(t *testing.T) {
		t.Parallel()
		for _, turnOnly := range []bool{true, false} {
			t.Run(fmt.Sprintf("Should verify the requested scope with turnOnly %t", turnOnly), func(t *testing.T) {
				t.Parallel()
				h := newHarness(t)
				active := createSession(t, h)
				proc := active.processHandle()
				promptDone := make(chan struct{})
				h.driver.cancelHook = func(*fakeProcess) error { close(promptDone); return nil }
				result, err := h.manager.runTerminationLadder(t.Context(), proc, terminationTarget{
					promptDone: promptDone, turnOnly: turnOnly,
				})
				if err != nil || !result.Verified || result.processExited == turnOnly {
					t.Fatalf("termination = %#v, %v", result, err)
				}
				if result.Escalated == turnOnly || isProcessDone(proc) == turnOnly {
					t.Fatalf("scope termination = %#v, process done = %t", result, isProcessDone(proc))
				}
				h.driver.mu.Lock()
				h.driver.cancelHook = nil
				h.driver.mu.Unlock()
				if err := h.manager.Stop(t.Context(), active.ID); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
	t.Run("Should share one cooperative operation across concurrent requests", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		active := createSession(t, h)
		entered, release := make(chan struct{}), make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		t.Cleanup(unblock)
		h.driver.cancelHook = func(proc *fakeProcess) error { close(entered); <-release; proc.exit(); return nil }
		if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
			t.Fatal(err)
		}
		<-entered
		var requests sync.WaitGroup
		for range 16 {
			requests.Go(func() {
				if err := h.manager.RequestStop(t.Context(), active.ID, CauseUserRequested); err != nil {
					t.Errorf("duplicate stop: %v", err)
				}
			})
		}
		requests.Wait()
		if active.Info().State != StateStopping {
			t.Fatalf("state = %s", active.Info().State)
		}
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := h.manager.AwaitStopped(canceled, active.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled waiter: %v", err)
		}
		unblock()
		outcome, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if err != nil || !outcome.Verified || outcome.Escalated || outcome.Phase != StopPhaseCooperative {
			t.Fatalf("cooperative outcome = %#v, %v", outcome, err)
		}
		if outcome.FinalState != StateStopped || readMeta(t, active.MetaPath()).State != string(StateStopped) {
			t.Fatalf("cooperative terminal state was not persisted: %#v", outcome)
		}
		replay, err := h.manager.AwaitStopped(t.Context(), active.ID)
		if err != nil || replay != outcome {
			t.Fatalf("shared result = %#v, %v", replay, err)
		}
		h.driver.mu.Lock()
		cancels, stops, kills := h.driver.cancelCalls, h.driver.stopCalls, h.driver.killCalls
		h.driver.mu.Unlock()
		if cancels != 1 || stops != 0 || kills != 0 {
			t.Fatalf("calls = cancel:%d stop:%d kill:%d", cancels, stops, kills)
		}
	})
	for _, phase := range []StopPhase{StopPhaseForced, StopPhaseKilled} {
		name := "Should advance after cooperative deadline to " + string(phase) + " when transport ignores cancellation"
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(
				t,
				WithSessionStopConfig(compozyconfig.SessionStopConfig{CooperativeGrace: 20 * time.Millisecond}),
			)
			active := createSession(t, h)
			release := make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			t.Cleanup(unblock)
			h.driver.cancelHook = func(*fakeProcess) error { <-release; return nil }
			h.driver.stopHook = func(proc *fakeProcess) error {
				if phase == StopPhaseKilled {
					return errors.New("forced transport refused")
				}
				proc.exit()
				return nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := h.manager.Stop(ctx, active.ID); err != nil {
				t.Fatalf("bounded stop: %v", err)
			}
			outcome, err := h.manager.AwaitStopped(ctx, active.ID)
			if err != nil || !outcome.Verified || outcome.Phase != phase {
				t.Fatalf("outcome = %#v, %v", outcome, err)
			}
			if outcome.FinalState != StateStopped || readMeta(t, active.MetaPath()).State != string(StateStopped) {
				t.Fatalf("escalated terminal state was not persisted: %#v", outcome)
			}
			if outcome.Elapsed < h.manager.stopConfig.CooperativeGrace || !outcome.Escalated ||
				!readMeta(t, active.MetaPath()).StopEscalated {
				t.Fatalf("stop timing or persisted escalation does not match the ladder: %#v", outcome)
			}
			h.driver.mu.Lock()
			stops, kills := h.driver.stopCalls, h.driver.killCalls
			h.driver.mu.Unlock()
			wantKills := 0
			if phase == StopPhaseKilled {
				wantKills = 1
			}
			if stops != 1 || kills != wantKills {
				t.Fatalf("phase %s signals = stop:%d kill:%d", phase, stops, kills)
			}
			pendingCtx, cancelPending := context.WithCancel(t.Context())
			cancelPending()
			if err := h.manager.WaitForPromptDrains(pendingCtx); !errors.Is(err, context.Canceled) {
				t.Fatalf("pending transport must remain owned: %v", err)
			}
			unblock()
			if err := h.manager.WaitForPromptDrains(ctx); err != nil {
				t.Fatalf("transport join: %v", err)
			}
		})
	}
}

type terminalCloseFailingRecorder struct {
	EventRecorder
	fail     *atomic.Bool
	closeErr error
	terminal bool
}

type terminalWriteFailingRecorder struct {
	EventRecorder
	fail        *atomic.Bool
	writeErr    error
	afterCommit bool
}

func (r *terminalWriteFailingRecorder) AppendEventIfAbsent(
	ctx context.Context,
	event store.SessionEvent,
) (store.SessionEvent, error) {
	if event.Type == EventTypeSessionStopped && r.fail.Load() && !r.afterCommit {
		return store.SessionEvent{}, r.writeErr
	}
	persisted, err := recordIdempotentSessionEvent(ctx, r.EventRecorder, event)
	if event.Type == EventTypeSessionStopped && r.fail.Load() {
		return persisted, errors.Join(err, r.writeErr)
	}
	return persisted, err
}

func (r *terminalCloseFailingRecorder) AppendEventIfAbsent(
	ctx context.Context, event store.SessionEvent,
) (store.SessionEvent, error) {
	persisted, err := appendDurableSessionEventWithRecorder(ctx, r.EventRecorder, event)
	r.terminal = err == nil && event.Type == EventTypeSessionStopped
	return persisted, err
}

func (r *terminalCloseFailingRecorder) Close(ctx context.Context) error {
	err := r.EventRecorder.Close(ctx)
	if r.terminal && r.fail.CompareAndSwap(true, false) {
		return errors.Join(err, r.closeErr)
	}
	return err
}

func seedRecoveredRemoteStop(t *testing.T, h *harness) *Session {
	t.Helper()
	active := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), active.ID); err != nil {
		t.Fatal(err)
	}
	started, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	meta := readMeta(t, active.MetaPath())
	meta.State = string(StateStopping)
	meta.Liveness = &store.SessionLivenessMeta{SubprocessPID: os.Getpid(), SubprocessStartedAt: &started}
	meta.Sandbox = &store.SessionSandboxMeta{Backend: "daytona"}
	if err := store.WriteSessionMeta(active.MetaPath(), meta); err != nil {
		t.Fatal(err)
	}
	h.manager.mu.Lock()
	delete(h.manager.stopRuns, active.ID)
	h.manager.mu.Unlock()
	return active
}

func seedRecoveredLocalStop(t *testing.T, h *harness) *Session {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group lifecycle; Windows termination has its own procutil suite")
	}
	active := createSession(t, h)
	ctx := testutil.Context(t)
	if err := h.manager.Stop(ctx, active.ID); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "sleep", "30")
	procutil.ConfigureCommandProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	t.Cleanup(func() {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("cleanup recovered process: %v", err)
		}
		if err := <-waited; err != nil {
			if _, ok := errors.AsType[*exec.ExitError](err); !ok {
				t.Errorf("wait recovered process: %v", err)
			}
		}
	})
	started, err := procutil.StartedAt(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	meta := readMeta(t, active.MetaPath())
	meta.State, meta.Sandbox = string(StateStopping), nil
	meta.Liveness = &store.SessionLivenessMeta{SubprocessPID: cmd.Process.Pid, SubprocessStartedAt: &started}
	if err := store.WriteSessionMeta(active.MetaPath(), meta); err != nil {
		t.Fatal(err)
	}
	h.manager.mu.Lock()
	delete(h.manager.stopRuns, active.ID)
	h.manager.mu.Unlock()
	return active
}

type recoveredProcessTestProvider struct {
	sandbox.Provider
	verify func(context.Context, sandbox.SessionState) (bool, error)
	signal func(context.Context, sandbox.SessionState, sandbox.ProcessSignal) error
}

func (*recoveredProcessTestProvider) Backend() sandbox.Backend { return sandbox.BackendDaytona }
func (p *recoveredProcessTestProvider) ProcessExitVerified(
	ctx context.Context, state sandbox.SessionState,
) (bool, error) {
	return p.verify(ctx, state)
}
func (p *recoveredProcessTestProvider) SignalProcess(
	ctx context.Context, state sandbox.SessionState, signal sandbox.ProcessSignal,
) error {
	return p.signal(ctx, state, signal)
}
