package session

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session/sessionid"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/testutil/otlptest"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

const traceRequestRoute = "/api/workspaces/:workspace_id/sessions/:session_id/prompt"

// Not parallel: every case installs the global OTel providers.
func TestPromptTurnTrace(t *testing.T) {
	t.Run(
		"Should nest the turn under the request span and hand its context to runtime start and prompt",
		func(t *testing.T) {
			collector := exportTurnTraces(t)
			h := newHarness(t)
			driver := &promptContextCapturingDriver{fakeDriver: h.driver}
			h.manager = newManagerWithHarness(t, h, WithDriver(driver))
			session := createTraceSession(t, h)
			session.markRuntimeUnbound(time.Now().UTC())
			startContexts := make(chan context.Context, 1)
			h.driver.mu.Lock()
			h.driver.startContextHook = func(ctx context.Context, opts acp.StartOpts, sequence int) (*fakeProcess, error) {
				startContexts <- ctx
				acpSessionID := opts.ResumeSessionID
				if acpSessionID == "" {
					acpSessionID = fmt.Sprintf("acp-traced-%d", sequence)
				}
				return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, acpSessionID), nil
			}
			h.driver.mu.Unlock()

			ctx, request := startRequestSpan(t)
			events, err := h.manager.Prompt(ctx, session.ID, "hello")
			if err != nil {
				t.Fatalf("Prompt() error = %v", err)
			}
			collectEvents(t, events)
			drainedAt := time.Now()
			request.End()

			turn := onlyTurnSpan(t, collector.Flush(t), session.ID)
			if turn.TraceID != request.SpanContext().TraceID() || turn.ParentSpanID != request.SpanContext().SpanID() {
				t.Fatalf("session.turn trace %s parent %s, want a child of the request span %s",
					turn.TraceID, turn.ParentSpanID, request.SpanContext().SpanID())
			}
			if turn.End.After(drainedAt) {
				t.Fatalf("session.turn ended at %s, after the prompt stream drained at %s", turn.End, drainedAt)
			}
			assertTurnAttributes(t, turn, session, telemetry.OutcomeOK)
			if turn.StatusCode != tracepb.Status_STATUS_CODE_UNSET {
				t.Fatalf("session.turn status = %s, want unset", turn.StatusCode)
			}
			// The persistence pump's writes count toward the turn and close with its span.
			flushes, ok := turn.Attributes[string(telemetry.AttrDBFlushes)].(int64)
			if _, totalOK := turn.Attributes[string(telemetry.AttrDBWriteMSTotal)].(int64); !ok || !totalOK ||
				flushes < 1 {
				t.Fatalf("session.turn write counters = %v, want the pump's flushes and their total", turn.Attributes)
			}
			for name, providerCtx := range map[string]context.Context{
				"runtime start": receiveContext(t, startContexts),
				"prompt":        driver.lastPromptContext(t),
			} {
				if got := trace.SpanContextFromContext(providerCtx).SpanID(); got != turn.SpanID {
					t.Fatalf("%s context span = %s, want session.turn %s", name, got, turn.SpanID)
				}
			}
		},
	)

	// UT-030
	t.Run("Should mark a turn that ends with a provider error with an empty error status", func(t *testing.T) {
		collector := exportTurnTraces(t)
		h := newHarness(t)
		session := createTraceSession(t, h)
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- acp.AgentEvent{
				Type: acp.EventTypeError, SessionID: proc.handle.SessionID, TurnID: req.TurnID,
				Timestamp: time.Now().UTC(), Error: "open /home/ana/secret.txt: upstream refused",
				Failure: &store.SessionFailure{Kind: store.FailurePrompt, Summary: "upstream refused"},
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)

		turn := onlyTurnSpan(t, collector.Flush(t), session.ID)
		if turn.StatusCode != tracepb.Status_STATUS_CODE_ERROR || turn.StatusMessage != "" {
			t.Fatalf(
				"session.turn status = %s %q, want error with an empty description",
				turn.StatusCode,
				turn.StatusMessage,
			)
		}
		if got := turn.Attributes[string(telemetry.AttrErrorType)]; got != telemetry.ErrorProvider {
			t.Fatalf("session.turn error.type = %v, want %s", got, telemetry.ErrorProvider)
		}
		assertTurnAttributes(t, turn, session, telemetry.OutcomeError)
		duration := collector.FlushMetrics(t)[telemetry.MetricSessionTurnDuration].Points
		if len(duration) != 1 || duration[0].Count != 1 ||
			duration[0].Attributes[string(telemetry.AttrOutcome)] != telemetry.OutcomeError ||
			duration[0].Attributes[string(telemetry.AttrAgent)] != session.AgentName {
			t.Fatalf("%s points = %+v, want one error turn of %s", telemetry.MetricSessionTurnDuration, duration,
				session.AgentName)
		}
	})

	// IT-004
	t.Run("Should end a canceled turn with the canceled outcome before the stream drains", func(t *testing.T) {
		collector := exportTurnTraces(t)
		h := newHarness(t)
		session := createTraceSession(t, h)
		source := make(chan acp.AgentEvent, 1)
		var turnID atomic.Value
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID.Store(req.TurnID)
			return source, nil
		}

		ctx, request := startRequestSpan(t)
		events, err := h.manager.Prompt(ctx, session.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		source <- acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, SessionID: session.Info().ACPSessionID,
			TurnID: turnID.Load().(string), Timestamp: time.Now().UTC(), Text: "partial",
		}
		h.notifier.waitForAgentEvent(t, session.ID, acp.EventTypeAgentMessage)
		if _, err := h.manager.CancelPrompt(testutil.Context(t), session.ID); err != nil {
			t.Fatalf("CancelPrompt() error = %v", err)
		}
		close(source)
		collectEvents(t, events)
		request.End()

		spans := collector.Flush(t)
		turn := onlyTurnSpan(t, spans, session.ID)
		assertTurnAttributes(t, turn, session, telemetry.OutcomeCancelled)
		if turn.StatusCode != tracepb.Status_STATUS_CODE_UNSET {
			t.Fatalf("canceled session.turn status = %s, want unset", turn.StatusCode)
		}
		if got := len(traceSpans(spans, request.SpanContext().TraceID())); got != 2 {
			t.Fatalf("ended spans in the request trace = %d, want the request and its turn", got)
		}
	})

	// IT-005
	for name, queuedOpts := range map[string]SendPromptOpts{
		"Should start a queued turn as a root linked to the request that queued it": {
			Message: "second", Mode: BusyInputModeQueue,
		},
		"Should start an admitted queued turn as a root linked to the request that queued it": {
			Message: "second", Mode: BusyInputModeQueue, MessageID: "msg-2", IdempotencyKey: "key-2",
		},
	} {
		t.Run(name, func(t *testing.T) {
			collector := exportTurnTraces(t)
			h, session, release, prompts := busyTraceSession(t)
			busy, err := h.manager.Prompt(testutil.Context(t), session.ID, "first")
			if err != nil {
				t.Fatalf("Prompt(first) error = %v", err)
			}

			ctx, request := startRequestSpan(t)
			queued, err := h.manager.SendPrompt(ctx, session.ID, queuedOpts)
			request.End()
			if err != nil || queued.Status != store.SessionPromptResultStatusQueued {
				t.Fatalf("SendPrompt(second) = %+v, %v; want queued", queued, err)
			}
			close(release)
			collectEvents(t, busy)
			waitForPrompts(t, prompts, 2)
			if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
				t.Fatalf("WaitForPromptDrains() error = %v", err)
			}

			turns := turnSpans(collector.Flush(t), session.ID)
			if len(turns) != 2 {
				t.Fatalf("session.turn spans = %d, want the busy turn and the queued turn", len(turns))
			}
			var queuedTurn otlptest.Span
			for _, turn := range turns {
				if len(turn.Links) > 0 {
					queuedTurn = turn
				}
			}
			if queuedTurn.ParentSpanID.IsValid() || queuedTurn.TraceID == request.SpanContext().TraceID() {
				t.Fatalf(
					"queued session.turn parent %s trace %s, want a new root",
					queuedTurn.ParentSpanID,
					queuedTurn.TraceID,
				)
			}
			links := queuedTurn.Links
			if len(links) != 1 || links[0].TraceID() != request.SpanContext().TraceID() ||
				links[0].SpanID() != request.SpanContext().SpanID() {
				t.Fatalf("queued session.turn links = %+v, want the enqueuing request span", links)
			}
		})
	}

	t.Run("Should keep a queued input's link until a turn accepts the input", func(t *testing.T) {
		exportTurnTraces(t)
		ctx, request := startRequestSpan(t)
		defer request.End()
		var links queuedTurnLinks
		links.remember(ctx, "input-head")
		links.remember(ctx, "input-batched")
		entry := queuedInput{id: "input-head", batch: []queuedInput{{id: "input-batched"}}}

		// A failed submission reads the links and returns the input to the queue.
		if got := links.linksFor(entry); len(got) != 2 {
			t.Fatalf("linksFor() before a failed submission = %d links, want 2", len(got))
		}
		if got := links.linksFor(entry); len(got) != 2 {
			t.Fatalf("linksFor() on the retry = %d links, want the 2 links kept after the failed submission", len(got))
		}
		links.forget(entry)
		if got := links.linksFor(entry); len(got) != 0 {
			t.Fatalf("linksFor() after forget = %d links, want none", len(got))
		}
	})

	// IT-017
	t.Run("Should end the turn normally when the client stops reading the stream", func(t *testing.T) {
		collector := exportTurnTraces(t)
		h := newHarness(t)
		session := createTraceSession(t, h)
		source := make(chan acp.AgentEvent, 2)
		var turnID atomic.Value
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			turnID.Store(req.TurnID)
			return source, nil
		}
		deliveryCtx, disconnect := context.WithCancel(testutil.Context(t))
		result, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
			Message: "hello", DeliveryContext: deliveryCtx,
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}

		disconnect()
		source <- acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, SessionID: session.Info().ACPSessionID,
			TurnID: turnID.Load().(string), Timestamp: time.Now().UTC(), Text: "still running",
		}
		source <- acp.AgentEvent{
			Type: acp.EventTypeDone, SessionID: session.Info().ACPSessionID, TurnID: turnID.Load().(string),
			Timestamp: time.Now().UTC(), PromptStopReason: acp.PromptStopReasonEndTurn,
		}
		close(source)
		if err := h.manager.WaitForPromptDrains(testutil.Context(t)); err != nil {
			t.Fatalf("WaitForPromptDrains() error = %v", err)
		}
		collectEvents(t, result.Events)

		assertTurnAttributes(t, onlyTurnSpan(t, collector.Flush(t), session.ID), session, telemetry.OutcomeOK)
	})

	// UT-037, IT-018 (session part)
	t.Run("Should end a turn whose runtime cannot start once as setup_failed", func(t *testing.T) {
		collector := exportTurnTraces(t)
		h := newHarness(t)
		session := createTraceSession(t, h)
		session.markRuntimeUnbound(time.Now().UTC())
		h.driver.mu.Lock()
		h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) {
			return nil, errors.New("exec: \"/home/ana/bin/agent\": executable file not found in $PATH")
		}
		h.driver.mu.Unlock()

		ctx, request := startRequestSpan(t)
		if _, err := h.manager.Prompt(ctx, session.ID, "hello"); err == nil {
			t.Fatal("Prompt() error = nil, want the runtime start failure")
		}
		request.End()

		spans := traceSpans(collector.Flush(t), request.SpanContext().TraceID())
		if len(spans) != 2 {
			t.Fatalf("spans in the request trace = %d, want the request and its turn only", len(spans))
		}
		turn := onlyTurnSpan(t, spans, session.ID)
		assertTurnAttributes(t, turn, session, telemetry.OutcomeSetupFailed)
		if turn.StatusCode != tracepb.Status_STATUS_CODE_ERROR ||
			turn.Attributes[string(telemetry.AttrErrorType)] == nil {
			t.Fatalf(
				"session.turn status %s attributes %v, want an error with error.type",
				turn.StatusCode,
				turn.Attributes,
			)
		}
	})

	// UT-037
	t.Run("Should ignore a second end of the turn span", func(t *testing.T) {
		collector := exportTurnTraces(t)
		h := newHarness(t)
		session := createTraceSession(t, h)

		_, turnTrace := startPromptTurnTrace(context.Background(), session, promptRequest{turnID: "turn-ended-once"})
		turnTrace.end(session, telemetry.OutcomeSetupFailed, errors.New("runtime unavailable"))
		turnTrace.end(session, telemetry.OutcomeOK, nil)

		turn := onlyTurnSpan(t, collector.Flush(t), session.ID)
		if got := turn.Attributes[string(telemetry.AttrOutcome)]; got != telemetry.OutcomeSetupFailed {
			t.Fatalf("session.turn outcome = %v, want the first end's %s", got, telemetry.OutcomeSetupFailed)
		}
	})

	// IT-019
	t.Run("Should keep one turn span with a recovery event across a replayed prompt", func(t *testing.T) {
		collector := exportTurnTraces(t)
		h := newHarness(t)
		driver := &promptContextCapturingDriver{fakeDriver: h.driver}
		h.manager = newManagerWithHarness(t, h, WithDriver(driver))
		h.manager.promptRecoveryDelays = []time.Duration{0}
		session := createTraceSession(t, h)
		var prompts atomic.Int64
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			if prompts.Add(1) == 1 {
				proc.crash(errors.New("provider process exited"), "provider disconnected")
				events <- acp.AgentEvent{
					Type: acp.EventTypeError, SessionID: proc.handle.SessionID, TurnID: req.TurnID,
					Timestamp: time.Now().UTC(), Error: "peer disconnected before response",
					Failure: &store.SessionFailure{Kind: store.FailureTransport, Summary: "peer disconnected"},
				}
			} else {
				events <- acp.AgentEvent{
					Type: acp.EventTypeDone, SessionID: proc.handle.SessionID, TurnID: req.TurnID,
					Timestamp: time.Now().UTC(), PromptStopReason: acp.PromptStopReasonEndTurn,
				}
			}
			close(events)
			return events, nil
		}

		events, err := h.manager.Prompt(testutil.Context(t), session.ID, "complete a long task")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)

		turn := onlyTurnSpan(t, collector.Flush(t), session.ID)
		if !turn.HasEvent(telemetry.EventRecovery) {
			t.Fatalf("session.turn events = %+v, want a recovery event", turn.Events)
		}
		assertTurnAttributes(t, turn, session, telemetry.OutcomeOK)
		driver.mu.Lock()
		promptContexts := append([]context.Context(nil), driver.contexts...)
		driver.mu.Unlock()
		if len(promptContexts) != 2 {
			t.Fatalf("provider prompts = %d, want the failed attempt and its replay", len(promptContexts))
		}
		for attempt, ctx := range promptContexts {
			if got := trace.SpanContextFromContext(ctx).SpanID(); got != turn.SpanID {
				t.Fatalf("prompt attempt %d context span = %s, want session.turn %s", attempt+1, got, turn.SpanID)
			}
		}
	})
}

// busyTraceSession returns a session with a durable input queue whose first prompt runs until release closes.
func busyTraceSession(t *testing.T) (*harness, *Session, chan struct{}, *atomic.Int64) {
	t.Helper()
	database := openManagerInputQueueStore(t)
	h := newHarness(t, WithSessionInputQueueStore(database))
	registerManagerInputQueueWorkspace(t, database, h)
	session := createTraceSession(t, h)
	registerManagerInputQueueSession(t, database, h, session)
	release := make(chan struct{})
	prompts := &atomic.Int64{}
	h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent, 1)
		first := prompts.Add(1) == 1
		go func() {
			defer close(events)
			if first {
				<-release
			}
			events <- acp.AgentEvent{
				Type: acp.EventTypeDone, SessionID: proc.handle.SessionID, TurnID: req.TurnID,
				Timestamp: time.Now().UTC(), PromptStopReason: acp.PromptStopReasonEndTurn,
			}
		}()
		return events, nil
	}
	return h, session, release, prompts
}

// exportTurnTraces exports through the real telemetry runtime with registries that accept the harness identities.
func exportTurnTraces(t *testing.T) *otlptest.Collector {
	t.Helper()
	accept := func(string) bool { return true }
	collector := otlptest.NewCollector(t)
	collector.ExportWithMetrics(t, 1, telemetry.Registries{
		RouteTemplates: []string{traceRequestRoute},
		Agents:         accept,
		Providers:      accept,
		Models:         func(string, string) telemetry.ModelClass { return telemetry.ModelCatalog },
		IDs: map[attribute.Key]func(string) bool{
			telemetry.AttrSessionID: accept, telemetry.AttrTurnID: accept, telemetry.AttrRunID: accept,
			telemetry.AttrWorkspaceID: accept, telemetry.AttrProfileID: accept,
		},
	})
	return collector
}

func createTraceSession(t *testing.T, h *harness) *Session {
	t.Helper()
	session := createSession(t, h)
	t.Cleanup(func() {
		if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil && !errors.Is(err, ErrSessionNotFound) {
			t.Errorf("Stop(%q) cleanup error = %v", session.ID, err)
		}
	})
	return session
}

// startRequestSpan stands in for the HTTP request span the API middleware starts.
func startRequestSpan(t *testing.T) (context.Context, trace.Span) {
	t.Helper()
	return telemetry.Tracer(telemetry.ScopeHTTP).Start(testutil.Context(t), "POST "+traceRequestRoute)
}

func turnSpans(spans []otlptest.Span, sessionID string) []otlptest.Span {
	var turns []otlptest.Span
	for _, span := range spans {
		if span.Name == telemetry.SpanSessionTurn && span.Attributes[string(telemetry.AttrSessionID)] == sessionID {
			turns = append(turns, span)
		}
	}
	return turns
}

func onlyTurnSpan(t *testing.T, spans []otlptest.Span, sessionID string) otlptest.Span {
	t.Helper()
	turns := turnSpans(spans, sessionID)
	if len(turns) != 1 {
		t.Fatalf("session.turn spans for %s = %d, want 1 (exported: %+v)", sessionID, len(turns), spans)
	}
	return turns[0]
}

func traceSpans(spans []otlptest.Span, traceID trace.TraceID) []otlptest.Span {
	var matched []otlptest.Span
	for _, span := range spans {
		if span.TraceID == traceID {
			matched = append(matched, span)
		}
	}
	return matched
}

func assertTurnAttributes(t *testing.T, turn otlptest.Span, session *Session, outcome string) {
	t.Helper()
	info := session.Info()
	for key, value := range map[attribute.Key]any{
		telemetry.AttrSessionID:   session.ID,
		telemetry.AttrWorkspaceID: info.WorkspaceID,
		telemetry.AttrAgent:       info.AgentName,
		telemetry.AttrGenAISystem: info.Provider,
		telemetry.AttrOutcome:     outcome,
	} {
		if got := turn.Attributes[string(key)]; got != value {
			t.Fatalf("session.turn %s = %v, want %v (attributes %v)", key, got, value, turn.Attributes)
		}
	}
	for _, key := range []attribute.Key{telemetry.AttrTurnID, telemetry.AttrRunID, telemetry.AttrGenAIModel} {
		if turn.Attributes[string(key)] == nil {
			t.Fatalf("session.turn lacks %s (attributes %v)", key, turn.Attributes)
		}
	}
}

func receiveContext(t *testing.T, contexts <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-contexts:
		return ctx
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the runtime start context")
		return nil
	}
}

func waitForPrompts(t *testing.T, prompts *atomic.Int64, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for prompts.Load() < want {
		if time.Now().After(deadline) {
			t.Fatalf("provider prompts = %d, want %d", prompts.Load(), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPromptTurnOutcome(t *testing.T) {
	t.Parallel()

	failure := &promptPumpFatal{failure: &store.SessionFailure{Kind: store.FailureProcess}}
	cases := []struct {
		name    string
		state   *promptTurnDispatchState
		fatal   *promptPumpFatal
		want    string
		wantErr bool
	}{
		{
			name:  "Should report ok for a turn that ended normally",
			state: &promptTurnDispatchState{turnEnded: true, stopReason: acp.PromptStopReasonEndTurn}, want: "ok",
		},
		{
			name:  "Should report an error for a turn whose terminal event failed",
			state: &promptTurnDispatchState{turnEnded: true, failed: true}, want: "error", wantErr: true,
		},
		{
			name:  "Should report an error for a fatal runtime failure",
			state: &promptTurnDispatchState{turnEnded: true}, fatal: failure, want: "error", wantErr: true,
		},
		{
			name:  "Should report a provider cancellation as canceled",
			state: &promptTurnDispatchState{turnEnded: true, stopReason: acp.PromptStopReasonCancelled},
			want:  telemetry.OutcomeCancelled,
		},
		{
			name:  "Should report a turn interrupted before any terminal event as canceled",
			state: &promptTurnDispatchState{}, fatal: &promptPumpFatal{}, want: telemetry.OutcomeCancelled,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			outcome, err := promptTurnOutcome(tc.state, tc.fatal)
			if outcome != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("promptTurnOutcome() = %q, %v; want %q with error %t", outcome, err, tc.want, tc.wantErr)
			}
			if tc.wantErr && telemetry.Classify(err) != telemetry.ErrorProvider {
				t.Fatalf("turn error class = %q, want %q", telemetry.Classify(err), telemetry.ErrorProvider)
			}
		})
	}
}

// The turn span exports compozy.session_id, compozy.turn_id, and compozy.run_id only when the owner validators accept
// them, so every identity this package generates must keep matching sessionid.
func TestGeneratedIdentitiesMatchTelemetryValidators(t *testing.T) {
	t.Parallel()

	generated := func(prefix string) string {
		t.Helper()
		id, err := newIDGenerator(prefix)()
		if err != nil {
			t.Fatalf("newIDGenerator(%q)() error = %v", prefix, err)
		}
		return id
	}
	cases := []struct {
		name  string
		id    string
		valid func(string) bool
	}{
		{name: "Should accept generated session IDs", id: generated("sess"), valid: sessionid.IsSessionID},
		{name: "Should accept generated turn IDs", id: generated("turn"), valid: sessionid.IsTurnID},
		{name: "Should accept generated run IDs", id: generated("run"), valid: sessionid.IsRunID},
		{
			name: "Should accept subagent child session IDs", id: subagentChildSessionID("sess-parent", "review"),
			valid: sessionid.IsSessionID,
		},
		{
			name: "Should accept restart continuation turn IDs", id: "turn-" + restartEventID("turn-1", "continue"),
			valid: sessionid.IsTurnID,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if !tc.valid(tc.id) {
				t.Fatalf("owner validator rejected generated identity %q", tc.id)
			}
		})
	}
}
