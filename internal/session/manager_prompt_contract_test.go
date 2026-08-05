package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
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

func TestPromptFatalProcessFailureStopsSessionAndAllowsFreshResumeFallback(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	originalACP := session.Info().ACPSessionID
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
				SessionID: originalACP,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
				Error:     `{"code":-32603,"message":"Internal error: The Claude Agent process exited unexpectedly. Please start a new session."}`,
				Failure: &store.SessionFailure{
					Kind:    store.FailureProcess,
					Summary: `{"code":-32603,"message":"Internal error: The Claude Agent process exited unexpectedly. Please start a new session."}`,
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
	if got, want := events[0].Type, acp.EventTypeError; got != want {
		t.Fatalf("Prompt() first event type = %q, want %q", got, want)
	}
	if events[0].Failure == nil || events[0].Failure.Kind != store.FailureProcess {
		t.Fatalf("Prompt() failure = %#v, want process_exit", events[0].Failure)
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

	h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
		if opts.ResumeSessionID != "" {
			return nil, fmt.Errorf(
				"%w: load session %q for %q: %w",
				acp.ErrLoadSessionFailed,
				opts.ResumeSessionID,
				opts.AgentName,
				&acpsdk.RequestError{
					Code:    -32002,
					Message: "Resource not found: " + opts.ResumeSessionID,
				},
			)
		}
		return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
	}

	resumed, err := h.manager.Resume(testutil.Context(t), session.ID)
	if err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, resumed.ID)
	})

	if got := h.driver.startCalls[1].ResumeSessionID; got != originalACP {
		t.Fatalf("first resume start ResumeSessionID = %q, want %q", got, originalACP)
	}
	if got := h.driver.startCalls[2].ResumeSessionID; got != "" {
		t.Fatalf("fallback resume start ResumeSessionID = %q, want empty", got)
	}
	if got := resumed.Info().ACPSessionID; got == "" || got == originalACP {
		t.Fatalf("resumed ACPSessionID = %q, want fresh ACP session id distinct from %q", got, originalACP)
	}
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

	t.Run("Should cancel driver prompt for an active prompting session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			session.clearCurrentTurnSource()
			reportSessionStop(t, h, session.ID)
		})

		promptEvents := make(chan acp.AgentEvent)
		promptStarted := make(chan struct{})
		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			close(promptStarted)
			return promptEvents, nil
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

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
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

		close(promptEvents)
		_ = collectEvents(t, eventsCh)
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

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
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

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
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

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("driver cancel calls = %d, want 1", got)
		}
	})

	t.Run("Should no-op for an active session without a prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			reportSessionStop(t, h, session.ID)
		})

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
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

		if err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		if got := h.driver.cancelCalls; got != 0 {
			t.Fatalf("driver cancel calls = %d, want 0", got)
		}
	})

	t.Run("Should return ErrSessionNotFound for an unknown session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)

		err := h.manager.CancelPrompt(testutil.Context(t), "missing")
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
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if err != nil {
			return nil, err
		}
		storedBeforePrompt = events

		ch := make(chan acp.AgentEvent)
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
	h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		seenSources = append(seenSources, session.CurrentTurnSource())

		ch := make(chan acp.AgentEvent)
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

	err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
		RequestID: "req-1",
		TurnID:    "turn-1",
		Decision:  "allow-once",
	})
	if err != nil {
		t.Fatalf("ApprovePermission() error = %v", err)
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

	err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
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
			err := h.manager.ApprovePermission(testutil.Context(t), session.ID, acp.ApproveRequest{
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

	t.Run("Should persist StopAgentCrashed when process exits during active prompt", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		source := make(chan acp.AgentEvent)
		promptStarted := make(chan struct{})
		var closePromptStarted sync.Once

		h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			closePromptStarted.Do(func() {
				close(promptStarted)
			})
			return source, nil
		}

		_, err := h.manager.Prompt(testutil.Context(t), session.ID, "run a long command")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		select {
		case <-promptStarted:
		case <-time.After(time.Second):
			t.Fatal("prompt hook did not start")
		}

		h.driver.lastProcess().exit()
		h.notifier.waitForStopped(t, session.ID)
		close(source)

		meta := readMeta(t, session.MetaPath())
		if got, want := *meta.StopReason, store.StopAgentCrashed; got != want {
			t.Fatalf("meta.StopReason = %q, want %q", got, want)
		}
		if meta.Failure == nil || meta.Failure.Kind != store.FailureProcess {
			t.Fatalf("meta.Failure = %#v, want process_exit", meta.Failure)
		}

		events := readStoredEvents(t, session)
		if !containsEventType(events, acp.EventTypeError) {
			t.Fatalf("stored events missing process-exit error: %#v", events)
		}
		stopEvent := storedEventByType(t, events, EventTypeSessionStopped)
		stopPayload := decodeStoredEventPayload(t, stopEvent)
		if got, want := stopPayload["stop_reason"], string(store.StopAgentCrashed); got != want {
			t.Fatalf("session_stopped stop_reason = %v, want %q", got, want)
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
