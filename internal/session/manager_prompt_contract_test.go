package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	skillbundled "github.com/compozy/compozy/skills"
)

func TestPumpPromptReturnsWhenContextIsCanceledWhileWaitingForSource(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	source := make(chan acp.AgentEvent)
	out := make(chan acp.AgentEvent)
	ctx, cancel := context.WithCancel(testutil.Context(t))

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.manager.pumpPrompt(
			ctx,
			ctx,
			nil,
			newPromptTurnDispatchState(nil, "turn-1", TurnSourceUser, ""),
			source,
			nil,
			out,
			nil,
			nil,
		)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pumpPrompt() did not return after context cancellation")
	}

	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("pumpPrompt() output channel remained open after cancellation")
		}
	default:
		t.Fatal("pumpPrompt() did not close output channel")
	}
}

func TestPumpPromptDrainsRuntimeEventsAfterTurnDone(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	source := make(chan acp.AgentEvent)
	runtimeEvents := make(chan acp.AgentEvent, 1)
	out := make(chan acp.AgentEvent)
	done := make(chan struct{})
	ctx := testutil.Context(t)

	go func() {
		defer close(done)
		h.manager.pumpPrompt(
			ctx,
			ctx,
			session,
			newPromptTurnDispatchState(session, "turn-runtime-drain", TurnSourceUser, ""),
			source,
			runtimeEvents,
			out,
			nil,
			nil,
		)
	}()

	source <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "turn-runtime-drain"}
	first := receivePromptEvent(t, out)
	if first.Type != acp.EventTypeDone {
		t.Fatalf("first event type = %q, want %q", first.Type, acp.EventTypeDone)
	}

	runtimeEvents <- acp.AgentEvent{Type: acp.EventTypeRuntimeProgress, TurnID: "turn-runtime-drain"}
	close(runtimeEvents)
	second := receivePromptEvent(t, out)
	if second.Type != acp.EventTypeRuntimeProgress {
		t.Fatalf("second event type = %q, want %q", second.Type, acp.EventTypeRuntimeProgress)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pumpPrompt() did not return after draining runtime events")
	}
}

func TestNextPromptPumpEventPrioritizesReadyRuntimeEvents(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer a ready runtime event over a ready source event", func(t *testing.T) {
		t.Parallel()

		source := make(chan acp.AgentEvent, 1)
		runtimeEvents := make(chan acp.AgentEvent, 1)
		loop := &promptPumpLoopState{source: source, runtime: runtimeEvents}

		source <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: "turn-runtime-priority"}
		runtimeEvents <- acp.AgentEvent{
			Type:   acp.EventTypeRuntimeWarning,
			TurnID: "turn-runtime-priority",
		}

		event, runtimeEvent, ok := nextPromptPumpEvent(testutil.Context(t), loop)
		if !ok {
			t.Fatal("nextPromptPumpEvent() ok = false, want true")
		}
		if !runtimeEvent {
			t.Fatal("nextPromptPumpEvent() runtimeEvent = false, want true")
		}
		if got, want := event.Type, acp.EventTypeRuntimeWarning; got != want {
			t.Fatalf("nextPromptPumpEvent() type = %q, want %q", got, want)
		}

		select {
		case remaining := <-source:
			if got, want := remaining.Type, acp.EventTypeError; got != want {
				t.Fatalf("remaining source event type = %q, want %q", got, want)
			}
		default:
			t.Fatal("source event drained unexpectedly")
		}
	})

	t.Run("Should probe source closure after a prioritized runtime event", func(t *testing.T) {
		t.Parallel()

		source := make(chan acp.AgentEvent)
		runtimeEvents := make(chan acp.AgentEvent, 2)
		loop := &promptPumpLoopState{source: source, runtime: runtimeEvents}

		runtimeEvents <- acp.AgentEvent{
			Type:   acp.EventTypeRuntimeProgress,
			TurnID: "turn-runtime-priority",
		}
		runtimeEvents <- acp.AgentEvent{
			Type:   acp.EventTypeRuntimeProgress,
			TurnID: "turn-runtime-priority",
		}

		event, runtimeEvent, ok := nextPromptPumpEvent(testutil.Context(t), loop)
		if !ok {
			t.Fatal("nextPromptPumpEvent() ok = false, want true")
		}
		if !runtimeEvent {
			t.Fatal("nextPromptPumpEvent() runtimeEvent = false, want true")
		}
		if got, want := event.Type, acp.EventTypeRuntimeProgress; got != want {
			t.Fatalf("nextPromptPumpEvent() type = %q, want %q", got, want)
		}

		close(source)
		event, runtimeEvent, ok = nextPromptPumpEvent(testutil.Context(t), loop)
		if !ok {
			t.Fatal("nextPromptPumpEvent() ok = false, want buffered runtime event after source closure")
		}
		if !runtimeEvent {
			t.Fatal("nextPromptPumpEvent() runtimeEvent = false, want true")
		}
		if got, want := event.Type, acp.EventTypeRuntimeProgress; got != want {
			t.Fatalf("nextPromptPumpEvent() type = %q, want %q", got, want)
		}
		if loop.source != nil {
			t.Fatal("loop source = non-nil, want closed source observed before draining more runtime events")
		}
	})
}

func TestPromptStreamsToRecorderAndNotifier(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if len(events) != 2 {
		t.Fatalf("Prompt() events = %d, want 2", len(events))
	}
	if events[0].Type != acp.EventTypeAgentMessage {
		t.Fatalf("first event type = %q, want %q", events[0].Type, acp.EventTypeAgentMessage)
	}
	if events[1].Type != acp.EventTypeDone {
		t.Fatalf("second event type = %q, want %q", events[1].Type, acp.EventTypeDone)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) != 3 {
		t.Fatalf("stored events = %d, want 3", len(stored))
	}
	if got := stored[0].Type; got != acp.EventTypeUserMessage {
		t.Fatalf("first stored event type = %q, want %q", got, acp.EventTypeUserMessage)
	}
	if got := h.notifier.eventCount(session.ID); got != 3 {
		t.Fatalf("notifier events = %d, want 3", got)
	}
}

func TestPromptPersistenceFailureStopsSessionBeforeLiveDelivery(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a failed single event out of live reload and turn completion", func(t *testing.T) {
		t.Parallel()

		recordErr := errors.New("projection failed token=super-secret")
		recorder := &failingSinglePromptRecorder{failErr: recordErr}
		h := newHarness(t, WithStore(func(context.Context, store.SessionDBOwner, string) (EventRecorder, error) {
			return recorder, nil
		}))
		turnEnds := 0
		h.manager.SetTurnEndNotifier(func(string) { turnEnds++ })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			go func() {
				defer close(events)
				events <- acp.AgentEvent{
					Type:       acp.EventTypeToolCall,
					SessionID:  proc.handle.SessionID,
					TurnID:     req.TurnID,
					ToolCallID: "call-failed",
					Title:      "Read",
				}
			}()
			return events, nil
		}
		session := createSession(t, h)
		t.Cleanup(func() {
			if _, active := h.manager.Get(session.ID); active {
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
					!errors.Is(err, ErrSessionNotFound) {
					t.Errorf("Stop() cleanup error = %v", err)
				}
			}
		})

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		if delivered := collectEvents(t, eventsCh); len(delivered) != 0 {
			t.Fatalf("Prompt() delivered = %#v, want no unpersisted event", delivered)
		}
		if got := countAgentEvents(h.notifier.eventsForSession(session.ID), acp.EventTypeToolCall); got != 0 {
			t.Fatalf("tool call notifier events = %d, want zero", got)
		}
		if turnEnds != 0 {
			t.Fatalf("turn end notifications = %d, want zero", turnEnds)
		}
		h.notifier.waitForStopped(t, session.ID)
		if _, active := h.manager.Get(session.ID); active {
			t.Fatalf("Get(%q) found session after stopped notification", session.ID)
		}
		meta := readMeta(t, session.MetaPath())
		if meta.Failure == nil || meta.Failure.Kind != store.FailureTransport {
			t.Fatalf("stopped session failure = %#v, want transport failure", meta.Failure)
		}
		if strings.Contains(meta.Failure.Summary, "super-secret") {
			t.Fatalf("stopped session failure leaked secret: %q", meta.Failure.Summary)
		}
		reloaded, queryErr := recorder.Query(testutil.Context(t), store.EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query(reload) error = %v", queryErr)
		}
		if got := countEventType(reloaded, acp.EventTypeToolCall); got != 0 {
			t.Fatalf("reloaded tool_call rows = %d, want zero", got)
		}
	})
}

func TestPromptDeadlineDeliversRuntimeWarningBeforeError(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithSessionSupervision(compozyconfig.SessionSupervisionConfig{
		ActivityHeartbeatInterval: time.Hour,
		ProgressNotifyInterval:    0,
		InactivityWarningAfter:    0,
		InactivityTimeout:         0,
		TimeoutCancelGrace:        2 * time.Second,
		PromptDeadline:            20 * time.Millisecond,
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil && !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
		}
	})

	source := make(chan acp.AgentEvent, 1)
	var promptTurnID string
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		promptTurnID = req.TurnID
		return source, nil
	}
	h.driver.cancelHook = func(proc *fakeProcess) error {
		source <- acp.AgentEvent{
			Type:      acp.EventTypeError,
			SessionID: proc.handle.SessionID,
			TurnID:    promptTurnID,
			Timestamp: time.Now().UTC(),
			Error:     `{"code":-32603,"message":"Internal error","data":{"error":"context deadline exceeded"}}`,
		}
		close(source)
		return nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "long running")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if got, want := len(events), 2; got != want {
		t.Fatalf("Prompt() events = %d, want %d", got, want)
	}
	if got, want := events[0].Type, acp.EventTypeRuntimeWarning; got != want {
		t.Fatalf("Prompt() first event type = %q, want %q", got, want)
	}
	if got, want := events[1].Type, acp.EventTypeError; got != want {
		t.Fatalf("Prompt() second event type = %q, want %q", got, want)
	}
	if events[0].Runtime == nil || events[0].Runtime.DeadlineAt == nil {
		t.Fatalf("Prompt() first runtime = %#v, want deadline payload", events[0].Runtime)
	}
	if got := h.driver.cancelCalls; got != 1 {
		t.Fatalf("driver cancel calls = %d, want 1", got)
	}
	if got := h.driver.stopCalls; got != 1 {
		t.Fatalf("driver stop calls = %d, want 1", got)
	}
}

func TestPromptFatalProcessFailureStopsSessionAndPreservesReadOnlyHistory(t *testing.T) {
	t.Run("Should stop after a fatal process failure without resuming the original session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		originalACP := session.Info().ACPSessionID
		processExitErr := errors.New("acp subprocess exited: exit status 23")
		h.driver.lastProcess().handle.exitStatusFn = func() (subprocess.ExitStatus, bool) {
			return subprocess.ExitStatus{ExitCode: 23}, true
		}
		t.Cleanup(func() {
			if _, ok := h.manager.Get(session.ID); ok {
				reportSessionStop(t, h, session.ID)
			}
		})
		h.driver.stopHook = func(proc *fakeProcess) error {
			proc.crash(processExitErr, "codex stderr tail")
			return nil
		}

		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			go func() {
				defer close(events)
				events <- acp.AgentEvent{
					Type:      acp.EventTypeAgentMessage,
					SessionID: originalACP,
					TurnID:    req.TurnID,
					Timestamp: time.Now().UTC(),
					Text:      "partial before disconnect",
				}
				events <- acp.AgentEvent{
					Type:      acp.EventTypeError,
					SessionID: originalACP,
					TurnID:    req.TurnID,
					Timestamp: time.Now().UTC(),
					Error:     `RequestError -32603: peer disconnected before response`,
					Failure: &store.SessionFailure{
						Kind:    store.FailureProcess,
						Summary: `RequestError -32603: peer disconnected before response`,
					},
				}
			}()
			return events, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		events := collectEvents(t, eventsCh)
		if got, want := len(events), 2; got != want {
			t.Fatalf("Prompt() events = %d, want %d", got, want)
		}
		if got, want := events[0].Type, acp.EventTypeAgentMessage; got != want {
			t.Fatalf("Prompt() first event type = %q, want %q", got, want)
		}
		if got, want := events[0].Text, "partial before disconnect"; got != want {
			t.Fatalf("Prompt() first event text = %q, want %q", got, want)
		}
		if events[1].Failure == nil || events[1].Failure.Kind != store.FailureProcess {
			t.Fatalf("Prompt() failure = %#v, want process_exit", events[1].Failure)
		}
		if events[1].Failure.CrashBundlePath == "" {
			t.Fatalf("Prompt() failure = %#v, want crash bundle path", events[1].Failure)
		}
		closedMeta := readMeta(t, session.MetaPath())
		if closedMeta.State != string(StateStopped) || closedMeta.Failure == nil ||
			closedMeta.Failure.Kind != store.FailureProcess {
			t.Fatalf("meta after prompt stream close = %#v, want stopped process failure", closedMeta)
		}
		publicBundle := readCrashBundleDocument(t, events[1].Failure.CrashBundlePath)
		if publicBundle.Process == nil || publicBundle.Process.ExitCode == nil ||
			*publicBundle.Process.ExitCode != 23 {
			t.Fatalf(
				"public crash bundle process = %#v, want exit code 23 before stop",
				publicBundle.Process,
			)
		}
		if publicBundle.Process.Signal != "" {
			t.Fatalf("public crash bundle signal = %q, want empty for numeric exit", publicBundle.Process.Signal)
		}
		if got, want := publicBundle.Stderr, "codex stderr tail"; got != want {
			t.Fatalf("public crash bundle stderr = %q, want %q", got, want)
		}

		h.notifier.waitForStopped(t, session.ID)
		if _, ok := h.manager.Get(session.ID); ok {
			t.Fatalf("Get(%q) found session after stopped notification", session.ID)
		}

		if got := h.driver.stopCalls; got != 1 {
			t.Fatalf("driver stop calls = %d, want 1", got)
		}

		meta := readMeta(t, session.MetaPath())
		if got := meta.State; got != string(StateStopped) {
			t.Fatalf("meta state = %q, want %q", got, StateStopped)
		}
		if meta.StopReason == nil || *meta.StopReason != store.StopAgentCrashed {
			t.Fatalf("meta.StopReason = %#v, want %q", meta.StopReason, store.StopAgentCrashed)
		}
		if meta.Failure == nil || meta.Failure.Kind != store.FailureProcess {
			t.Fatalf("meta.Failure = %#v, want process_exit", meta.Failure)
		}
		if got, want := meta.Failure.CrashBundlePath, events[1].Failure.CrashBundlePath; got != want {
			t.Fatalf("meta crash bundle = %q, want event bundle %q", got, want)
		}
		if !strings.Contains(meta.StopDetail, processExitErr.Error()) {
			t.Fatalf("meta.StopDetail = %q, want real process exit diagnostic %q", meta.StopDetail, processExitErr)
		}
		stored := readStoredEvents(t, session)
		if !containsEventType(stored, acp.EventTypeAgentMessage) || containsEventType(stored, acp.EventTypeDone) {
			t.Fatalf("stored events = %#v, want partial agent message and no done event", stored)
		}
		partial := storedEventByType(t, stored, acp.EventTypeAgentMessage)
		if !strings.Contains(partial.Content, "partial before disconnect") {
			t.Fatalf("stored agent message = %s, want persisted partial chunk", partial.Content)
		}

		startsBeforeRejectedPrompt := len(h.driver.startCalls)
		_, err = h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message:        "retry the dead session",
			MessageID:      "message-dead-session-retry",
			IdempotencyKey: "idempotency-dead-session-retry",
		})
		if !errors.Is(err, store.ErrSessionNotAttachable) {
			t.Fatalf("SendPrompt(dead session) error = %v, want ErrSessionNotAttachable", err)
		}
		if got := len(h.driver.startCalls); got != startsBeforeRejectedPrompt {
			t.Fatalf("dead-session Prompt() start calls = %d, want %d", got, startsBeforeRejectedPrompt)
		}
	})
}

func TestPromptStreamClosureWithoutTerminalStopsSession(t *testing.T) {
	t.Run("Should preserve partial output and stop on terminal-less stream closure", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		proc := h.driver.lastProcess()
		proc.handle.exitStatusFn = func() (subprocess.ExitStatus, bool) {
			select {
			case <-proc.done:
				return subprocess.ExitStatus{ExitCode: 23}, true
			default:
				return subprocess.ExitStatus{}, false
			}
		}
		var stoppedWhileActive atomic.Bool
		h.driver.stopHook = func(proc *fakeProcess) error {
			select {
			case <-proc.done:
			default:
				stoppedWhileActive.Store(true)
			}
			proc.crash(errors.New("ACP transport closed before terminal event"), "codex stderr after EOF")
			return nil
		}
		t.Cleanup(func() {
			if _, ok := h.manager.Get(sess.ID); ok {
				reportSessionStop(t, h, sess.ID)
			}
		})
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: proc.handle.SessionID,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Text:      "partial before EOF",
			}
			close(events)
			return events, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), sess.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		events := collectEvents(t, eventsCh)
		if got, want := len(events), 2; got != want {
			t.Fatalf("Prompt() events = %#v, want partial chunk and terminal failure", events)
		}
		if got, want := events[0].Text, "partial before EOF"; got != want {
			t.Fatalf("Prompt() first event text = %q, want %q", got, want)
		}
		terminal := events[1]
		if terminal.Type != acp.EventTypeError || terminal.Failure == nil ||
			terminal.Failure.Kind != store.FailureTransport {
			t.Fatalf("Prompt() terminal event = %#v, want transport failure", terminal)
		}
		if !strings.Contains(terminal.Error, "ended before a terminal event") {
			t.Fatalf("Prompt() terminal error = %q, want explicit incomplete-stream failure", terminal.Error)
		}
		if terminal.Failure.CrashBundlePath == "" {
			t.Fatalf("Prompt() terminal failure = %#v, want public crash bundle path", terminal.Failure)
		}
		publicBundle := readCrashBundleDocument(t, terminal.Failure.CrashBundlePath)
		if publicBundle.Process == nil || publicBundle.Process.ExitCode == nil ||
			*publicBundle.Process.ExitCode != 23 {
			t.Fatalf("public crash bundle process = %#v, want exit code 23 before stop", publicBundle.Process)
		}
		if got, want := publicBundle.Stderr, "codex stderr after EOF"; got != want {
			t.Fatalf("public crash bundle stderr = %q, want %q", got, want)
		}

		h.notifier.waitForStopped(t, sess.ID)
		if _, ok := h.manager.Get(sess.ID); ok {
			t.Fatalf("Get(%q) found session after terminal-less stream closure", sess.ID)
		}
		if !stoppedWhileActive.Load() {
			t.Fatal("process was not active when terminal-less stream closure initiated stop")
		}
		stored := readStoredEvents(t, sess)
		if !containsEventType(stored, acp.EventTypeAgentMessage) ||
			!containsEventType(stored, acp.EventTypeError) ||
			containsEventType(stored, acp.EventTypeDone) {
			t.Fatalf("stored events = %#v, want partial chunk, terminal error, and no done", stored)
		}
		persistedError := storedEventByType(t, stored, acp.EventTypeError)
		persistedAgentEvent, err := transcript.UnmarshalAgentEvent(persistedError.Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent(error event) error = %v", err)
		}
		meta := readMeta(t, sess.MetaPath())
		if persistedAgentEvent.Failure == nil || meta.Failure == nil {
			t.Fatalf(
				"persisted/final failures = %#v / %#v, want crash bundle evidence",
				persistedAgentEvent.Failure,
				meta.Failure,
			)
		}
		if got, want := persistedAgentEvent.Failure.CrashBundlePath, terminal.Failure.CrashBundlePath; got != want {
			t.Fatalf("persisted crash bundle = %q, want public event bundle %q", got, want)
		}
		if got, want := meta.Failure.CrashBundlePath, persistedAgentEvent.Failure.CrashBundlePath; got != want {
			t.Fatalf("final crash bundle = %q, want event-linked bundle %q", got, want)
		}
		bundle := readCrashBundleDocument(t, meta.Failure.CrashBundlePath)
		if bundle.Process == nil || bundle.Process.ExitCode == nil || *bundle.Process.ExitCode != 23 {
			t.Fatalf("crash bundle process = %#v, want exit code 23 after process stop", bundle.Process)
		}
		if got, want := bundle.Stderr, "codex stderr after EOF"; got != want {
			t.Fatalf("crash bundle stderr = %q, want %q", got, want)
		}
	})
}

func TestPromptGenericFailureKeepsSessionActive(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if _, ok := h.manager.Get(session.ID); ok {
			reportSessionStop(t, h, session.ID)
		}
	})

	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent, 1)
		go func() {
			defer close(events)
			events <- acp.AgentEvent{
				Type:      acp.EventTypeError,
				SessionID: session.Info().ACPSessionID,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Error:     `{"code":-32603,"message":"Internal error","data":{"details":"Tool invocation failed"}}`,
				Failure: &store.SessionFailure{
					Kind:    store.FailurePrompt,
					Summary: `{"code":-32603,"message":"Internal error","data":{"details":"Tool invocation failed"}}`,
				},
			}
		}()
		return events, nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	events := collectEvents(t, eventsCh)
	if got, want := len(events), 1; got != want {
		t.Fatalf("Prompt() events = %d, want %d", got, want)
	}

	if _, ok := h.manager.Get(session.ID); !ok {
		t.Fatal("session removed after generic prompt failure, want still active")
	}
	if got := h.driver.stopCalls; got != 0 {
		t.Fatalf("driver stop calls = %d, want 0", got)
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume(active) error = %v", err)
	}
	if resumed != session {
		t.Fatalf("Resume(active) returned %p, want %p", resumed, session)
	}
}

func TestCancelPrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the session active when the canceled provider stream closes without a terminal", func(
		t *testing.T,
	) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if _, ok := h.manager.Get(session.ID); ok {
				reportSessionStop(t, h, session.ID)
			}
		})

		firstPromptEvents := make(chan acp.AgentEvent)
		promptStarted := make(chan struct{})
		var promptCalls atomic.Int32
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if promptCalls.Add(1) == 1 {
				close(promptStarted)
				return firstPromptEvents, nil
			}
			events := make(chan acp.AgentEvent, 1)
			events <- acp.AgentEvent{
				Type:      acp.EventTypeDone,
				SessionID: proc.handle.SessionID,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
			}
			close(events)
			return events, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		select {
		case <-promptStarted:
		case <-time.After(time.Second):
			t.Fatal("driver prompt did not start")
		}
		if !session.IsPrompting() {
			t.Fatal("session IsPrompting() = false after driver prompt started")
		}

		cancelResult, err := h.manager.CancelPrompt(
			WithActingSession(testutil.Context(t), "sess-creator"),
			session.ID,
		)
		if err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if cancelResult.Outcome != PromptCancelOutcomeCanceled || strings.TrimSpace(cancelResult.TurnID) == "" {
			t.Fatalf("CancelPrompt() = %#v, want canceled turn", cancelResult)
		}
		repeated, err := h.manager.CancelPrompt(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("CancelPrompt(repeated) error = %v", err)
		}
		if repeated != cancelResult {
			t.Fatalf("CancelPrompt(repeated) = %#v, want idempotent %#v", repeated, cancelResult)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
		if got := len(h.driver.interruptScopes); got != 1 {
			t.Fatalf("driver interrupt calls = %d, want 1", got)
		}
		if got := h.driver.interruptScopes[0]; got.SessionID != session.ID || got.TurnID == "" {
			t.Fatalf("driver interrupt scope = %#v, want session and turn", got)
		}

		close(firstPromptEvents)
		var canceledEvents []acp.AgentEvent
		waitForClose := time.NewTimer(2 * time.Second)
		defer waitForClose.Stop()
	drainCanceledPrompt:
		for {
			select {
			case event, ok := <-eventsCh:
				if !ok {
					break drainCanceledPrompt
				}
				canceledEvents = append(canceledEvents, event)
			case <-waitForClose.C:
				t.Fatal("timed out waiting for canceled prompt output to close")
			}
		}
		for _, event := range canceledEvents {
			if event.Failure != nil && (event.Failure.Kind == store.FailureTransport ||
				event.Failure.Kind == store.FailureProcess) {
				t.Fatalf(
					"canceled prompt event failure kind = %q, want no fatal runtime failure",
					event.Failure.Kind,
				)
			}
		}
		if len(canceledEvents) != 1 || canceledEvents[0].Type != acp.EventTypeDone ||
			canceledEvents[0].PromptStopReason != acp.PromptStopReasonCancelled {
			t.Fatalf("canceled prompt events = %#v, want one canceled done event", canceledEvents)
		}
		if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
		}
		if session.IsPrompting() {
			t.Fatal("session IsPrompting() = true after canceled prompt drain")
		}
		marker := requireTranscriptMarker(t, h.manager, session.ID, transcript.MarkerPromptCancel)
		if marker.Evidence["actor_kind"] != actingSessionActorKind ||
			marker.Evidence["actor_id"] != "sess-creator" {
			t.Fatalf("prompt cancel marker evidence = %#v, want acting session", marker.Evidence)
		}
		active, ok := h.manager.Get(session.ID)
		if !ok {
			t.Fatal("session removed after prompt cancellation")
		}
		if got := active.Info().State; got != StateActive {
			t.Fatalf("session state = %q, want %q", got, StateActive)
		}
		if got := h.driver.stopCalls; got != 0 {
			t.Fatalf("driver stop calls = %d, want 0", got)
		}
		if got := h.notifier.stoppedCount(); got != 0 {
			t.Fatalf("post-stop notifications = %d, want 0", got)
		}

		nextEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "next prompt")
		if err != nil {
			t.Fatalf("next Prompt() error = %v", err)
		}
		if events := collectEvents(t, nextEvents); len(events) != 1 || events[0].Type != acp.EventTypeDone {
			t.Fatalf("next Prompt() events = %#v, want one done event", events)
		}
	})

	t.Run("Should cancel prompt setup before driver prompt is registered", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			session.clearCurrentPromptCancel()
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		promptCtx, cancelPrompt := context.WithCancel(testutil.Context(t))
		session.setCurrentTurnID("turn-setup")
		session.setCurrentTurnSource(TurnSourceUser)
		session.setCurrentPromptCancel(cancelPrompt)

		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		select {
		case <-promptCtx.Done():
		default:
			t.Fatal("prompt setup context is still active after CancelPrompt()")
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
		if got := len(h.driver.interruptScopes); got != 1 {
			t.Fatalf("driver interrupt calls = %d, want 1", got)
		}
	})

	t.Run("Should no-op when a prompting session loses its process handle", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			reportSessionStop(t, h, session.ID)
		})

		session.setCurrentTurnSource(TurnSourceUser)
		session.clearProcess(time.Now().UTC())

		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should ignore cancel errors once the process is already done", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		session.setCurrentTurnSource(TurnSourceUser)
		h.driver.cancelHook = func(_ *fakeProcess) error {
			return errors.New("test: cancel after process exit")
		}
		h.driver.lastProcess().exit()

		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
	})

	t.Run("Should allow retry when the live provider cancel fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			reportSessionStop(t, h, session.ID)
		})

		session.setCurrentTurnSource(TurnSourceUser)
		var attempts atomic.Int32
		h.driver.cancelHook = func(_ *fakeProcess) error {
			if attempts.Add(1) == 1 {
				return errors.New("test: transient provider cancel failure")
			}
			return nil
		}

		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err == nil {
			t.Fatal("CancelPrompt(first) error = nil, want provider failure")
		}
		result, err := h.manager.CancelPrompt(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("CancelPrompt(retry) error = %v", err)
		}
		if result.Outcome != PromptCancelOutcomeCanceled {
			t.Fatalf("CancelPrompt(retry) = %#v, want canceled", result)
		}
		if got := h.driver.cancelCalls; got != 2 {
			t.Fatalf("driver cancel calls = %d, want 2", got)
		}
	})

	t.Run("Should allow retry when scoped tool interruption fails", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			reportSessionStop(t, h, session.ID)
		})

		session.setCurrentTurnID("turn-scoped-retry")
		session.setCurrentTurnSource(TurnSourceUser)
		h.driver.interruptErr = errors.New("test: transient scoped interrupt failure")

		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err == nil {
			t.Fatal("CancelPrompt(first) error = nil, want scoped interrupt failure")
		}
		h.driver.interruptErr = nil

		result, err := h.manager.CancelPrompt(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("CancelPrompt(retry) error = %v", err)
		}
		if result.Outcome != PromptCancelOutcomeCanceled || result.TurnID != "turn-scoped-retry" {
			t.Fatalf("CancelPrompt(retry) = %#v, want canceled scoped turn", result)
		}
		if got := h.driver.cancelCalls; got != 2 {
			t.Fatalf("driver cancel calls = %d, want 2", got)
		}
		if got := len(h.driver.interruptScopes); got != 2 {
			t.Fatalf("driver interrupt calls = %d, want 2", got)
		}
	})

	t.Run("Should no-op for an active session without a prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		result, err := h.manager.CancelPrompt(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if result.Outcome != PromptCancelOutcomeNothingInFlight {
			t.Fatalf("CancelPrompt() = %#v, want nothing-in-flight", result)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should no-op for a known stopped session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		result, err := h.manager.CancelPrompt(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if result.Outcome != PromptCancelOutcomeNothingInFlight {
			t.Fatalf("CancelPrompt() = %#v, want nothing-in-flight", result)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should return ErrSessionNotFound for an unknown session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		_, err := h.manager.CancelPrompt(testutil.Context(t), "missing")
		if !errors.Is(err, ErrSessionNotFound) {
			t.Fatalf("CancelPrompt(missing) error = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestPromptPersistsUserMessageBeforeDriverPrompt(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	var storedBeforePrompt []store.SessionEvent
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if err != nil {
			return nil, err
		}
		storedBeforePrompt = events

		ch := make(chan acp.AgentEvent, 1)
		ch <- acp.AgentEvent{
			Type:             acp.EventTypeDone,
			TurnID:           req.TurnID,
			StopReason:       string(acp.PromptStopReasonEndTurn),
			PromptStopReason: acp.PromptStopReasonEndTurn,
		}
		close(ch)
		return ch, nil
	}

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	for event := range eventsCh {
		_ = event
	}

	if len(storedBeforePrompt) != 1 {
		t.Fatalf("storedBeforePrompt = %d events, want 1", len(storedBeforePrompt))
	}
	if got := storedBeforePrompt[0].Type; got != acp.EventTypeUserMessage {
		t.Fatalf("storedBeforePrompt[0].Type = %q, want %q", got, acp.EventTypeUserMessage)
	}
	if !strings.Contains(storedBeforePrompt[0].Content, `"text":"remember me"`) {
		t.Fatalf("stored user_message content = %s", storedBeforePrompt[0].Content)
	}
}

func TestApplyAutomaticSessionTitleOwnsGeneratedIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should include the ellipsis inside the eight-word and sixty-four-rune bounds", func(t *testing.T) {
		t.Parallel()

		got := normalizeAutomaticSessionTitle(
			"aaaaaaa bbbbbbb ccccccc ddddddd eeeeeee fffffff ggggggg hhhhhhhh ninth",
		)
		if words := strings.Fields(got); len(words) != automaticSessionTitleMaxWords {
			t.Fatalf("normalized title words = %d, want %d; title=%q", len(words), automaticSessionTitleMaxWords, got)
		}
		if runes := utf8.RuneCountInString(got); runes != automaticSessionTitleMaxRunes {
			t.Fatalf("normalized title runes = %d, want %d; title=%q", runes, automaticSessionTitleMaxRunes, got)
		}
		if !strings.HasSuffix(got, "…") {
			t.Fatalf("normalized title = %q, want ellipsis", got)
		}
	})

	t.Run("Should hide the generated title until identity persistence completes", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})

		const wantTitle = "Checkout webhook retries"
		persistenceStarted := make(chan struct{})
		releasePersistence := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releasePersistence) }) }
		t.Cleanup(release)
		catalog.registerHook = func(ctx context.Context, info store.SessionInfo) error {
			if info.Name != wantTitle {
				return nil
			}
			close(persistenceStarted)
			select {
			case <-releasePersistence:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		type applyResult struct {
			applied bool
			err     error
		}
		resultCh := make(chan applyResult, 1)
		go func() {
			applied, applyErr := h.manager.ApplyAutomaticSessionTitle(
				testutil.Context(t),
				session.ID,
				wantTitle,
			)
			resultCh <- applyResult{applied: applied, err: applyErr}
		}()

		select {
		case <-persistenceStarted:
		case <-time.After(time.Second):
			t.Fatal("automatic title persistence did not reach the catalog")
		}
		if session.mu.TryRLock() {
			session.mu.RUnlock()
			t.Fatal("session read lock acquired during persistence, want blocked")
		}

		release()
		result := <-resultCh
		if result.err != nil {
			t.Fatalf("ApplyAutomaticSessionTitle() error = %v", result.err)
		}
		if !result.applied {
			t.Fatal("ApplyAutomaticSessionTitle() applied = false, want true")
		}
		if got := session.Info().Name; got != wantTitle {
			t.Fatalf("session title after persistence = %q, want %q", got, wantTitle)
		}
	})

	t.Run("Should persist the first generated title as durable session identity", func(t *testing.T) {
		t.Parallel()

		catalog := newRecordingSessionCatalog()
		h := newHarness(t, WithSessionCatalog(catalog))
		session, err := h.manager.Create(testutil.Context(t), CreateOpts{
			AgentName: "coder",
			Workspace: h.workspaceID,
			Type:      SessionTypeUser,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		t.Cleanup(func() {
			if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil &&
				!errors.Is(stopErr, ErrSessionNotFound) {
				t.Errorf("Stop() error = %v", stopErr)
			}
		})
		catalogEvents, cancelCatalogEvents, err := h.manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancelCatalogEvents()

		const wantTitle = "Checkout webhook retries"
		applied, err := h.manager.ApplyAutomaticSessionTitle(testutil.Context(t), session.ID, wantTitle)
		if err != nil {
			t.Fatalf("ApplyAutomaticSessionTitle() error = %v", err)
		}
		if !applied {
			t.Fatal("ApplyAutomaticSessionTitle() applied = false, want true")
		}
		reapplied, err := h.manager.ApplyAutomaticSessionTitle(
			testutil.Context(t),
			session.ID,
			"Later title must lose",
		)
		if err != nil {
			t.Fatalf("ApplyAutomaticSessionTitle(second) error = %v", err)
		}
		if reapplied {
			t.Fatal("ApplyAutomaticSessionTitle(second) applied = true, want false")
		}
		if got := session.Info().Name; got != wantTitle {
			t.Fatalf("session title = %q, want %q", got, wantTitle)
		}
		if got := readMeta(t, session.MetaPath()).Name; got != wantTitle {
			t.Fatalf("persisted session title = %q, want %q", got, wantTitle)
		}
		catalogSessions, err := catalog.ListSessions(testutil.Context(t), store.SessionListQuery{
			WorkspaceID: h.workspaceID,
			SessionType: string(SessionTypeUser),
		})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(catalogSessions) != 1 || catalogSessions[0].Name != wantTitle {
			t.Fatalf("catalog sessions = %#v, want one session titled %q", catalogSessions, wantTitle)
		}
		select {
		case event := <-catalogEvents:
			if event.Kind != CatalogEventUpserted || event.WorkspaceID != h.workspaceID ||
				event.SessionID != session.ID {
				t.Fatalf("catalog title event = %#v, want workspace-scoped upsert", event)
			}
		default:
			t.Fatal("catalog title event was not published")
		}

		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
		if err != nil {
			t.Fatalf("Resume() error = %v", err)
		}
		if got := resumed.Info().Name; got != wantTitle {
			t.Fatalf("resumed session title = %q, want %q", got, wantTitle)
		}
	})

	t.Run("Should preserve explicit and internal session identities", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name        string
			sessionName string
			sessionType Type
		}{
			{
				name:        "Should preserve an explicit user title",
				sessionName: "Release review",
				sessionType: SessionTypeUser,
			},
			{name: "Should leave a system session unnamed", sessionType: SessionTypeSystem},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				h := newHarness(t)
				session, err := h.manager.Create(testutil.Context(t), CreateOpts{
					AgentName: "coder",
					Name:      test.sessionName,
					Workspace: h.workspaceID,
					Type:      test.sessionType,
				})
				if err != nil {
					t.Fatalf("Create() error = %v", err)
				}
				t.Cleanup(func() {
					if stopErr := h.manager.Stop(testutil.Context(t), session.ID); stopErr != nil &&
						!errors.Is(stopErr, ErrSessionNotFound) {
						t.Errorf("Stop() error = %v", stopErr)
					}
				})

				applied, err := h.manager.ApplyAutomaticSessionTitle(
					testutil.Context(t),
					session.ID,
					"Ignore internal bookkeeping",
				)
				if err != nil {
					t.Fatalf("ApplyAutomaticSessionTitle() error = %v", err)
				}
				if applied {
					t.Fatal("ApplyAutomaticSessionTitle() applied = true, want explicit/internal identity preserved")
				}

				if got := session.Info().Name; got != test.sessionName {
					t.Fatalf("session title = %q, want preserved %q", got, test.sessionName)
				}
			})
		}
	})
}

func TestPromptRejectsConcurrentUserPromptWithoutPersistingSecondInput(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() {
		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
	})

	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent)
		go func() {
			defer close(events)
			if req.TurnID != "turn-1" {
				events <- acp.AgentEvent{
					Type:             acp.EventTypeDone,
					TurnID:           req.TurnID,
					StopReason:       string(acp.PromptStopReasonEndTurn),
					PromptStopReason: acp.PromptStopReasonEndTurn,
				}
				return
			}

			close(firstPromptEntered)
			<-releaseFirstPrompt

			ts := time.Now().UTC()
			events <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				TurnID:    req.TurnID,
				Timestamp: ts,
				Text:      "first prompt complete",
			}
			events <- acp.AgentEvent{
				Type:             acp.EventTypeDone,
				TurnID:           req.TurnID,
				Timestamp:        ts,
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}
		}()
		return events, nil
	}

	firstEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "first prompt")
	if err != nil {
		t.Fatalf("Prompt(first) error = %v", err)
	}
	<-firstPromptEntered

	_, err = h.manager.Prompt(testutil.Context(t), session.ID, "second prompt")
	if !errors.Is(err, ErrPromptInProgress) {
		t.Fatalf("Prompt(second) error = %v, want ErrPromptInProgress", err)
	}
	if got, want := len(h.driver.promptCalls), 1; got != want {
		t.Fatalf("len(promptCalls) while first prompt active = %d, want %d", got, want)
	}

	storedWhileBusy, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(while busy) error = %v", err)
	}
	if got, want := countEventType(storedWhileBusy, acp.EventTypeUserMessage), 1; got != want {
		t.Fatalf("countEventType(user_message) while busy = %d, want %d", got, want)
	}

	releaseOnce.Do(func() {
		close(releaseFirstPrompt)
	})
	_ = collectEvents(t, firstEvents)

	storedAfterRelease, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query(after release) error = %v", err)
	}
	if got, want := countEventType(storedAfterRelease, acp.EventTypeUserMessage), 1; got != want {
		t.Fatalf("countEventType(user_message) after release = %d, want %d", got, want)
	}

	thirdEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "third prompt")
	if err != nil {
		t.Fatalf("Prompt(third) error = %v", err)
	}
	_ = collectEvents(t, thirdEvents)

	if got, want := len(h.driver.promptCalls), 2; got != want {
		t.Fatalf("len(promptCalls) after recovery = %d, want %d", got, want)
	}
}

func TestPromptAugmenterPreservesStoredUserMessageAndAugmentsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		message string,
	) (string, error) {
		return "MEMORY RECALL\n\n" + message, nil
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("stored events = 0, want at least one event")
	}
	if got := h.driver.promptCalls[0].Message; got != "MEMORY RECALL\n\nremember me" {
		t.Fatalf("driver prompt message = %q, want augmented content", got)
	}
	if !strings.Contains(stored[0].Content, `"text":"remember me"`) {
		t.Fatalf("stored user_message content = %s, want original message", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "MEMORY RECALL") {
		t.Fatalf("stored user_message content = %s, want no augmentation block", stored[0].Content)
	}
}

func TestPromptSkillCommandsPreserveAuthoredTextAndExpandOnlyOperatorIngress(t *testing.T) {
	t.Parallel()

	service := &promptCommandServiceStub{}
	h := newHarness(t, WithCommandService(service))
	sess := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, sess.ID)
	})

	result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
		Message: "Please /review this change", AllowCommands: true,
		Caller: PromptCaller{Kind: "human", ID: "operator", Source: "http"},
	})
	if err != nil {
		t.Fatalf("SendPrompt() error = %v", err)
	}
	_ = collectEvents(t, result.Events)
	if got, want := service.catalogCalls, 1; got != want {
		t.Fatalf("Catalog() calls = %d, want %d", got, want)
	}
	if len(service.expanded) != 1 || service.expanded[0].Token != "/review" {
		t.Fatalf("Expand() invocations = %#v, want one /review", service.expanded)
	}
	if got, want := h.driver.promptCalls[0].Message, "VERIFIED REVIEW INSTRUCTIONS\n\nPlease /review this change"; got != want {
		t.Fatalf("driver prompt message = %q, want %q", got, want)
	}
	stored := readStoredEvents(t, sess)
	if len(stored) == 0 || !strings.Contains(stored[0].Content, `"text":"Please /review this change"`) ||
		!strings.Contains(stored[0].Content, `"skill_invocations"`) {
		t.Fatalf("stored prompt event = %#v, want authored text and durable invocation refs", stored)
	}
	if strings.Contains(stored[0].Content, "VERIFIED REVIEW INSTRUCTIONS") {
		t.Fatalf("stored prompt event contains provider-only instructions: %s", stored[0].Content)
	}

	for _, caller := range []PromptCaller{
		{},
		{Kind: "agent", ID: "agent-1", Source: "native_tool"},
	} {
		_, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "Please /review this untrusted prompt", AllowCommands: true, Caller: caller,
		})
		if err == nil {
			t.Fatalf("SendPrompt(untrusted caller %#v) error = nil, want rejection", caller)
		}
	}
	if got, want := service.catalogCalls, 1; got != want {
		t.Fatalf("Catalog() calls after untrusted ingress = %d, want %d", got, want)
	}
	if got, want := len(h.driver.promptCalls), 1; got != want {
		t.Fatalf("driver prompt calls after untrusted ingress = %d, want %d", got, want)
	}

	internalService := &promptCommandServiceStub{}
	internalHarness := newHarness(t, WithCommandService(internalService))
	internalSession := createSession(t, internalHarness)
	t.Cleanup(func() {
		reportSessionStop(t, internalHarness, internalSession.ID)
	})
	internalEvents, err := internalHarness.manager.Prompt(
		testutil.Context(t),
		internalSession.ID,
		"Please /review this internal prompt",
	)
	if err != nil {
		t.Fatalf("Prompt(internal) error = %v", err)
	}
	_ = collectEvents(t, internalEvents)
	if internalService.catalogCalls != 0 || len(internalService.expanded) != 0 {
		t.Fatalf(
			"internal command service calls = catalog %d expanded %#v, want none",
			internalService.catalogCalls,
			internalService.expanded,
		)
	}
	if got, want := internalHarness.driver.promptCalls[0].Message, "Please /review this internal prompt"; got != want {
		t.Fatalf("internal driver message = %q, want literal %q", got, want)
	}
}

type promptCommandServiceStub struct {
	catalogCalls int
	expanded     []commandpkg.Invocation
}

func (s *promptCommandServiceStub) Catalog(
	context.Context,
	*Info,
	compozyconfig.AgentDef,
) (commandpkg.Catalog, error) {
	s.catalogCalls++
	return commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), nil, []commandpkg.SkillSpec{{
		Name: "review", Description: "Review carefully", Available: true,
		Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
	}})
}

func (s *promptCommandServiceStub) Expand(
	_ context.Context,
	_ *Info,
	_ compozyconfig.AgentDef,
	invocations []commandpkg.Invocation,
	message string,
) (string, error) {
	s.expanded = append(s.expanded, invocations...)
	return "VERIFIED REVIEW INSTRUCTIONS\n\n" + message, nil
}

func TestPromptNetworkAugmenterPreservesStoredUserMessageAndAugmentsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		message string,
	) (string, error) {
		return message + "\n\nNETWORK AUGMENT", nil
	}))
	session := createLiveNetworkSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.PromptNetwork(
		testutil.Context(t),
		session.ID,
		"network message",
		acp.PromptNetworkMeta{
			MessageID: "msg-1",
			Kind:      "direct",
			Channel:   "builders",
			From:      "ops.peer",
		},
	)
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("stored events = 0, want at least one event")
	}
	if got := h.driver.promptCalls[0].Message; got != "network message\n\nNETWORK AUGMENT" {
		t.Fatalf("driver prompt message = %q, want augmented network content", got)
	}
	if got := h.driver.promptCalls[0].Meta.TurnSource; got != acp.PromptTurnSourceNetwork {
		t.Fatalf("driver turn source = %q, want %q", got, acp.PromptTurnSourceNetwork)
	}
	if !strings.Contains(stored[0].Content, `"text":"network message"`) {
		t.Fatalf("stored user_message content = %s, want original network message", stored[0].Content)
	}
	if strings.Contains(stored[0].Content, "NETWORK AUGMENT") {
		t.Fatalf("stored user_message content = %s, want no augmentation block", stored[0].Content)
	}
}

func TestPromptAugmenterPropagatesFailureAndSkipsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		_ string,
	) (string, error) {
		return "", errors.New("boom")
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me"); err == nil {
		t.Fatal("Prompt() error = nil, want augmentation failure")
	} else if !strings.Contains(err.Error(), "augment prompt input") {
		t.Fatalf("Prompt() error = %v, want augmentation error context", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(driver.promptCalls) = %d, want 0 after augmentation failure", got)
	}
}

func TestPromptAugmenterPropagatesCancellationAndSkipsDriverDispatch(t *testing.T) {
	t.Parallel()

	h := newHarness(t, WithPromptInputAugmenter(func(
		_ context.Context,
		_ *Session,
		_ string,
	) (string, error) {
		return "", context.Canceled
	}))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if _, err := h.manager.Prompt(testutil.Context(t), session.ID, "remember me"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Prompt() error = %v, want context.Canceled", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(driver.promptCalls) = %d, want 0 after canceled augmentation", got)
	}
}

func TestPromptWithOptsTracksTurnSourceAndClearsAfterPrompt(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createLiveNetworkSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	seenSources := make([]TurnSource, 0, 2)
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		seenSources = append(seenSources, session.CurrentTurnSource())

		ch := make(chan acp.AgentEvent, 1)
		ch <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
		close(ch)
		return ch, nil
	}

	firstEvents, err := h.manager.PromptWithOpts(testutil.Context(t), session.ID, PromptOpts{
		Message: "user prompt",
	})
	if err != nil {
		t.Fatalf("PromptWithOpts(user) error = %v", err)
	}
	_ = collectEvents(t, firstEvents)

	secondEvents, err := h.manager.PromptNetwork(testutil.Context(t), session.ID, "network prompt")
	if err != nil {
		t.Fatalf("PromptNetwork() error = %v", err)
	}
	_ = collectEvents(t, secondEvents)

	if got, want := len(h.driver.promptCalls), 2; got != want {
		t.Fatalf("len(promptCalls) = %d, want %d", got, want)
	}
	if got, want := h.driver.promptCalls[0].Meta.TurnSource, acp.PromptTurnSourceUser; got != want {
		t.Fatalf("promptCalls[0].Meta.TurnSource = %q, want %q", got, want)
	}
	networkMeta := acp.PromptNetworkMeta{
		MessageID: "msg-1",
		Kind:      "direct",
		Channel:   "builders",
		From:      "ops.peer",
	}
	thirdEvents, err := h.manager.PromptNetwork(testutil.Context(t), session.ID, "network prompt", networkMeta)
	if err != nil {
		t.Fatalf("PromptNetwork(with meta) error = %v", err)
	}
	_ = collectEvents(t, thirdEvents)
	if got, want := h.driver.promptCalls[2].Meta.TurnSource, acp.PromptTurnSourceNetwork; got != want {
		t.Fatalf("promptCalls[2].Meta.TurnSource = %q, want %q", got, want)
	}
	if h.driver.promptCalls[2].Meta.Network == nil {
		t.Fatal("promptCalls[2].Meta.Network = nil, want populated metadata")
	}
	if got, want := h.driver.promptCalls[2].Meta.Network.MessageID, networkMeta.MessageID; got != want {
		t.Fatalf("promptCalls[2].Meta.Network.MessageID = %q, want %q", got, want)
	}
	if !slices.Equal(seenSources, []TurnSource{TurnSourceUser, TurnSourceNetwork, TurnSourceNetwork}) {
		t.Fatalf(
			"seen turn sources = %#v, want %#v",
			seenSources,
			[]TurnSource{TurnSourceUser, TurnSourceNetwork, TurnSourceNetwork},
		)
	}
	if got := session.CurrentTurnSource(); got != "" {
		t.Fatalf("CurrentTurnSource() after prompts = %q, want empty", got)
	}
}

func TestPromptNetworkRejectsMultipleMetadataValues(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	_, err := h.manager.PromptNetwork(
		testutil.Context(t),
		session.ID,
		"network prompt",
		acp.PromptNetworkMeta{MessageID: "msg-1", Kind: "direct"},
		acp.PromptNetworkMeta{MessageID: "msg-2", Kind: "direct"},
	)
	if err == nil {
		t.Fatal("PromptNetwork(multiple meta) error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "at most one metadata value") {
		t.Fatalf("PromptNetwork(multiple meta) error = %v, want multiple-metadata validation", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(promptCalls) = %d, want 0 when PromptNetwork validation fails", got)
	}
}

func TestPromptWithOptsRejectsSyntheticTurnSourceUntilDedicatedPathExists(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	_, err := h.manager.PromptWithOpts(testutil.Context(t), session.ID, PromptOpts{
		Message:    "synthetic prompt",
		TurnSource: TurnSourceSynthetic,
	})
	if err == nil {
		t.Fatal("PromptWithOpts(synthetic) error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "dedicated synthetic submission path") {
		t.Fatalf("PromptWithOpts(synthetic) error = %v, want dedicated-path validation", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(promptCalls) = %d, want 0 when synthetic validation fails", got)
	}
}

func TestApprovePermissionRoutesToActiveSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	var (
		gotReq sessionApproveCapture
		called bool
	)
	h.driver.approveHook = func(proc *fakeProcess, req acp.ApproveRequest) error {
		called = true
		gotReq = sessionApproveCapture{
			SessionID: proc.handle.SessionID,
			RequestID: req.RequestID,
			TurnID:    req.TurnID,
			Decision:  req.Decision,
		}
		return nil
	}

	result, err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
		RequestID: "req-1",
		TurnID:    "turn-1",
		Decision:  "allow-once",
	})
	if err != nil {
		t.Fatalf("ApprovePermission() error = %v", err)
	}
	if result.Outcome != store.PendingInteractionOutcomeApplied ||
		result.RequestID != "req-1" || result.Decision != "allow-once" {
		t.Fatalf("ApprovePermission() result = %#v", result)
	}
	if !called {
		t.Fatal("ApprovePermission() did not reach the active session process")
	}
	if gotReq.RequestID != "req-1" || gotReq.TurnID != "turn-1" || gotReq.Decision != "allow-once" {
		t.Fatalf("approve request = %#v", gotReq)
	}
}

func TestApprovePermissionReturnsNotActiveForStoppedSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	_, err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
		RequestID: "req-1",
		Decision:  "allow-once",
	})
	if !errors.Is(err, ErrSessionNotActive) {
		t.Fatalf("ApprovePermission(stopped) error = %v, want ErrSessionNotActive", err)
	}
}

func TestApprovePermissionMapsPendingLookupErrors(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	testCases := []struct {
		name    string
		hookErr error
		wantErr error
	}{
		{
			name:    "ShouldMapNotFound",
			hookErr: acp.ErrPendingPermissionNotFound,
			wantErr: ErrPendingPermissionNotFound,
		},
		{
			name:    "ShouldMapConflict",
			hookErr: acp.ErrPendingPermissionConflict,
			wantErr: ErrPendingPermissionConflict,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			session := createSession(t, h)
			t.Cleanup(func() {
				reportSessionStop(t, h, session.ID)
			})

			h.driver.approveHook = func(*fakeProcess, acp.ApproveRequest) error {
				return tc.hookErr
			}
			_, err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
				RequestID: "req-1",
				Decision:  "allow-once",
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ApprovePermission() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestProcessExitDuringActivePromptPersistsAgentCrashedStopReason(t *testing.T) {
	t.Parallel()

	t.Run("Should emit process failure before finalizing when process exits during active prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		proc := h.driver.lastProcess()
		proc.handle.exitStatusFn = func() (subprocess.ExitStatus, bool) {
			select {
			case <-proc.done:
				return subprocess.ExitStatus{ExitCode: 23}, true
			default:
				return subprocess.ExitStatus{}, false
			}
		}
		t.Cleanup(func() {
			if _, ok := h.manager.Get(session.ID); ok {
				reportSessionStop(t, h, session.ID)
			}
		})

		source := make(chan acp.AgentEvent, 1)

		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			source <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: proc.handle.SessionID,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Text:      "partial before process exit",
			}
			return source, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "run a long command")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		processExitErr := acp.WrapFailure(
			store.FailureProcess,
			"ACP subprocess exited unexpectedly",
			errors.New("acp subprocess exited: exit status 23"),
		)
		var typedProcessExit *acp.FailureError
		if !errors.As(processExitErr, &typedProcessExit) || typedProcessExit.Kind != store.FailureProcess {
			t.Fatalf("process exit error = %#v, want typed process_exit failure", processExitErr)
		}
		proc.crash(processExitErr, "codex stderr after process exit")
		close(source)

		events := collectEvents(t, eventsCh)
		if got, want := len(events), 2; got != want {
			t.Fatalf("Prompt() events = %#v, want partial chunk and one process terminal", events)
		}
		if got, want := events[0].Text, "partial before process exit"; got != want {
			t.Fatalf("Prompt() first event text = %q, want %q", got, want)
		}
		terminal := events[1]
		if terminal.Type != acp.EventTypeError || terminal.Failure == nil ||
			terminal.Failure.Kind != store.FailureProcess {
			t.Fatalf("Prompt() terminal event = %#v, want process_exit", terminal)
		}
		if terminal.Failure.CrashBundlePath == "" {
			t.Fatalf("Prompt() terminal failure = %#v, want public crash bundle path", terminal.Failure)
		}
		publicBundle := readCrashBundleDocument(t, terminal.Failure.CrashBundlePath)
		if publicBundle.Process == nil || publicBundle.Process.ExitCode == nil ||
			*publicBundle.Process.ExitCode != 23 {
			t.Fatalf("public crash bundle process = %#v, want exit code 23 before stop", publicBundle.Process)
		}
		if got, want := publicBundle.Stderr, "codex stderr after process exit"; got != want {
			t.Fatalf("public crash bundle stderr = %q, want %q", got, want)
		}

		h.notifier.waitForStopped(t, session.ID)
		if got := h.driver.stopCalls; got != 0 {
			t.Fatalf("driver stop calls = %d, want none for an already exited process", got)
		}
		if got, want := h.notifier.stoppedCount(), 1; got != want {
			t.Fatalf("stopped notifications = %d, want %d", got, want)
		}

		meta := readMeta(t, session.MetaPath())
		if got, want := *meta.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("meta.StopReason = %q, want %q", got, want)
		}
		if meta.Failure == nil || meta.Failure.Kind != store.FailureProcess {
			t.Fatalf("meta.Failure = %#v, want process_exit", meta.Failure)
		}
		if got, want := meta.Failure.CrashBundlePath, terminal.Failure.CrashBundlePath; got != want {
			t.Fatalf("final crash bundle = %q, want public event bundle %q", got, want)
		}

		stored := readStoredEvents(t, session)
		if !containsEventType(stored, acp.EventTypeAgentMessage) ||
			!containsEventType(stored, acp.EventTypeError) ||
			containsEventType(stored, acp.EventTypeDone) {
			t.Fatalf("stored events = %#v, want partial chunk, process error, and no done", stored)
		}
		stopEvent := storedEventByType(t, stored, EventTypeSessionStopped)
		stopPayload := decodeStoredEventPayload(t, stopEvent)
		if got, want := stopPayload["stop_reason"], string(store.StopAgentCrashed); got != want {
			t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
		}
	})

	t.Run("Should promote a generic terminal when exit status proves the subprocess died", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		proc := h.driver.lastProcess()
		proc.handle.exitStatusFn = func() (subprocess.ExitStatus, bool) {
			return subprocess.ExitStatus{ExitCode: 23}, true
		}
		h.driver.stopHook = func(proc *fakeProcess) error {
			proc.crash(
				acp.WrapFailure(
					store.FailureProcess,
					"ACP subprocess exited unexpectedly",
					errors.New("acp subprocess exited: exit status 23"),
				),
				"codex stderr after status observation",
			)
			return nil
		}
		t.Cleanup(func() {
			if _, ok := h.manager.Get(session.ID); ok {
				reportSessionStop(t, h, session.ID)
			}
		})

		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			source := make(chan acp.AgentEvent, 1)
			source <- acp.AgentEvent{
				Type:      acp.EventTypeError,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Error:     "context canceled",
				Failure: &store.SessionFailure{
					Kind:    store.FailureCanceled,
					Summary: "context canceled",
				},
			}
			close(source)
			return source, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "run a long command")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		events := collectEvents(t, eventsCh)
		if got, want := len(events), 1; got != want {
			t.Fatalf("Prompt() events = %#v, want one promoted terminal", events)
		}
		terminal := events[0]
		if terminal.Type != acp.EventTypeError || terminal.Failure == nil ||
			terminal.Failure.Kind != store.FailureProcess {
			t.Fatalf("Prompt() terminal event = %#v, want process_exit", terminal)
		}
		if !strings.Contains(terminal.Error, "exit code 23") {
			t.Fatalf("Prompt() terminal error = %q, want stable exit status diagnostic", terminal.Error)
		}
		publicBundle := readCrashBundleDocument(t, terminal.Failure.CrashBundlePath)
		if publicBundle.Process == nil || publicBundle.Process.ExitCode == nil ||
			*publicBundle.Process.ExitCode != 23 {
			t.Fatalf("public crash bundle process = %#v, want exit code 23 before stop", publicBundle.Process)
		}
		if got, want := publicBundle.Stderr, "codex stderr after status observation"; got != want {
			t.Fatalf("public crash bundle stderr = %q, want %q", got, want)
		}

		h.notifier.waitForStopped(t, session.ID)
		if got, want := h.driver.stopCalls, 1; got != want {
			t.Fatalf("driver stop calls = %d, want %d", got, want)
		}
		if got, want := h.notifier.stoppedCount(), 1; got != want {
			t.Fatalf("stopped notifications = %d, want %d", got, want)
		}
	})
}

func TestPromptSerializesSetupAgainstConcurrentStop(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)

	promptEntered := make(chan struct{})
	releasePrompt := make(chan struct{})
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		close(promptEntered)
		<-releasePrompt
		events := make(chan acp.AgentEvent)
		close(events)
		return events, nil
	}

	promptDone := make(chan error, 1)
	go func() {
		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			promptDone <- err
			return
		}
		for event := range eventsCh {
			_ = event
		}
		promptDone <- nil
	}()

	<-promptEntered

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- h.manager.Stop(testutil.Context(t), session.ID)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("Stop() returned before prompt setup finished: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(releasePrompt)

	if err := <-promptDone; err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestWaitForPromptDrains(t *testing.T) {
	t.Run("Should wait for active prompt pump exit", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		promptEvents := make(chan acp.AgentEvent)
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return promptEvents, nil
		}

		eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		waitDone := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
			defer cancel()
			waitDone <- h.manager.WaitForPromptDrains(ctx)
		}()

		select {
		case err := <-waitDone:
			t.Fatalf("WaitForPromptDrains() returned before prompt source closed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		close(promptEvents)
		_ = collectEvents(t, eventsCh)

		if err := <-waitDone; err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
		}
	})

	t.Run("Should make shutdown wait for every tracked prompt drain", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		finishDrain := h.manager.trackPromptDrain()
		shutdownDone := make(chan error, 1)
		go func() {
			ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
			defer cancel()
			shutdownDone <- h.manager.Shutdown(ctx)
		}()

		select {
		case err := <-shutdownDone:
			t.Fatalf("Shutdown() returned before prompt drain completed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		finishDrain()
		if err := <-shutdownDone; err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	t.Run("Should join a queued synthetic forward through the prompt task owner", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		source := make(chan acp.AgentEvent, 1)
		out := make(chan acp.AgentEvent)
		source <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "queued"}
		h.manager.startTrackedPromptTask(func() {
			h.manager.forwardQueuedSyntheticPrompt("session-missing", out, source)
		})

		waitCtx, cancelWait := context.WithCancel(testutil.Context(t))
		cancelWait()
		if err := h.manager.WaitForPromptDrains(waitCtx); !errors.Is(err, context.Canceled) {
			t.Fatalf("WaitForPromptDrains(blocked forward) error = %v, want cancellation", err)
		}

		event := <-out
		if event.Text != "queued" {
			t.Fatalf("forwarded event text = %q, want queued", event.Text)
		}
		close(source)
		if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
			t.Fatalf("WaitForPromptDrains(released forward) error = %v", err)
		}
	})

	t.Run("Should cancel and join live process watchers during shutdown", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "bind the runtime")
		if err != nil {
			t.Fatalf("Prompt(bind runtime) error = %v", err)
		}
		collectEvents(t, events)

		proc := h.driver.lastProcess()
		if proc == nil {
			t.Fatal("bound process = nil")
		}

		ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
		defer cancel()
		if err := h.manager.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}

		select {
		case <-proc.done:
			t.Fatal("Shutdown() stopped the process, want only watcher cancellation")
		default:
		}
		h.driver.mu.Lock()
		stopCalls := h.driver.stopCalls
		h.driver.mu.Unlock()
		if stopCalls != 0 {
			t.Fatalf("driver stop calls = %d, want 0", stopCalls)
		}
	})
}

func TestNormalizeEventSetsTimestampOnlyWhenZero(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	now := h.manager.now()

	normalized := h.manager.normalizeEvent(session, "turn-1", acp.AgentEvent{})
	if normalized.Timestamp.IsZero() {
		t.Fatal("normalizeEvent() left zero timestamp")
	}

	explicit := time.Date(2026, 4, 3, 14, 0, 0, 0, time.UTC)
	preserved := h.manager.normalizeEvent(session, "turn-1", acp.AgentEvent{Timestamp: explicit})
	if !preserved.Timestamp.Equal(explicit) {
		t.Fatalf("normalizeEvent() timestamp = %v, want %v", preserved.Timestamp, explicit)
	}
	if normalized.Timestamp.Before(now) {
		t.Fatalf("normalizeEvent() timestamp = %v, want >= %v", normalized.Timestamp, now)
	}
	if normalized.PromptRuntimeSnapshot() == nil {
		t.Fatal("normalizeEvent() runtime = nil, want session runtime fallback")
	}
	if normalized.PromptRuntimeSnapshot().Provider != session.Provider {
		t.Fatalf(
			"normalizeEvent() runtime provider = %q, want %q",
			normalized.PromptRuntimeSnapshot().Provider,
			session.Provider,
		)
	}
}

func TestCreateInvokesPromptAssemblerWhenConfigured(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	var (
		called         bool
		gotWorkspace   string
		gotAgentName   string
		gotAgentPrompt string
	)
	h.manager = newManagerWithHarness(
		t,
		h,
		WithPromptAssembler(
			promptAssemblerFunc(
				func(_ context.Context, agent compozyconfig.AgentDef, workspace *workspacepkg.ResolvedWorkspace) (string, error) {
					called = true
					gotWorkspace = workspace.RootDir
					gotAgentName = agent.Name
					gotAgentPrompt = agent.Prompt
					return agent.Prompt + "\n\nmemory block", nil
				},
			),
		),
	)

	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if !called {
		t.Fatal("Create() did not invoke the configured prompt assembler")
	}
	if gotWorkspace != h.workspace {
		t.Fatalf("assembler workspace = %q, want %q", gotWorkspace, h.workspace)
	}
	if gotAgentName != "coder" {
		t.Fatalf("assembler agent name = %q, want %q", gotAgentName, "coder")
	}
	if gotAgentPrompt != "You are a coding assistant." {
		t.Fatalf("assembler prompt = %q, want original agent prompt", gotAgentPrompt)
	}
	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant.\n\nmemory block" {
		t.Fatalf("start system prompt = %q, want assembled prompt", got)
	}
}

func TestCreateInvokesStartupPromptOverlayWhenConfigured(t *testing.T) {
	t.Parallel()

	h := newHarness(t)

	var (
		called       bool
		gotChannel   string
		gotType      Type
		gotWorkspace string
	)
	h.manager = newManagerWithHarness(
		t,
		h,
		WithPromptAssembler(nil),
		WithStartupPromptOverlay(
			startupPromptOverlayFunc(func(
				_ context.Context,
				startup StartupPromptContext,
				prompt string,
			) (string, error) {
				called = true
				gotChannel = startup.NetworkParticipation.ChannelID
				gotType = startup.SessionType
				gotWorkspace = startup.Workspace
				return prompt + "\n\noverlay block", nil
			}),
		),
	)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if !called {
		t.Fatal("Create() did not invoke the configured startup prompt overlay")
	}
	if gotChannel != "builders" {
		t.Fatalf("startup overlay channel = %q, want %q", gotChannel, "builders")
	}
	if gotType != SessionTypeUser {
		t.Fatalf("startup overlay session type = %q, want %q", gotType, SessionTypeUser)
	}
	if gotWorkspace != h.workspace {
		t.Fatalf("startup overlay workspace = %q, want %q", gotWorkspace, h.workspace)
	}
	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant.\n\noverlay block" {
		t.Fatalf("start system prompt = %q, want overlay output", got)
	}
}

func TestCreateWithChannelAppendsBundledNetworkSkillAfterPromptAssembly(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	networkSkill, err := skillbundled.LoadResource(testBundledCompozySkillName, testBundledNetworkReference)
	if err != nil {
		t.Fatalf("LoadResource(%q, %q) error = %v", testBundledCompozySkillName, testBundledNetworkReference, err)
	}
	networkSkill = strings.TrimSpace(networkSkill)

	h.manager = newManagerWithHarness(
		t,
		h,
		WithPromptAssembler(
			startupPromptAssemblerFunc(
				func(
					_ context.Context,
					startup StartupPromptContext,
					agent compozyconfig.AgentDef,
					workspace *workspacepkg.ResolvedWorkspace,
				) (string, error) {
					if got, want := workspace.RootDir, h.workspace; got != want {
						t.Fatalf("assembler workspace = %q, want %q", got, want)
					}
					prompt := agent.Prompt + "\n\nmemory block"
					if startup.NetworkParticipation.Mode != participation.ModeLive {
						return prompt, nil
					}
					return prompt + "\n\n" + networkSkill, nil
				},
			),
		),
	)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName:                    "coder",
		Name:                         "networked",
		Workspace:                    h.workspaceID,
		ResolvedNetworkParticipation: testLiveParticipationPtr(h.workspaceID, "builders"),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	wantPrompt := "You are a coding assistant.\n\nmemory block\n\n" + networkSkill
	if got := h.driver.startCalls[0].SystemPrompt; got != wantPrompt {
		t.Fatalf("start system prompt = %q, want %q", got, wantPrompt)
	}
	if got := strings.Count(h.driver.startCalls[0].SystemPrompt, networkSkill); got != 1 {
		t.Fatalf("network skill occurrences = %d, want 1", got)
	}
}

func TestCreateUsesRawPromptWhenAssemblerIsNil(t *testing.T) {
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
		reportSessionStop(t, h, session.ID)
	})

	if got := h.driver.startCalls[0].SystemPrompt; got != "You are a coding assistant." {
		t.Fatalf("start system prompt = %q, want raw agent prompt", got)
	}
}

func TestCreateAppliesDreamPermissionsOverride(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.cfg.Permissions.Mode = compozyconfig.PermissionModeDenyAll
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{
			{
				Name:     compozyconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
		},
	})
	h.manager = newManagerWithHarness(t, h)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
		Type:      SessionTypeDream,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := h.driver.startCalls[0].Permissions; got != compozyconfig.PermissionModeApproveAll {
		t.Fatalf("start permissions = %q, want %q", got, compozyconfig.PermissionModeApproveAll)
	}
}

func TestCreateUsesConfiguredPermissionsForUserSessions(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	h.cfg.Permissions.Mode = compozyconfig.PermissionModeDenyAll
	h.resolver.upsert(&workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      h.workspaceID,
			RootDir: h.workspace,
			Name:    h.workspaceName,
		},
		Config: h.cfg,
		Agents: []compozyconfig.AgentDef{
			{
				Name:     compozyconfig.DefaultAgentName,
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
			{
				Name:     "coder",
				Provider: "claude",
				Prompt:   "You are a coding assistant.",
			},
		},
	})
	h.manager = newManagerWithHarness(t, h)

	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Workspace: h.workspaceID,
		Type:      SessionTypeUser,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	if got := h.driver.startCalls[0].Permissions; got != compozyconfig.PermissionModeDenyAll {
		t.Fatalf("start permissions = %q, want %q", got, compozyconfig.PermissionModeDenyAll)
	}
}
