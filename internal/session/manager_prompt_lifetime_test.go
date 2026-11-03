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
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestPromptCallerCancellationContract(t *testing.T) {
	t.Run("Should release execution before delivering terminal output", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := createSession(t, h)
		finishing := make(chan struct{})
		release := make(chan struct{})
		unblock := sync.OnceFunc(func() { close(release) })
		t.Cleanup(unblock)
		h.manager.SetTurnEndNotifier(func(ctx context.Context, _ PromptRunIdentity) {
			close(finishing)
			select {
			case <-release:
			case <-ctx.Done():
			}
		})
		result, err := h.manager.SendPrompt(t.Context(), sess.ID, SendPromptOpts{Message: "finish before terminal"})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-finishing:
		case <-time.After(5 * time.Second):
			t.Fatal("execution did not reach finalization")
		}
		deadline := time.NewTimer(25 * time.Millisecond)
		defer deadline.Stop()
	wait:
		for {
			select {
			case event := <-result.Events:
				if isPromptTerminalEvent(event.Type) {
					t.Fatalf("terminal %q escaped before finalization", event.Type)
				}
			case <-deadline.C:
				break wait
			}
		}
		unblock()
		events := collectEvents(t, result.Events)
		if len(events) == 0 || events[len(events)-1].Type != acp.EventTypeDone {
			t.Fatalf("terminal delivery after finalization = %#v", events)
		}
	})

	t.Run("Should persist a provider burst while the delivery consumer is stalled", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		h.manager = newManagerWithHarness(t, h, WithPromptBufferSize(1))
		notifierDrainCtx, cancelNotifierDrain := context.WithCancel(testutil.Context(t))
		t.Cleanup(cancelNotifierDrain)
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

		const eventCount = promptDeliveryPageSize*3 + sessionEventSubscriberBuffer + 32
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

		waitForCondition(t, "provider burst persistence", func() bool {
			if session.IsPrompting() {
				return false
			}
			stored, queryErr := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
			return queryErr == nil && len(stored) == eventCount+2
		})

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
		if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
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
		close(source)
		if _, err := h.manager.AwaitTurnQuiesced(testutil.Context(t), session.ID, turnID); err != nil {
			t.Fatal(err)
		}
		select {
		case <-providerCtx.Done():
		default:
			t.Fatal("provider context is still active after verified turn quiescence")
		}
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

	t.Run("Should preserve maintenance and pending context after runtime recovery", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.promptRecoveryDelays = []time.Duration{0, 0, 0}
		session := createSession(t, h)
		session.replaceAdvertisedCommands([]store.SessionAdvertisedCommand{{Name: "compact"}}, h.manager.now())
		h.manager.stageResumeReplay(
			session.ID,
			renderResumeReplay(session.ID, nil, `[{"role":"user","content":"prior ordinary history"}]`, false, false),
		)
		var calls atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			if calls.Add(1) <= 3 {
				proc.crash(errors.New("provider process exited"), "provider disconnected")
				events <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: req.TurnID, Timestamp: h.manager.now(), Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "peer disconnected"}}
			} else {
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID, Timestamp: h.manager.now()}
			}
			close(events)
			return events, nil
		}
		_, events, err := h.manager.RequestCompaction(t.Context(), session.ID)
		if err != nil {
			t.Fatal(err)
		}
		delivered := collectEvents(t, events)
		if countAgentEvents(delivered, acp.EventTypeRuntimeRecoverySucceeded) != 3 {
			t.Fatalf("recovery events=%#v", delivered)
		}
		h.driver.mu.Lock()
		requests := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
		h.driver.mu.Unlock()
		if len(requests) != 4 {
			t.Fatalf("requests=%#v", requests)
		}
		for _, req := range requests {
			if req.Message != "/compact" || !req.Maintenance {
				t.Fatalf("recovery changed maintenance request=%#v", req)
			}
		}
		if h.manager.pendingResumeReplay(session.ID) == "" {
			t.Fatal("maintenance recovery consumed rebuild replay")
		}
	})

	t.Run("Should discard a recovery candidate when session stop wins the binding race", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		session := createSession(t, h)
		previous := session.processHandle()
		var candidate *fakeProcess
		h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if _, _, err := session.prepareStop(time.Now(), CauseUserRequested, "concurrent stop"); err != nil {
				return nil, err
			}
			candidate = newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "recovery-candidate")
			return candidate, nil
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("cleanup stop: %v", err)
			}
		})
		process, _, err := h.manager.recoverPromptRuntime(testutil.Context(t), session)
		if !errors.Is(err, ErrSessionNotActive) || process != nil {
			t.Fatalf("recovery = %v, %v; want no replacement and inactive error", process, err)
		}
		if session.Info().State != StateStopping || session.processHandle() != previous {
			t.Fatal("recovery changed the binding after stop claimed the session")
		}
		if candidate == nil || !isProcessDone(candidate.handle) {
			t.Fatal("discarded recovery candidate was not stopped")
		}
	})

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
		session.ReasoningEffort = "ultra"
		session.Speed = "fast"
		session.processHandle().setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{
			{ID: "effort", Kind: acp.SessionConfigOptionKindSelect, CurrentValueID: "ultra"},
		}})
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
		recoveredRuntime := h.driver.startCalls[startCalls-1]
		promptRequests := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
		h.driver.mu.Unlock()
		if startCalls != 2 {
			t.Fatalf("driver Start() calls = %d, want initial runtime plus one recovery", startCalls)
		}
		if recoveredRuntime.ReasoningEffort != "ultra" || recoveredRuntime.Speed != "fast" {
			t.Fatalf(
				"recovered runtime = %s/%s, want ultra/fast",
				recoveredRuntime.ReasoningEffort,
				recoveredRuntime.Speed,
			)
		}
		if len(promptRequests) != 2 || promptRequests[0].TurnID != promptRequests[1].TurnID {
			t.Fatalf("driver Prompt() requests = %#v, want one replay with the original turn id", promptRequests)
		}
		if promptRequests[0].RunID == "" || promptRequests[0].RunID != promptRequests[1].RunID ||
			promptRequests[0].RunID == promptRequests[0].TurnID ||
			promptRequests[0].Generation != 1 || promptRequests[1].Generation != 2 {
			t.Fatalf("driver Prompt() identities = %#v, want one run across generations 1 and 2", promptRequests)
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
		if started.TurnID != succeeded.TurnID || started.RunID != promptRequests[0].RunID ||
			succeeded.RunID != promptRequests[0].RunID || started.RunID == started.TurnID ||
			started.Generation != 2 || succeeded.Generation != 2 ||
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
		workspace, err := h.resolver.Resolve(t.Context(), h.workspaceID)
		if err != nil {
			t.Fatal(err)
		}
		workspace.Config.Session.Derive = compozyconfig.SessionDeriveConfig{MaxReplayBytes: 8192, MaxMessageBytes: 4096}
		h.resolver.upsert(&workspace)
		for index := range 20 {
			turnID := fmt.Sprintf("recovery-history-%d", index)
			for _, event := range []acp.AgentEvent{
				{Type: acp.EventTypeUserMessage, TurnID: turnID, Text: strings.Repeat("history user ", 512)},
				{Type: acp.EventTypeAgentMessage, TurnID: turnID, Text: strings.Repeat("history answer ", 512)},
				{Type: acp.EventTypeDone, TurnID: turnID},
			} {
				event.Timestamp = h.manager.now()
				if err := h.manager.recordEvent(t.Context(), session, event); err != nil {
					t.Fatal(err)
				}
			}
		}
		replay, _, err := h.manager.buildResumeReplay(t.Context(), session)
		if err != nil {
			t.Fatal(err)
		}
		h.manager.stageResumeReplay(session.ID, replay)

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
		requests := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
		h.driver.mu.Unlock()
		if startCalls != 4 {
			t.Fatalf("driver Start() calls = %d, want initial runtime plus three recoveries", startCalls)
		}
		if len(requests) != 4 {
			t.Fatalf("delivered requests=%d, want initial plus three recoveries", len(requests))
		}
		for index, request := range requests {
			if strings.Count(request.Message, resumeReplayOpenTag) != 1 ||
				strings.Count(request.Message, resumeReplayCloseTag) != 1 {
				t.Fatalf("dispatch %d does not contain exactly one historical array", index)
			}
			messages := resumeReplayMessagesFromPrompt(t, request.Message)
			if len(messages) == 0 || replayArrayBytes(messages) > 8192 {
				t.Fatalf(
					"dispatch %d history bytes=%d count=%d, want nonempty <=8192",
					index,
					replayArrayBytes(messages),
					len(messages),
				)
			}
			for _, message := range messages {
				if replayMessageBytes(message) > 4096 {
					t.Fatalf("dispatch %d message bytes=%d, want <=4096", index, replayMessageBytes(message))
				}
			}
			_, suffix, ok := strings.Cut(request.Message, resumeReplayCloseTag)
			if !ok || suffix != "\n\nUser request:\n\ncomplete a long task" {
				t.Fatalf("dispatch %d ordinary payload suffix=%q", index, suffix)
			}
			if request.TurnID != requests[0].TurnID || request.RunID != requests[0].RunID ||
				request.Generation != int64(index+1) || request.Maintenance {
				t.Fatalf("dispatch %d changed ordinary prompt identity: %+v", index, request)
			}
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

func TestPromptProviderAuthRecovery(t *testing.T) {
	t.Parallel()

	authLostEvent := func(proc *fakeProcess, turnID string) acp.AgentEvent {
		at := time.Now().UTC()
		return acp.AgentEvent{
			Type: acp.EventTypeError, SessionID: proc.handle.SessionID, TurnID: turnID, Timestamp: at,
			Error:   `{"code":-32000,"message":"Authentication required"}`,
			Failure: &store.SessionFailure{Kind: store.FailurePrompt, Summary: "Authentication required"},
			ProviderError: &acp.ProviderErrorDiagnostic{
				Code: acp.ProviderErrorAuthRequired, Provider: "claude", NextAction: acp.ProviderFailureActionLogin,
				Guidance: "run provider auth login for this provider", OccurrenceCount: 1, FirstSeenAt: at, LastSeenAt: at,
			},
		}
	}
	doneEvent := func(proc *fakeProcess, turnID string) acp.AgentEvent {
		return acp.AgentEvent{
			Type: acp.EventTypeDone, SessionID: proc.handle.SessionID, TurnID: turnID, Timestamp: time.Now().UTC(),
		}
	}
	promptRequests := func(h *harness) ([]acp.PromptRequest, int) {
		h.driver.mu.Lock()
		defer h.driver.mu.Unlock()
		return append([]acp.PromptRequest(nil), h.driver.promptCalls...), len(h.driver.startCalls)
	}
	// recordRecoveryReasons captures the runtime-recovery projection each replacement runtime starts under.
	recordRecoveryReasons := func(h *harness, session *Session) *[]store.SessionRuntimeRecoveryReason {
		var (
			mu      sync.Mutex
			reasons []store.SessionRuntimeRecoveryReason
		)
		h.driver.startHook = func(opts acp.StartOpts, attempt int) (*fakeProcess, error) {
			if recovery := session.Info().RuntimeRecovery; recovery != nil {
				mu.Lock()
				reasons = append(reasons, recovery.Reason)
				mu.Unlock()
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("recovered-%d", attempt)), nil
		}
		return &reasons
	}
	requireProviderAuthReasons := func(t *testing.T, reasons *[]store.SessionRuntimeRecoveryReason, want int) {
		t.Helper()
		if len(*reasons) != want {
			t.Fatalf("recovery starts = %v, want %d", *reasons, want)
		}
		for _, reason := range *reasons {
			if reason != store.SessionRuntimeRecoveryProviderAuth {
				t.Fatalf("recovery reasons = %v, want provider_auth", *reasons)
			}
		}
	}

	t.Run("Should resend an untouched turn once the provider sign-in returns", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = []time.Duration{0, 0, 0}
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		reasons := recordRecoveryReasons(h, session)
		var calls atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			if calls.Add(1) <= 2 {
				events <- authLostEvent(proc, req.TurnID)
			} else {
				events <- doneEvent(proc, req.TurnID)
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "ship the release notes")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		delivered := collectEvents(t, events)
		if got := countAgentEvents(delivered, acp.EventTypeError); got != 0 {
			t.Fatalf("delivered errors = %d, want the sign-in loss absorbed: %#v", got, delivered)
		}
		if countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryStarted) != 2 ||
			countAgentEvents(delivered, acp.EventTypeRuntimeRecoverySucceeded) != 2 {
			t.Fatalf("delivered = %#v, want two sign-in recoveries", delivered)
		}
		requireProviderAuthReasons(t, reasons, 2)
		if countAgentEvents(delivered, acp.EventTypeDone) != 1 {
			t.Fatalf("delivered = %#v, want the resumed turn to finish", delivered)
		}
		requests, starts := promptRequests(h)
		if len(requests) != 3 || starts != 3 {
			t.Fatalf("prompts = %d, starts = %d; want one turn reloaded on two replacement runtimes", len(requests), starts)
		}
		for index, request := range requests {
			if request.TurnID != requests[0].TurnID || !strings.HasSuffix(request.Message, "ship the release notes") {
				t.Fatalf("dispatch %d = %+v, want the original turn resent", index, request)
			}
		}
		if got := countEventType(readStoredEvents(t, session), acp.EventTypeUserMessage); got != 1 {
			t.Fatalf("stored user messages = %d, want 1", got)
		}
		info := session.Info()
		if info.State != StateActive || info.RuntimeStatus != RuntimeStatusReady || info.RuntimeRecovery != nil {
			t.Fatalf("session = %s/%s recovery %#v, want active and ready", info.State, info.RuntimeStatus, info.RuntimeRecovery)
		}
	})

	t.Run("Should continue instead of resending when the sign-in loss interrupts tool work", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = []time.Duration{0}
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		var calls atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			if calls.Add(1) == 1 {
				events <- acp.AgentEvent{
					Type: acp.EventTypeToolCall, SessionID: proc.handle.SessionID, TurnID: req.TurnID,
					Timestamp: time.Now().UTC(), ToolCallID: "tool-1", Title: "git push",
				}
				events <- authLostEvent(proc, req.TurnID)
			} else {
				events <- doneEvent(proc, req.TurnID)
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "push the branch")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		delivered := collectEvents(t, events)
		if countAgentEvents(delivered, acp.EventTypeError) != 0 || countAgentEvents(delivered, acp.EventTypeDone) != 1 {
			t.Fatalf("delivered = %#v, want the interrupted turn to finish", delivered)
		}
		requests, _ := promptRequests(h)
		if len(requests) != 2 {
			t.Fatalf("prompts = %d, want the original plus one continuation", len(requests))
		}
		resumed := requests[1]
		if resumed.TurnID != requests[0].TurnID || !strings.HasSuffix(resumed.Message, authRecoveryContinueMessage) ||
			strings.Contains(resumed.Message, "User request:\n\npush the branch") {
			t.Fatalf("continuation = %q, want the continuation instead of the original request", resumed.Message)
		}
		if got := countEventType(readStoredEvents(t, session), acp.EventTypeToolCall); got != 1 {
			t.Fatalf("stored tool calls = %d, want the interrupted call recorded once", got)
		}
	})

	t.Run("Should fail with the provider guidance once the sign-in window ends", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = []time.Duration{0, 0}
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		reasons := recordRecoveryReasons(h, session)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- authLostEvent(proc, req.TurnID)
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "keep going")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		delivered := collectEvents(t, events)
		if countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryStarted) != 2 ||
			countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryExhausted) != 1 {
			t.Fatalf("delivered = %#v, want two sign-in waits and one exhaustion", delivered)
		}
		requireProviderAuthReasons(t, reasons, 2)
		last := delivered[len(delivered)-1]
		if last.Type != acp.EventTypeError || last.ProviderError == nil ||
			last.ProviderError.Code != acp.ProviderErrorAuthRequired ||
			last.ProviderError.NextAction != acp.ProviderFailureActionHandoff {
			t.Fatalf("terminal event = %#v, want the provider sign-in failure with handoff guidance", last)
		}
		info := session.Info()
		if info.State != StateActive || info.RuntimeStatus != RuntimeStatusReady || info.RuntimeRecovery != nil {
			t.Fatalf("session = %s/%s recovery %#v, want active and ready", info.State, info.RuntimeStatus, info.RuntimeRecovery)
		}

		next, err := h.manager.Prompt(testutil.Context(t), session.ID, "and the next one")
		if err != nil {
			t.Fatalf("Prompt(next) error = %v", err)
		}
		followUp := collectEvents(t, next)
		if len(followUp) != 1 || followUp[0].Type != acp.EventTypeError {
			t.Fatalf("follow-up = %#v, want an immediate failure after the window was spent", followUp)
		}
	})

	t.Run("Should return the session to ready when the sign-in wait is cancelled", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = []time.Duration{time.Hour}
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- authLostEvent(proc, req.TurnID)
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "wait for me")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		waitForCondition(t, "sign-in wait", func() bool {
			recovery := session.Info().RuntimeRecovery
			return recovery != nil && recovery.Reason == store.SessionRuntimeRecoveryProviderAuth
		})
		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		collectEvents(t, events)
		info := session.Info()
		if info.State != StateActive || info.RuntimeStatus != RuntimeStatusReady || info.RuntimeRecovery != nil {
			t.Fatalf("session = %s/%s recovery %#v, want active and ready", info.State, info.RuntimeStatus, info.RuntimeRecovery)
		}
		if _, starts := promptRequests(h); starts != 1 {
			t.Fatalf("driver starts = %d, want no replacement runtime after cancellation", starts)
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
