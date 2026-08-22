package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestPromptCallerCancellationContract(t *testing.T) {
	t.Run("Should persist a provider burst while the delivery consumer is stalled", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.manager = newManagerWithHarness(t, h, WithPromptBufferSize(1))
		notifierDrainCtx, cancelNotifierDrain := context.WithCancel(testutil.Context(t))
		defer cancelNotifierDrain()
		go func() {
			for {
				select {
				case <-notifierDrainCtx.Done():
					return
				case <-h.notifier.eventSignal:
				}
			}
		}()
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		const eventCount = sessionEventSubscriberBuffer + 32
		source := make(chan acp.AgentEvent, eventCount+1)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			for index := range eventCount {
				source <- acp.AgentEvent{
					Type:       acp.EventTypeToolCall,
					TurnID:     req.TurnID,
					ToolCallID: fmt.Sprintf("tool-%03d", index),
					Timestamp:  time.Now().UTC(),
				}.WithTool("Read", nil, false)
			}
			source <- acp.AgentEvent{
				Type:      acp.EventTypeDone,
				TurnID:    req.TurnID,
				Timestamp: time.Now().UTC(),
			}
			close(source)
			return source, nil
		}

		deliveryCtx, cancelDelivery := context.WithCancel(testutil.Context(t))
		defer cancelDelivery()
		result, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message:         "run a provider burst",
			DeliveryContext: deliveryCtx,
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}

		drainCtx, cancelDrain := context.WithTimeout(testutil.Context(t), 5*time.Second)
		defer cancelDrain()
		if err := h.manager.WaitForPromptDrains(drainCtx); err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v, delivery consumer blocked execution", err)
		}

		events := collectEvents(t, result.Events)
		if got, want := len(events), eventCount+1; got != want {
			t.Fatalf("delivered events = %d, want %d", got, want)
		}
		for index := range eventCount {
			if got, want := events[index].ToolCallID, fmt.Sprintf("tool-%03d", index); got != want {
				t.Fatalf("event %d tool call id = %q, want %q", index, got, want)
			}
		}
		if got := events[len(events)-1].Type; got != acp.EventTypeDone {
			t.Fatalf("last event type = %q, want %q", got, acp.EventTypeDone)
		}
	})

	t.Run("Should keep accepted prompt execution after delivery context cancellation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		driver := &promptContextCapturingDriver{fakeDriver: h.driver}
		h.manager = newManagerWithHarness(t, h, WithDriver(driver))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		source := make(chan acp.AgentEvent, 2)
		var turnID string
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID = req.TurnID
			return source, nil
		}

		deliveryCtx, cancelDelivery := context.WithCancel(testutil.Context(t))
		result, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message:         "hello",
			DeliveryContext: deliveryCtx,
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		if result.Events == nil {
			t.Fatal("SendPrompt().Events = nil, want accepted stream")
		}
		providerCtx := driver.lastPromptContext(t)
		if !session.IsPrompting() {
			t.Fatal("session IsPrompting() = false after provider prompt started")
		}

		cancelDelivery()
		if !errors.Is(deliveryCtx.Err(), context.Canceled) {
			t.Fatalf("delivery context err = %v, want context.Canceled", deliveryCtx.Err())
		}
		select {
		case <-providerCtx.Done():
			t.Fatalf("provider context canceled with delivery context: %v", providerCtx.Err())
		default:
		}

		source <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: session.Info().ACPSessionID,
			TurnID:    turnID,
			Timestamp: time.Date(2026, 5, 17, 17, 0, 0, 0, time.UTC),
			Text:      "still running after detach",
		}
		source <- acp.AgentEvent{
			Type:      acp.EventTypeDone,
			SessionID: session.Info().ACPSessionID,
			TurnID:    turnID,
			Timestamp: time.Date(2026, 5, 17, 17, 0, 1, 0, time.UTC),
		}
		close(source)
		if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
		}
		storedEvents, queryErr := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query() error = %v", queryErr)
		}
		if countEventType(storedEvents, acp.EventTypeUserMessage) != 1 ||
			countEventType(storedEvents, acp.EventTypeAgentMessage) != 1 ||
			countEventType(storedEvents, acp.EventTypeDone) != 1 {
			t.Fatalf("stored prompt events = %#v, want one user, agent, and done event", storedEvents)
		}
		if session.IsPrompting() {
			t.Fatal("session IsPrompting() = true after prompt drain")
		}
		if events := collectEvents(t, result.Events); len(events) != 0 {
			t.Fatalf("delivered events after delivery cancellation = %d, want 0", len(events))
		}
	})

	t.Run("Should keep accepted prompt execution after caller context cancellation", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		driver := &promptContextCapturingDriver{fakeDriver: h.driver}
		h.manager = newManagerWithHarness(t, h, WithDriver(driver))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		source := make(chan acp.AgentEvent, 1)
		var turnID string
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID = req.TurnID
			return source, nil
		}

		callerCtx, cancelCaller := context.WithCancel(testutil.Context(t))
		eventsCh, err := h.manager.Prompt(callerCtx, session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		providerCtx := driver.lastPromptContext(t)
		if !session.IsPrompting() {
			t.Fatal("session IsPrompting() = false after provider prompt started")
		}

		cancelCaller()
		select {
		case <-callerCtx.Done():
		default:
			t.Fatal("caller context is still active after cancel")
		}
		select {
		case <-providerCtx.Done():
			t.Fatalf("provider context canceled with caller context: %v", providerCtx.Err())
		default:
		}
		if !session.IsPrompting() {
			t.Fatal("session prompting = false after caller cancellation, want active prompt execution")
		}

		source <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			SessionID: session.Info().ACPSessionID,
			TurnID:    turnID,
			Timestamp: time.Date(2026, 5, 17, 16, 0, 0, 0, time.UTC),
			Text:      "still running",
		}
		h.notifier.waitForAgentEvent(t, session.ID, acp.EventTypeAgentMessage)
		storedEvents, queryErr := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
		if queryErr != nil {
			t.Fatalf("Query() error = %v", queryErr)
		}
		if got := countEventType(storedEvents, acp.EventTypeAgentMessage); got != 1 {
			t.Fatalf("stored agent_message events = %d, want 1", got)
		}

		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		select {
		case <-providerCtx.Done():
		default:
			t.Fatal("provider context is still active after CancelPrompt()")
		}
		close(source)
		if events := collectEvents(t, eventsCh); len(events) != 0 {
			t.Fatalf("delivered events after caller cancellation = %d, want 0", len(events))
		}
		if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
		}
		if session.IsPrompting() {
			t.Fatal("session IsPrompting() = true after canceled prompt drain")
		}
	})
}

func TestPromptRuntimeRecovery(t *testing.T) {
	t.Parallel()

	t.Run("Should replace the failed runtime and replay the interrupted turn", func(t *testing.T) {
		t.Parallel()

		startedHooks := make(chan hookspkg.SessionRuntimeRecoveryStartedPayload, 1)
		succeededHooks := make(chan hookspkg.SessionRuntimeRecoverySucceededPayload, 1)
		dispatcher := &spyHookDispatcher{
			dispatchSessionRuntimeRecoveryStartedFn: func(
				_ context.Context,
				payload hookspkg.SessionRuntimeRecoveryStartedPayload,
			) (hookspkg.SessionRuntimeRecoveryStartedPayload, error) {
				startedHooks <- payload
				return payload, nil
			},
			dispatchSessionRuntimeRecoverySucceededFn: func(
				_ context.Context,
				payload hookspkg.SessionRuntimeRecoverySucceededPayload,
			) (hookspkg.SessionRuntimeRecoverySucceededPayload, error) {
				succeededHooks <- payload
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		h.manager.promptRecoveryDelays = []time.Duration{0}
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil &&
				!errors.Is(err, ErrSessionNotFound) {
				t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
			}
		})

		var promptCalls atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			switch promptCalls.Add(1) {
			case 1:
				proc.crash(errors.New("provider process exited"), "provider disconnected")
				events <- acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, SessionID: proc.handle.SessionID,
					TurnID: req.TurnID, Timestamp: time.Now().UTC(), Text: "partial output",
				}
				events <- acp.AgentEvent{
					Type: acp.EventTypeError, SessionID: proc.handle.SessionID,
					TurnID: req.TurnID, Timestamp: time.Now().UTC(), Error: "peer disconnected before response",
					Failure: &store.SessionFailure{
						Kind: store.FailureTransport, Summary: "peer disconnected before response",
					},
				}
			default:
				events <- acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, SessionID: proc.handle.SessionID,
					TurnID: req.TurnID, Timestamp: time.Now().UTC(), Text: "recovered output",
				}
				events <- acp.AgentEvent{
					Type: acp.EventTypeDone, SessionID: proc.handle.SessionID,
					TurnID: req.TurnID, Timestamp: time.Now().UTC(),
				}
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "complete a long task")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		delivered := collectEvents(t, events)
		for _, event := range delivered {
			if event.Type == acp.EventTypeError {
				t.Fatalf("delivered terminal error after recoverable disconnect: %#v", event)
			}
		}
		for _, want := range []string{
			acp.EventTypeRuntimeRecoveryStarted,
			acp.EventTypeRuntimeRecoverySucceeded,
			acp.EventTypeDone,
		} {
			if countAgentEvents(delivered, want) != 1 {
				t.Fatalf("delivered %q events = %d, want 1: %#v", want, countAgentEvents(delivered, want), delivered)
			}
		}

		h.driver.mu.Lock()
		startCalls := len(h.driver.startCalls)
		promptRequests := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
		h.driver.mu.Unlock()
		if startCalls != 2 {
			t.Fatalf("driver Start() calls = %d, want initial runtime plus one recovery", startCalls)
		}
		if len(promptRequests) != 2 || promptRequests[0].TurnID != promptRequests[1].TurnID {
			t.Fatalf("driver Prompt() requests = %#v, want one replay with the original turn id", promptRequests)
		}
		if got := countEventType(readStoredEvents(t, session), acp.EventTypeUserMessage); got != 1 {
			t.Fatalf("stored user messages = %d, want exactly one across replay", got)
		}
		if got := session.Info().State; got != StateActive {
			t.Fatalf("session state = %q, want %q after recovery", got, StateActive)
		}
		if got, want := session.Info().RuntimeGeneration, int64(2); got != want {
			t.Fatalf("runtime generation = %d, want %d", got, want)
		}
		started := <-startedHooks
		succeeded := <-succeededHooks
		if started.TurnID != succeeded.TurnID || started.Generation != 2 || succeeded.Generation != 2 ||
			started.Attempt != 1 || succeeded.Attempt != 1 {
			t.Fatalf("recovery hooks = started %#v, succeeded %#v", started, succeeded)
		}
	})

	t.Run("Should exhaust three recoveries before emitting one terminal failure", func(t *testing.T) {
		t.Parallel()

		exhaustedHooks := make(chan hookspkg.SessionRuntimeRecoveryExhaustedPayload, 1)
		dispatcher := &spyHookDispatcher{
			dispatchSessionRuntimeRecoveryExhaustedFn: func(
				_ context.Context,
				payload hookspkg.SessionRuntimeRecoveryExhaustedPayload,
			) (hookspkg.SessionRuntimeRecoveryExhaustedPayload, error) {
				exhaustedHooks <- payload
				return payload, nil
			},
		}
		h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
		h.manager.promptRecoveryDelays = []time.Duration{0, 0, 0}
		session := createSession(t, h)

		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- acp.AgentEvent{
				Type: acp.EventTypeError, SessionID: proc.handle.SessionID,
				TurnID: req.TurnID, Timestamp: time.Now().UTC(), Error: "transport unavailable",
				Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "transport unavailable"},
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "complete a long task")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		delivered := collectEvents(t, events)
		if got, want := countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryStarted), 3; got != want {
			t.Fatalf("recovery started events = %d, want %d", got, want)
		}
		if got, want := countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryExhausted), 1; got != want {
			t.Fatalf("recovery exhausted events = %d, want %d", got, want)
		}
		if got, want := countAgentEvents(delivered, acp.EventTypeError), 1; got != want {
			t.Fatalf("terminal error events = %d, want %d", got, want)
		}

		h.driver.mu.Lock()
		startCalls := len(h.driver.startCalls)
		h.driver.mu.Unlock()
		if startCalls != 4 {
			t.Fatalf("driver Start() calls = %d, want initial runtime plus three recoveries", startCalls)
		}
		exhausted := <-exhaustedHooks
		if exhausted.Attempt != 3 || exhausted.MaxAttempts != 3 || exhausted.Generation != 4 {
			t.Fatalf("recovery exhausted hook = %#v", exhausted)
		}
		h.notifier.waitForStopped(t, session.ID)
		if got, want := countEventType(readStoredEvents(t, session), acp.EventTypeError), 1; got != want {
			t.Fatalf("persisted terminal error events = %d, want %d", got, want)
		}
		if got, want := countTranscriptMarkers(
			t,
			h.manager,
			session.ID,
			transcript.MarkerProviderFailure,
		), 1; got != want {
			t.Fatalf("provider failure transcript markers = %d, want %d", got, want)
		}
	})
}

func TestDetachedPromptStopContext(t *testing.T) {
	t.Parallel()

	t.Run("Should detach prompt cancellation while preserving prompt values", func(t *testing.T) {
		t.Parallel()

		type promptKey struct{}
		const promptValue = "prompt-owned"
		promptCtx, cancelPrompt := context.WithCancel(
			context.WithValue(testutil.Context(t), promptKey{}, promptValue),
		)
		cancelPrompt()

		stopCtx, cancel := detachedPromptStopContext(promptCtx, &Manager{})
		defer cancel()

		if err := stopCtx.Err(); err != nil {
			t.Fatalf("detachedPromptStopContext(canceled) error = %v, want detached cleanup", err)
		}
		if got := stopCtx.Value(promptKey{}); got != promptValue {
			t.Fatalf("detachedPromptStopContext(canceled) value = %#v, want %q", got, promptValue)
		}
		if _, ok := stopCtx.Deadline(); !ok {
			t.Fatal("detachedPromptStopContext(canceled) deadline = none, want bounded cleanup")
		}
	})

	t.Run("Should use bounded manager lifecycle ownership when prompt context is absent", func(t *testing.T) {
		t.Parallel()

		type lifecycleKey struct{}
		const lifecycleValue = "manager-owned"
		lifecycleCtx, cancelLifecycle := context.WithCancel(
			context.WithValue(testutil.Context(t), lifecycleKey{}, lifecycleValue),
		)
		manager := &Manager{lifecycleCtx: lifecycleCtx}
		cancelLifecycle()

		stopCtx, cancel := detachedPromptStopContext(nilContextForGuardTest(), manager)
		defer cancel()

		if err := stopCtx.Err(); err != nil {
			t.Fatalf("detachedPromptStopContext() error = %v, want detached cleanup", err)
		}
		if got := stopCtx.Value(lifecycleKey{}); got != lifecycleValue {
			t.Fatalf("detachedPromptStopContext() lifecycle value = %#v, want %q", got, lifecycleValue)
		}
		if _, ok := stopCtx.Deadline(); !ok {
			t.Fatal("detachedPromptStopContext() deadline = none, want bounded cleanup")
		}
	})
}

func TestPromptTranscriptMarkerClassifiesStructuredMCPAuthReason(t *testing.T) {
	t.Parallel()

	t.Run("Should classify MCP auth from structured request error data", func(t *testing.T) {
		t.Parallel()

		kind, _, _, ok := promptTranscriptMarker(acp.AgentEvent{
			Type:  acp.EventTypeError,
			Error: "provider authentication failed",
			Failure: &store.SessionFailure{
				Kind: store.FailureProviderAuth,
			},
			Raw: []byte(`{"data":{"reason_codes":["mcp_auth_required"]}}`),
		})
		if !ok {
			t.Fatal("promptTranscriptMarker() ok = false, want true")
		}
		if kind != transcript.MarkerMCPAuthRequired {
			t.Fatalf("promptTranscriptMarker() kind = %q, want %q", kind, transcript.MarkerMCPAuthRequired)
		}
	})

	t.Run("Should stop MCP auth reason scanning at bounded JSON depth", func(t *testing.T) {
		t.Parallel()

		raw := strings.Repeat(`{"child":`, maxMCPAuthReasonJSONDepth+2) +
			`{"reason":"mcp_auth_required"}` +
			strings.Repeat(`}`, maxMCPAuthReasonJSONDepth+2)
		if eventHasMCPAuthReason(acp.AgentEvent{Raw: []byte(raw)}) {
			t.Fatal("eventHasMCPAuthReason() = true for reason beyond bounded scan depth")
		}
	})
}

func TestPromptTranscriptMarkerSuppressesExpectedCancellation(t *testing.T) {
	t.Run("Should suppress expected cancellation transcript markers", func(t *testing.T) {
		t.Parallel()

		_, _, _, ok := promptTranscriptMarker(acp.AgentEvent{
			Type:  acp.EventTypeError,
			Error: context.Canceled.Error(),
			Failure: &store.SessionFailure{
				Kind:    store.FailureCanceled,
				Summary: context.Canceled.Error(),
			},
		})
		if ok {
			t.Fatal("promptTranscriptMarker() ok = true for expected cancellation, want false")
		}
	})
}

func TestPromptFileMutationVerifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		recovered  bool
		terminal   string
		wantMarker int
	}{
		{name: "Should persist one verifier marker for an unrecovered failed write", wantMarker: 1},
		{name: "Should suppress the verifier marker after a later successful write", recovered: true},
		{
			name:       "Should persist one verifier marker when the failed-write turn ends with an error",
			terminal:   acp.EventTypeError,
			wantMarker: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := newHarness(t)
			sess := createSession(t, h)
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil &&
					!errors.Is(err, ErrSessionNotFound) {
					t.Errorf("Stop() error = %v", err)
				}
			})
			terminal := tc.terminal
			if terminal == "" {
				terminal = acp.EventTypeDone
			}
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent, 8)
				now := time.Now().UTC()
				events <- acp.AgentEvent{
					Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Timestamp: now,
					Text: "I attempted the requested file update.",
				}
				events <- fileMutationToolKindUpdate(req.TurnID, "write-failed", now)
				events <- fileMutationToolInputUpdate(t, req.TurnID, "write-failed", "src/retry.go", now)
				events <- acp.AgentEvent{
					Type: acp.EventTypeToolResult, TurnID: req.TurnID, ToolCallID: "write-failed", Timestamp: now,
				}.WithToolDetail("Write", nil, true, "permission denied")
				if tc.recovered {
					events <- fileMutationToolKindUpdate(req.TurnID, "write-recovered", now)
					events <- fileMutationToolInputUpdate(t, req.TurnID, "write-recovered", "src/retry.go", now)
					events <- acp.AgentEvent{
						Type: acp.EventTypeToolResult, TurnID: req.TurnID,
						ToolCallID: "write-recovered", Timestamp: now,
					}.WithTool("Write", nil, false)
				}
				terminalEvent := acp.AgentEvent{Type: terminal, TurnID: req.TurnID, Timestamp: now}
				if terminal == acp.EventTypeError {
					terminalEvent.Error = "provider turn failed"
				}
				events <- terminalEvent
				close(events)
				return events, nil
			}

			events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "Update the retry implementation")
			if err != nil {
				t.Fatalf("Prompt() error = %v", err)
			}
			collectEvents(t, events)
			stored, err := h.manager.Events(testutil.Context(t), sess.ID, store.EventQuery{})
			if err != nil {
				t.Fatalf("Events() error = %v", err)
			}
			markers := 0
			persistedEditKind := false
			for _, storedEvent := range stored {
				event, unmarshalErr := transcript.UnmarshalAgentEvent(storedEvent.Content)
				if unmarshalErr != nil {
					t.Fatalf("UnmarshalAgentEvent() error = %v", unmarshalErr)
				}
				if event.Type == acp.EventTypeToolCall && event.ToolKind() == "edit" {
					persistedEditKind = true
				}
				marker, ok := transcript.ParseMarker(event.Raw)
				if ok && marker.Kind == transcript.MarkerFileMutationUnverified {
					markers++
					if got := marker.Evidence["failure_count"]; got != float64(1) && got != 1 {
						t.Fatalf("marker failure_count = %#v, want 1", got)
					}
				}
			}
			if markers != tc.wantMarker {
				t.Fatalf("verifier markers = %d, want %d", markers, tc.wantMarker)
			}
			if !persistedEditKind {
				t.Fatal("persisted edit tool kind = false, want canonical edit evidence")
			}
			if tc.wantMarker > 0 {
				assertFileMutationMarkerNotifiedBeforeTerminal(t, h.notifier.eventsForSession(sess.ID), terminal)
			}
		})
	}

	t.Run("Should bound marker paths while retaining the total unresolved count", func(t *testing.T) {
		t.Parallel()

		verifier := fileMutationVerifier{}
		verifier.Observe(acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "Mutation summary"})
		const failures = maxFileMutationEvidencePaths + 3
		for index := range failures {
			callID := fmt.Sprintf("write-%d", index)
			verifier.Observe(fileMutationToolCall(
				t,
				"turn-bounded",
				callID,
				fmt.Sprintf("src/file-%02d.go", index),
				time.Now().UTC(),
			))
			verifier.Observe(acp.AgentEvent{
				Type: acp.EventTypeToolResult, ToolCallID: callID,
			}.WithToolDetail("Write", nil, true, "permission denied"))
		}

		_, evidence, ok := verifier.Marker()
		if !ok {
			t.Fatal("Marker() ok = false, want bounded marker")
		}
		if got := evidence["failure_count"]; got != failures {
			t.Fatalf("Marker() failure_count = %#v, want %d", got, failures)
		}
		paths, ok := evidence["paths"].([]string)
		if !ok || len(paths) != maxFileMutationEvidencePaths {
			t.Fatalf("Marker() paths = %#v, want %d entries", evidence["paths"], maxFileMutationEvidencePaths)
		}
		if truncated, ok := evidence["paths_truncated"].(bool); !ok || !truncated {
			t.Fatalf("Marker() paths_truncated = %#v, want true", evidence["paths_truncated"])
		}
	})
}

func assertFileMutationMarkerNotifiedBeforeTerminal(
	t *testing.T,
	events []acp.AgentEvent,
	terminal string,
) {
	t.Helper()
	markerIndex := -1
	terminalIndex := -1
	for index, event := range events {
		if marker, ok := transcript.ParseMarker(event.Raw); ok &&
			marker.Kind == transcript.MarkerFileMutationUnverified {
			markerIndex = index
		}
		if event.Type == terminal {
			terminalIndex = index
		}
	}
	if markerIndex < 0 || terminalIndex < 0 || markerIndex >= terminalIndex {
		t.Fatalf(
			"notifier marker index = %d, terminal index = %d, want marker before %q",
			markerIndex,
			terminalIndex,
			terminal,
		)
	}
}

func fileMutationToolCall(t *testing.T, turnID string, callID string, path string, at time.Time) acp.AgentEvent {
	t.Helper()

	input, err := json.Marshal(map[string]string{"file_path": path})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return acp.AgentEvent{
		Type: acp.EventTypeToolCall, TurnID: turnID, ToolCallID: callID, Timestamp: at,
	}.WithTool("Write", input, false).WithToolKind("edit")
}

func fileMutationToolKindUpdate(turnID string, callID string, at time.Time) acp.AgentEvent {
	return acp.AgentEvent{
		Type: acp.EventTypeToolCall, TurnID: turnID, ToolCallID: callID, Timestamp: at,
	}.WithTool("Write", nil, false).WithToolKind("edit")
}

func fileMutationToolInputUpdate(
	t *testing.T,
	turnID string,
	callID string,
	path string,
	at time.Time,
) acp.AgentEvent {
	t.Helper()

	event := fileMutationToolCall(t, turnID, callID, path, at)
	return event.WithToolKind("")
}

type promptContextCapturingDriver struct {
	*fakeDriver
	mu       sync.Mutex
	contexts []context.Context
}

func (d *promptContextCapturingDriver) Prompt(
	ctx context.Context,
	proc *AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	d.contexts = append(d.contexts, ctx)
	d.mu.Unlock()
	return d.fakeDriver.Prompt(ctx, proc, req)
}

func (d *promptContextCapturingDriver) lastPromptContext(t *testing.T) context.Context {
	t.Helper()

	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.contexts) == 0 {
		t.Fatal("driver prompt contexts = 0, want at least 1")
	}
	return d.contexts[len(d.contexts)-1]
}
