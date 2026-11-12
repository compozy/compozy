package session

import (
	"bytes"
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
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/modelcatalog"
	authproviders "github.com/compozy/compozy/internal/providers"
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

	// Invariant: canceling runtime-exit backoff settles recovery without advertising a dead runtime as ready.
	// Owner: session manager; existing runtime recovery lifetime suite.
	t.Run("Should settle cancellation during runtime exit backoff", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.promptRecoveryDelays = []time.Duration{time.Hour}
		session := createSession(t, h)
		previous := session.processHandle()
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			proc.crash(errors.New("runtime dropped"), "runtime dropped")
			stream := make(chan acp.AgentEvent, 1)
			stream <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: req.TurnID, Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "runtime dropped"}}
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), session.ID, "original request")
		if err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "runtime-exit recovery", func() bool {
			r := session.Info().RuntimeRecovery
			return r != nil && r.Reason == store.SessionRuntimeRecoveryRuntimeExit
		})
		if _, err := h.manager.CancelPrompt(t.Context(), session.ID); err != nil {
			t.Fatal(err)
		}
		events := collectEvents(t, stream)
		if countAgentEvents(events, acp.EventTypeError) != 0 || countAgentEvents(events, acp.EventTypeDone) != 1 {
			t.Fatalf("cancel events=%#v", events)
		}
		atDelivery := session.Info()
		if atDelivery.RuntimeRecovery != nil || atDelivery.RuntimeStatus == RuntimeStatusReady ||
			atDelivery.RuntimeStatus == RuntimeStatusRecovering {
			t.Fatalf("canceled terminal delivered stale runtime state: %#v", atDelivery)
		}
		if err := h.manager.handleProcessExit(t.Context(), session, previous, previous.Wait()); err != nil {
			t.Fatal(err)
		}
		info := session.Info()
		if info.RuntimeRecovery != nil || info.RuntimeStatus != RuntimeStatusUnbound {
			t.Fatalf("canceled recovery runtime=%s recovery=%#v", info.RuntimeStatus, info.RuntimeRecovery)
		}
	})

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
				Code:            acp.ProviderErrorAuthRequired,
				Provider:        "claude",
				NextAction:      acp.ProviderFailureActionLogin,
				Guidance:        "run provider auth login for this provider",
				OccurrenceCount: 1,
				FirstSeenAt:     at,
				LastSeenAt:      at,
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

	// Invariant: runtime loss that discovers a signed-out provider gets the auth window and verified wake.
	// Owner: session manager; canonical provider-auth recovery suite.
	t.Run(
		"Should switch runtime exit recovery to sign in recovery and wake on verified credentials",
		func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.manager.promptRecoveryDelays = []time.Duration{0}
			h.manager.authRecoveryDelays = []time.Duration{time.Hour}
			h.manager.authRecoveryWindow = 2 * time.Hour
			session := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
			var signedIn atomic.Bool
			h.driver.startHook = func(opts acp.StartOpts, attempt int) (*fakeProcess, error) {
				if !signedIn.Load() {
					return nil, acp.WrapFailure(
						store.FailureProviderAuth,
						"provider sign-in required",
						errors.New("not logged in"),
					)
				}
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("signed-in-%d", attempt)), nil
			}
			var prompts atomic.Int64
			h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				stream := make(chan acp.AgentEvent, 1)
				if prompts.Add(1) == 1 {
					proc.crash(errors.New("runtime dropped"), "runtime dropped")
					stream <- acp.AgentEvent{Type: acp.EventTypeError, Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "runtime dropped"}}
				} else {
					stream <- doneEvent(proc, req.TurnID)
				}
				close(stream)
				return stream, nil
			}
			stream, err := h.manager.Prompt(t.Context(), session.ID, "original request")
			if err != nil {
				t.Fatal(err)
			}
			var delivered []acp.AgentEvent
			for event := range stream {
				delivered = append(delivered, event)
				if event.Type == acp.EventTypeRuntimeRecoveryStarted && infoProviderAuthRecovery(session.Info()) {
					signedIn.Store(true)
					info := session.Info()
					if err := h.manager.RecoverProviderAuthentication(
						t.Context(),
						info.ProfileID,
						info.Provider,
						authproviders.AuthenticationRouteFingerprint(
							info.Provider,
							providerConfigFromResolvedAgent(session.providerRoutingSnapshot()),
						),
					); err != nil {
						t.Fatal(err)
					}
				}
			}
			if countAgentEvents(delivered, acp.EventTypeError) != 0 ||
				countAgentEvents(delivered, acp.EventTypeDone) != 1 {
				t.Fatalf("events = %#v, want automatic completion after native sign in", delivered)
			}
			if !signedIn.Load() || BadgeForInfo(session.Info()) == BadgeWaitingForAuth {
				t.Fatal("verified sign in did not clear the wait")
			}
		},
	)

	t.Run("Should cancel after runtime exit switches to sign in recovery", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.promptRecoveryDelays = []time.Duration{0}
		h.manager.authRecoveryDelays = []time.Duration{time.Hour}
		h.manager.authRecoveryWindow = 2 * time.Hour
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
			return nil, acp.WrapFailure(
				store.FailureProviderAuth,
				"provider sign-in required",
				errors.New("not logged in"),
			)
		}
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			proc.crash(errors.New("runtime dropped"), "runtime dropped")
			stream := make(chan acp.AgentEvent, 1)
			stream <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: req.TurnID, Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "runtime dropped"}}
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), session.ID, "original request")
		if err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "native sign-in recovery", func() bool { return infoProviderAuthRecovery(session.Info()) })
		if _, err := h.manager.CancelPrompt(t.Context(), session.ID); err != nil {
			t.Fatal(err)
		}
		events := collectEvents(t, stream)
		if countAgentEvents(events, acp.EventTypeError) != 0 || countAgentEvents(events, acp.EventTypeDone) != 1 {
			t.Fatalf("cancel events = %#v", events)
		}
		for _, event := range events {
			if event.Type == acp.EventTypeDone && event.StopReason != string(acp.PromptStopReasonCancelled) {
				t.Fatalf("cancel stop reason = %q", event.StopReason)
			}
		}
	})

	t.Run("Should keep retrying the pinned model while the offline catalog recovers", func(t *testing.T) {
		t.Parallel()
		const pinnedModel = "claude-opus-5-5"
		h := newHarness(t, WithModelCatalog(modelCatalogStub{models: []modelcatalog.Model{
			{
				ProviderID:        "claude",
				ModelID:           pinnedModel,
				AvailabilityState: modelcatalog.AvailabilityStateAvailableLive,
				TransportBindings: []modelcatalog.ModelTransportBinding{{TransportModelID: pinnedModel}},
				Sources: []modelcatalog.SourceRef{
					{
						SourceID:   modelcatalog.SourceKindProviderLiveID("claude"),
						SourceKind: modelcatalog.SourceKindProviderLive,
					},
				},
			},
		}}))
		h.manager.authRecoveryDelays = []time.Duration{0, 0}
		session, err := h.manager.Create(
			t.Context(),
			new(CreateOpts{
				AgentName: "coder", Name: "session", Workspace: h.workspaceID,
				Provider: "claude", Model: pinnedModel,
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		var prompts, starts atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			stream := make(chan acp.AgentEvent, 1)
			if prompts.Add(1) == 1 {
				stream <- authLostEvent(proc, req.TurnID)
			} else {
				stream <- doneEvent(proc, req.TurnID)
			}
			close(stream)
			return stream, nil
		}
		h.driver.startHook = func(opts acp.StartOpts, attempt int) (*fakeProcess, error) {
			if opts.PreferredModel != pinnedModel {
				t.Errorf("recovery model = %q, want exact pinned model %q", opts.PreferredModel, pinnedModel)
			}
			if starts.Add(1) == 1 {
				return nil, acp.WrapFailure(
					store.FailureProtocol,
					"model negotiation failed",
					acp.ValidateModelConfigValue([]acp.SessionConfigOption{
						{
							ID:       "model",
							Category: "model",
							Kind:     acp.SessionConfigOptionKindSelect,
							Values:   []acp.SessionConfigOptionValue{{Value: "default"}, {Value: "opus"}},
						},
					}, pinnedModel),
				)
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("recovered-%d", attempt)), nil
		}
		stream, err := h.manager.Prompt(t.Context(), session.ID, "original request")
		if err != nil {
			t.Fatal(err)
		}
		events := collectEvents(t, stream)
		if session.Info().Model != pinnedModel {
			t.Fatalf("completed model = %q", session.Info().Model)
		}
		if starts.Load() != 2 || countAgentEvents(events, acp.EventTypeError) != 0 ||
			countAgentEvents(events, acp.EventTypeDone) != 1 {
			t.Fatalf("recovery starts=%d events=%#v", starts.Load(), events)
		}
	})

	t.Run("Should retain login guidance when runtime exit exhausts the sign in window", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.promptRecoveryDelays = []time.Duration{0}
		h.manager.authRecoveryDelays = []time.Duration{0}
		session := createSession(t, h)
		previous := session.processHandle()
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
			return nil, acp.WrapFailure(
				store.FailureProviderAuth,
				"provider sign-in required",
				errors.New("not logged in"),
			)
		}
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			proc.crash(errors.New("runtime dropped"), "runtime dropped")
			stream := make(chan acp.AgentEvent, 1)
			stream <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: req.TurnID, Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "runtime dropped"}}
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), session.ID, "original request")
		if err != nil {
			t.Fatal(err)
		}
		events := collectEvents(t, stream)
		if countAgentEvents(events, acp.EventTypeError) != 1 {
			t.Fatalf("events = %#v", events)
		}
		for _, event := range events {
			if event.Type == acp.EventTypeError &&
				(event.ProviderError == nil || event.ProviderError.NextAction != acp.ProviderFailureActionLogin) {
				t.Fatalf("terminal error = %#v, want login", event)
			}
		}
		if session.processHandle() != nil {
			t.Fatal("exhausted sign-in wait retained the dead process binding")
		}
		if err := h.manager.handleProcessExit(t.Context(), session, previous, previous.Wait()); err != nil {
			t.Fatal(err)
		}
		if got := BadgeForInfo(session.Info()); got != BadgeWaitingForAuth {
			t.Fatalf("badge = %s, want waiting for sign in", got)
		}
	})

	for _, tc := range []struct {
		name        string
		runtimeExit bool
	}{
		{name: "Should persist a genuine recovery rejection from a live runtime"},
		{name: "Should persist a genuine recovery rejection after runtime exit and sign in", runtimeExit: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.manager.authRecoveryDelays = []time.Duration{0, 0}
			h.manager.promptRecoveryDelays = []time.Duration{0}
			var starts atomic.Int64
			session := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
			h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				stream := make(chan acp.AgentEvent, 1)
				if tc.runtimeExit {
					proc.crash(errors.New("runtime dropped"), "runtime dropped")
					stream <- acp.AgentEvent{Type: acp.EventTypeError, TurnID: req.TurnID, Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "runtime dropped"}}
				} else {
					stream <- authLostEvent(proc, req.TurnID)
				}
				close(stream)
				return stream, nil
			}
			h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
				if starts.Add(1) == 1 && tc.runtimeExit {
					return nil, acp.WrapFailure(
						store.FailureProviderAuth,
						"provider sign-in required",
						errors.New("not logged in"),
					)
				}
				return nil, acp.WrapFailure(
					store.FailureProtocol,
					"model negotiation failed",
					acp.ValidateModelConfigValue([]acp.SessionConfigOption{
						{
							ID:       "model",
							Category: "model",
							Kind:     acp.SessionConfigOptionKindSelect,
							Values:   []acp.SessionConfigOptionValue{{Value: "another-model"}},
						},
					}, "claude-opus-5-5"),
				)
			}
			stream, err := h.manager.Prompt(t.Context(), session.ID, "needs recovery")
			if err != nil {
				t.Fatal(err)
			}
			events := collectEvents(t, stream)
			wantAttempts := len(h.manager.authRecoveryDelays)
			if tc.runtimeExit {
				wantAttempts++
			}
			if countAgentEvents(events, acp.EventTypeError) != 1 ||
				countAgentEvents(events, acp.EventTypeRuntimeRecoveryStarted) != wantAttempts {
				t.Fatalf("events = %#v", events)
			}
			for _, event := range events {
				if event.Type == acp.EventTypeError &&
					(event.Failure == nil || event.Failure.Kind != store.FailureProtocol || event.ProviderError != nil && event.ProviderError.Code == acp.ProviderErrorAuthRequired) {
					t.Fatalf("terminal error = %#v", event)
				}
			}
			info := session.Info()
			if BadgeForInfo(info) != BadgeFailed || info.RuntimeRecovery != nil || info.Failure == nil ||
				info.Failure.Kind != store.FailureProtocol {
				t.Fatalf("rejection must retain truthful failed state: %#v", info)
			}
		})
	}

	t.Run("Should rebuild a stale authenticated process on the next prompt after exhaustion", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = []time.Duration{0}
		session := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		var credentialsValid atomic.Bool
		var stale *fakeProcess
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			stream := make(chan acp.AgentEvent, 1)
			if !credentialsValid.Load() || proc == stale {
				stale = proc
				stream <- authLostEvent(proc, req.TurnID)
			} else {
				stream <- doneEvent(proc, req.TurnID)
			}
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), session.ID, "original request")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, stream)
		if got := BadgeForInfo(session.Info()); got != BadgeWaitingForAuth {
			t.Errorf("badge = %s, want waiting-for-auth", got)
		}
		credentialsValid.Store(true)
		stream, err = h.manager.Prompt(t.Context(), session.ID, "retry after signing in")
		if err != nil {
			t.Fatal(err)
		}
		delivered := collectEvents(t, stream)
		if countAgentEvents(delivered, acp.EventTypeError) != 0 || countAgentEvents(delivered, acp.EventTypeDone) != 1 {
			t.Fatalf("events = %#v, want a completed turn on a fresh provider", delivered)
		}
		if got := BadgeForInfo(session.Info()); got == BadgeWaitingForAuth {
			t.Fatal("successful retry retained auth attention")
		}
	})

	t.Run("Should recover only auth failed sessions on the verified profile provider and account", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = nil
		target := createSession(t, h)
		healthy := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, target.ID); reportSessionStop(t, h, healthy.ID) })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			stream := make(chan acp.AgentEvent, 1)
			stream <- authLostEvent(proc, req.TurnID)
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), target.ID, "needs sign in")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, stream)
		before := target.runtimeBindingSnapshot()
		info := target.Info()
		for _, wrong := range []struct{ profile, provider, command string }{
			{"another-profile", info.Provider, target.providerRoutingSnapshot().Command},
			{info.ProfileID, "another-provider", target.providerRoutingSnapshot().Command},
			{info.ProfileID, info.Provider, "another-account-cli"},
		} {
			cfg := providerConfigFromResolvedAgent(target.providerRoutingSnapshot())
			cfg.Command = wrong.command
			if err := h.manager.RecoverProviderAuthentication(
				t.Context(),
				wrong.profile,
				wrong.provider,
				authproviders.AuthenticationRouteFingerprint(info.Provider, cfg),
			); err != nil {
				t.Fatal(err)
			}
			if target.processHandle() != before.process {
				t.Fatal("unrelated authentication proof replaced the runtime")
			}
		}
		// A provider-level proof cannot authorize a workspace's different launch isolation.
		route := target.providerRoutingSnapshot()
		isolatedHome, isolatedEnv := route, route
		isolatedHome.HomePolicy = compozyconfig.ProviderHomePolicyIsolated
		isolatedEnv.EnvPolicy = compozyconfig.ProviderEnvPolicyIsolated
		for _, isolated := range []compozyconfig.ResolvedAgent{isolatedHome, isolatedEnv} {
			target.setProviderRouting(isolated)
			if err := h.manager.RecoverProviderAuthentication(
				t.Context(),
				info.ProfileID,
				info.Provider,
				authproviders.AuthenticationRouteFingerprint(
					info.Provider,
					providerConfigFromResolvedAgent(route),
				),
			); err != nil {
				t.Fatal(err)
			}
			if target.processHandle() != before.process {
				t.Fatal("authentication proof crossed the session isolation policy")
			}
		}
		target.setProviderRouting(route)
		// Model a provider that needs six seconds of its normal startup budget, without sleeping.
		h.driver.startContextHook = func(ctx context.Context, opts acp.StartOpts, _ int) (*fakeProcess, error) {
			if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 6*time.Second {
				return nil, context.DeadlineExceeded
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "normal-startup-budget"), nil
		}
		verifiedConfig := providerConfigFromResolvedAgent(route)
		verifiedConfig.RuntimeProvider = "" // configured native defaults become explicit in the launch route.
		healthyProcess := healthy.processHandle()
		if !h.manager.ProviderAuthenticationReady(
			info.ProfileID,
			info.Provider,
			authproviders.AuthenticationRouteFingerprint(info.Provider, verifiedConfig),
		) {
			t.Fatal("recovery admission refused")
		}
		waitForCondition(t, "verified auth recovery", func() bool {
			return target.processHandle() != nil && target.Info().RuntimeGeneration > before.generation &&
				target.Info().Failure == nil
		})
		if target.processHandle() == before.process || BadgeForInfo(target.Info()) == BadgeWaitingForAuth {
			t.Fatal("valid authentication proof did not rebuild the failed session")
		}
		if healthy.processHandle() != healthyProcess {
			t.Fatal("authentication recovery replaced a healthy session")
		}
	})

	t.Run("Should wake an active sign in wait when credentials are verified", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		h.manager.authRecoveryDelays = []time.Duration{time.Hour}
		target := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
		var calls atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			stream := make(chan acp.AgentEvent, 1)
			if calls.Add(1) == 1 {
				stream <- authLostEvent(proc, req.TurnID)
			} else {
				stream <- doneEvent(proc, req.TurnID)
			}
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), target.ID, "resume when signed in")
		if err != nil {
			t.Fatal(err)
		}
		waitForCondition(t, "auth recovery", func() bool { return target.Info().RuntimeRecovery != nil })
		info := target.Info()
		if got := BadgeForInfo(info); got != BadgeWaitingForAuth {
			t.Fatalf("badge = %s, want waiting-for-auth", got)
		}
		if !h.manager.ProviderAuthenticationReady(
			info.ProfileID,
			info.Provider,
			authproviders.AuthenticationRouteFingerprint(
				info.Provider, providerConfigFromResolvedAgent(target.providerRoutingSnapshot()),
			),
		) {
			t.Fatal("recovery admission refused")
		}
		delivered := collectEvents(t, stream)
		if countAgentEvents(delivered, acp.EventTypeDone) != 1 || countAgentEvents(delivered, acp.EventTypeError) != 0 {
			t.Fatalf("events = %#v, want immediate recovery on verified sign-in", delivered)
		}
	})

	t.Run("Should dispatch a prompt queued during a background authentication reload", func(t *testing.T) {
		t.Parallel()
		db := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(db))
		h.manager.authRecoveryDelays = nil
		registerManagerInputQueueWorkspace(t, db, h)
		target := createSession(t, h)
		registerManagerInputQueueSession(t, db, h, target)
		t.Cleanup(func() { reportSessionStop(t, h, target.ID) })
		var calls atomic.Int64
		dispatched := make(chan struct{}, 1)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			stream := make(chan acp.AgentEvent, 1)
			if calls.Add(1) == 1 {
				stream <- authLostEvent(proc, req.TurnID)
			} else {
				dispatched <- struct{}{}
				stream <- doneEvent(proc, req.TurnID)
			}
			close(stream)
			return stream, nil
		}
		stream, err := h.manager.Prompt(t.Context(), target.ID, "needs login")
		if err != nil {
			t.Fatal(err)
		}
		collectEvents(t, stream)
		entered, release := make(chan struct{}), make(chan struct{})
		releaseOnce := sync.OnceFunc(func() { close(release) })
		t.Cleanup(releaseOnce)
		h.driver.startContextHook = func(ctx context.Context, opts acp.StartOpts, _ int) (*fakeProcess, error) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "reauth-queued"), nil
		}
		info := target.Info()
		if !h.manager.ProviderAuthenticationReady(
			info.ProfileID,
			info.Provider,
			authproviders.AuthenticationRouteFingerprint(
				info.Provider, providerConfigFromResolvedAgent(target.providerRoutingSnapshot()),
			),
		) {
			t.Fatal("recovery admission refused")
		}
		select {
		case <-entered:
		case <-t.Context().Done():
			t.Fatal("reload did not start")
		}
		result, err := h.manager.SendPrompt(
			t.Context(),
			target.ID,
			SendPromptOpts{
				Message:        "continue after login",
				Mode:           BusyInputModeQueue,
				MessageID:      "queued-auth",
				IdempotencyKey: "queued-auth",
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if result.QueueEntryID == "" {
			t.Fatalf("result = %#v, want queued input", result)
		}
		releaseOnce()
		select {
		case <-dispatched:
		case <-time.After(3 * time.Second):
			t.Fatal("queued prompt was not dispatched after authentication reload")
		}
	})

	for _, action := range []acp.ProviderFailureAction{acp.ProviderFailureActionBindSecret, acp.ProviderFailureActionInspect} {
		t.Run("Should show actionable provider guidance for "+string(action), func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			session := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
			h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				event := authLostEvent(proc, req.TurnID)
				event.ProviderError.NextAction = action
				events := make(chan acp.AgentEvent, 1)
				events <- event
				close(events)
				return events, nil
			}
			events, err := h.manager.Prompt(t.Context(), session.ID, "check provider access")
			if err != nil {
				t.Fatal(err)
			}
			delivered := collectEvents(t, events)
			if countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryStarted) != 0 || len(delivered) != 1 ||
				delivered[0].ProviderError == nil {
				t.Fatalf("events = %#v, want an immediate actionable provider notice", delivered)
			}
			_, starts := promptRequests(h)
			if starts != 1 {
				t.Fatalf("starts = %d, want no automatic configuration retry", starts)
			}
		})
	}
	for _, canceled := range []bool{false, true} {
		name := "Should rearm auth recovery after a completed turn"
		if canceled {
			name = "Should preserve auth recovery exhaustion after cancellation"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			session := &Session{}
			session.setAuthRecoveryExhausted(true)
			event := acp.AgentEvent{Type: acp.EventTypeDone}
			if canceled {
				event.StopReason = string(acp.PromptStopReasonCancelled)
			}
			notePromptEventDelivered(session, &promptTurnDispatchState{recovery: &promptRecoveryState{}}, event)
			if session.authRecoveryAvailable() == canceled {
				t.Fatalf(
					"auth recovery available = %v after cancellation=%v",
					session.authRecoveryAvailable(),
					canceled,
				)
			}
		})
	}
	t.Run("Should give a changed provider its own auth recovery window", func(t *testing.T) {
		t.Parallel()
		session := &Session{Provider: "claude"}
		session.setAuthRecoveryExhausted(true)
		at := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
		session.completeRuntimeTransition(
			nil,
			RuntimeSelection{Provider: "claude", Model: "another"},
			RuntimeTransitionInitialBind,
			at,
		)
		if session.authRecoveryAvailable() {
			t.Fatal("model change rearmed exhausted auth recovery")
		}
		session.completeRuntimeTransition(nil, RuntimeSelection{Provider: "codex"}, RuntimeTransitionInitialBind, at)
		if !session.authRecoveryAvailable() {
			t.Fatal("new provider inherited exhausted auth recovery")
		}
	})
	t.Run("Should retain the configured auth window in the process exit wait budget", func(t *testing.T) {
		t.Parallel()
		manager := &Manager{
			authRecoveryWindow:   10 * time.Minute,
			authRecoveryDelays:   []time.Duration{15 * time.Second},
			promptRecoveryDelays: []time.Duration{time.Second},
		}
		if got, want := manager.maxPromptRecoveryWait(), 10*time.Minute+time.Second+defaultLifecycleTimeout; got != want {
			t.Fatalf("process exit recovery budget = %s, want %s", got, want)
		}
	})

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
			t.Fatalf(
				"prompts = %d, starts = %d; want one turn reloaded on two replacement runtimes",
				len(requests),
				starts,
			)
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
			t.Fatalf(
				"session = %s/%s recovery %#v, want active and ready",
				info.State,
				info.RuntimeStatus,
				info.RuntimeRecovery,
			)
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

	for _, startup := range []bool{false, true} {
		name := "Should count delayed auth refusals against the recovery window"
		if startup {
			name = "Should count provider startup against the recovery window"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base := time.Now().UTC()
			var elapsed atomic.Int64
			h := newHarness(t, WithNow(func() time.Time { return base.Add(time.Duration(elapsed.Load())) }))
			h.manager.authRecoveryDelays = []time.Duration{0, 0, 0}
			h.manager.authRecoveryWindow = time.Minute
			session := createSession(t, h)
			t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
			h.driver.startContextHook = func(ctx context.Context, opts acp.StartOpts, attempt int) (*fakeProcess, error) {
				if startup {
					deadline, ok := ctx.Deadline()
					if !ok || time.Until(deadline) > time.Minute {
						return nil, errors.New("recovery startup has no bounded deadline")
					}
					elapsed.Store(int64(2 * time.Minute))
					return nil, context.DeadlineExceeded
				}
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("recovered-%d", attempt)), nil
			}
			var calls atomic.Int64
			h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				if calls.Add(1) > 1 {
					elapsed.Store(int64(2 * time.Minute))
				}
				events := make(chan acp.AgentEvent, 1)
				events <- authLostEvent(proc, req.TurnID)
				close(events)
				return events, nil
			}
			events, err := h.manager.Prompt(t.Context(), session.ID, "wait for sign-in")
			if err != nil {
				t.Fatal(err)
			}
			delivered := collectEvents(t, events)
			if countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryStarted) != 1 ||
				countAgentEvents(delivered, acp.EventTypeRuntimeRecoveryExhausted) != 1 ||
				countAgentEvents(delivered, acp.EventTypeError) != 1 {
				t.Fatalf("events = %#v, want one attempt and one terminal auth failure", delivered)
			}
			_, starts := promptRequests(h)
			if starts != 2 {
				t.Fatalf("starts = %d, want no new runtime after expiry", starts)
			}
		})
	}

	t.Run("Should retain image and PDF bytes when auth recovery starts a fresh session", func(t *testing.T) {
		t.Parallel()
		imageID := "att_" + strings.Repeat("a", 64)
		pdfID := "att_" + strings.Repeat("b", 64)
		image, pdf := []byte("image contents"), []byte("PDF contents")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{imageID: image, pdfID: pdf},
			refs: map[string]attachmentspkg.AttachmentRef{
				imageID: storedPromptAttachmentRef(t, imageID, "photo.png", "image/png", image),
				pdfID:   storedPromptAttachmentRef(t, pdfID, "report.pdf", "application/pdf", pdf),
			},
		}
		admissions := openManagerInputQueueStore(t)
		h := newHarness(t, WithAttachmentOpener(opener), WithSessionPromptAdmissionStore(admissions))
		h.manager.authRecoveryDelays = []time.Duration{0}
		registerManagerInputQueueWorkspace(t, admissions, h)
		h.driver.startHook = func(opts acp.StartOpts, attempt int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, acp.ErrAgentDoesNotSupportSession
			}
			proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("fresh-%d", attempt))
			proc.handle.caps.PromptImage = true
			proc.handle.caps.PromptEmbeddedContext = true
			return proc, nil
		}
		session := createSession(t, h)
		registerManagerInputQueueSession(t, admissions, h, session)
		t.Cleanup(func() { reportSessionStop(t, h, session.ID) })
		var calls atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			if calls.Add(1) == 1 {
				events <- acp.AgentEvent{Type: acp.EventTypeThought, TurnID: req.TurnID, Timestamp: h.manager.now(), Text: "Reading files"}
				events <- authLostEvent(proc, req.TurnID)
			} else {
				events <- doneEvent(proc, req.TurnID)
			}
			close(events)
			return events, nil
		}
		result, err := h.manager.SendPrompt(t.Context(), session.ID, SendPromptOpts{
			MessageID: "files-recovery", IdempotencyKey: "files-recovery",
			Message: "compare the attached files",
			Attachments: []AttachmentMeta{
				promptAttachmentMeta(t, imageID, "photo.png", "image/png", image),
				promptAttachmentMeta(t, pdfID, "report.pdf", "application/pdf", pdf),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		delivered := collectEvents(t, result.Events)
		requests, starts := promptRequests(h)
		if countAgentEvents(delivered, acp.EventTypeDone) != 1 || len(requests) != 2 || starts != 3 {
			t.Fatalf(
				"done/requests/starts = %d/%d/%d, want 1/2/3",
				countAgentEvents(delivered, acp.EventTypeDone),
				len(requests),
				starts,
			)
		}
		continued := requests[1]
		if !strings.HasSuffix(continued.Message, authRecoveryContinueMessage) || len(continued.Attachments) != 2 {
			t.Fatalf("continued request = %+v, want continuation and both files", continued)
		}
		if !bytes.Equal(continued.Attachments[0].Data, image) || !bytes.Equal(continued.Attachments[1].Data, pdf) {
			t.Fatal("fresh-session continuation lost attachment bytes")
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
			last.ProviderError.NextAction != acp.ProviderFailureActionLogin {
			t.Fatalf("terminal event = %#v, want the provider sign-in failure with login guidance", last)
		}
		info := session.Info()
		if info.State != StateActive || info.RuntimeStatus != RuntimeStatusReady || info.RuntimeRecovery != nil {
			t.Fatalf(
				"session = %s/%s recovery %#v, want active and ready",
				info.State,
				info.RuntimeStatus,
				info.RuntimeRecovery,
			)
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

	t.Run("Should return the session to ready when the sign-in wait is canceled", func(t *testing.T) {
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
			t.Fatalf(
				"session = %s/%s recovery %#v, want active and ready",
				info.State,
				info.RuntimeStatus,
				info.RuntimeRecovery,
			)
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
