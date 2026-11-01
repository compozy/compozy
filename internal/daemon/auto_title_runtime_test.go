package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestBootAutoTitleRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the runtime configured when automatic titles are disabled", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.DefaultWithHome(compozyconfig.HomePaths{})
		cfg.Roles.AutoTitle.Enabled = false
		state := &bootState{cfg: cfg}
		cleanup := &bootCleanup{}
		err := (&Daemon{}).bootAutoTitleRuntime(
			testutil.Context(t), state, &fakeSessionManager{}, cleanup,
		)
		if err != nil {
			t.Fatalf("bootAutoTitleRuntime() error = %v", err)
		}
		if state.runtimeWorkers.autoTitle == nil || len(cleanup.fns) != 1 {
			t.Fatalf(
				"bootAutoTitleRuntime() state = runtime:%#v cleanup:%d, want live-config runtime",
				state.runtimeWorkers.autoTitle,
				len(cleanup.fns),
			)
		}
		t.Cleanup(func() {
			if err := state.runtimeWorkers.autoTitle.Shutdown(context.Background()); err != nil {
				t.Errorf("autoTitleRuntime.Shutdown() error = %v", err)
			}
		})
	})
}

func TestAutoTitleRuntime(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should generate exactly once after the first persisted assistant response and join shutdown",
		func(t *testing.T) {
			t.Parallel()

			started := make(chan struct{})
			release := make(chan struct{})
			releaseOnce := sync.OnceFunc(func() { close(release) })
			t.Cleanup(releaseOnce)
			generator := &autoTitleGeneratorStub{
				generate: func(_ context.Context, request autoTitleRequest) (string, error) {
					close(started)
					<-release
					if request.SessionID != "sess-title" || request.AgentName != "coder" ||
						request.ProfileID != store.DefaultProfileID ||
						request.UserMessage != "Fix checkout retries" || request.AssistantReply != "I will fix it." {
						return "", errors.New("automatic title request mismatch")
					}
					return "Checkout retry fix", nil
				},
			}
			sessions := &autoTitleSessionsStub{
				events: map[string][]store.SessionEvent{
					"sess-title": {autoTitleUserEvent(t, "sess-title", "turn-title", "Fix checkout retries")},
				},
			}
			runtime := newAutoTitleRuntime(sessions, generator, time.Second, slog.Default())
			if err := runtime.Start(testutil.Context(t)); err != nil {
				t.Fatalf("Start() error = %v", err)
			}

			payload := autoTitleHookPayload("sess-title", "turn-title", "I will fix it.")
			if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), payload); err != nil {
				t.Fatalf("HandleSessionMessagePersisted(first) error = %v", err)
			}
			payload.Text = "A later assistant response."
			if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), payload); err != nil {
				t.Fatalf("HandleSessionMessagePersisted(second) error = %v", err)
			}

			select {
			case <-started:
			case <-time.After(time.Second):
				t.Fatal("automatic title generation did not start")
			}
			shutdownErr := make(chan error, 1)
			go func() {
				shutdownCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
				defer cancel()
				shutdownErr <- runtime.Shutdown(shutdownCtx)
			}()
			select {
			case err := <-shutdownErr:
				t.Fatalf("Shutdown() returned before generator release: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			releaseOnce()
			if err := <-shutdownErr; err != nil {
				t.Fatalf("Shutdown() error = %v", err)
			}
			if got := generator.callCount(); got != 1 {
				t.Fatalf("Generate() calls = %d, want 1", got)
			}
			queries := sessions.querySnapshot()
			if len(queries) != 1 || queries[0].Type != acp.EventTypeUserMessage ||
				queries[0].TurnID != "turn-title" || queries[0].Limit != 1 {
				t.Fatalf("Events() queries = %#v, want one turn-scoped user-message query", queries)
			}
			applied := sessions.appliedSnapshot()
			if len(applied) != 1 || applied[0].sessionID != "sess-title" || applied[0].title != "Checkout retry fix" {
				t.Fatalf("applied titles = %#v", applied)
			}
		},
	)

	t.Run("Should preserve title eligibility when the bounded queue rejects an enqueue", func(t *testing.T) {
		t.Parallel()

		started := make(chan struct{})
		release := make(chan struct{})
		releaseOnce := sync.OnceFunc(func() { close(release) })
		t.Cleanup(releaseOnce)
		generator := &autoTitleGeneratorStub{
			generate: func(_ context.Context, _ autoTitleRequest) (string, error) {
				close(started)
				<-release
				return "Blocking title", nil
			},
		}
		sessions := &autoTitleSessionsStub{
			events: map[string][]store.SessionEvent{
				"sess-blocking": {autoTitleUserEvent(t, "sess-blocking", "turn-blocking", "Block title generation")},
			},
		}
		runtime := newAutoTitleRuntime(sessions, generator, time.Second, slog.Default())
		if err := runtime.Start(testutil.Context(t)); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			releaseOnce()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := runtime.Shutdown(shutdownCtx); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		if err := runtime.HandleSessionMessagePersisted(
			testutil.Context(t),
			autoTitleHookPayload("sess-blocking", "turn-blocking", "Blocking response."),
		); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(blocking) error = %v", err)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("blocking automatic title generation did not start")
		}

		for index := range autoTitleQueueCapacity {
			payload := autoTitleHookPayload(
				fmt.Sprintf("sess-queued-%d", index),
				fmt.Sprintf("turn-queued-%d", index),
				"Queued response.",
			)
			if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), payload); err != nil {
				t.Fatalf("HandleSessionMessagePersisted(queue %d) error = %v", index, err)
			}
		}

		retryPayload := autoTitleHookPayload("sess-retry", "turn-retry", "Retry response.")
		err := runtime.HandleSessionMessagePersisted(testutil.Context(t), retryPayload)
		if !errors.Is(err, errAutoTitleQueueFull) {
			t.Fatalf("HandleSessionMessagePersisted(full) error = %v, want %v", err, errAutoTitleQueueFull)
		}
		if _, ok, _ := runtime.nextJob(); !ok {
			t.Fatal("nextJob() did not free one queued slot")
		}
		if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), retryPayload); err != nil {
			t.Fatalf("HandleSessionMessagePersisted(retry) error = %v", err)
		}
	})
}

func TestAutoTitleRuntimeRetries(t *testing.T) {
	t.Parallel()

	t.Run("Should retry a failed title only on a later turn until the attempt budget is spent", func(t *testing.T) {
		t.Parallel()

		generator := &autoTitleGeneratorStub{
			generate: func(context.Context, autoTitleRequest) (string, error) {
				return "", errors.New("collect automatic title output: context deadline exceeded")
			},
		}
		sessions := &autoTitleSessionsStub{events: map[string][]store.SessionEvent{
			"sess-retry": {autoTitleUserEvent(t, "sess-retry", "turn-1", "Fix checkout retries")},
		}}
		for attempt := 2; attempt <= autoTitleMaxAttempts; attempt++ {
			sessions.events["sess-retry"] = append(sessions.events["sess-retry"],
				autoTitleUserEvent(t, "sess-retry", fmt.Sprintf("turn-%d", attempt), "Retry checkout"))
		}
		events := newAutoTitleEventRecorder()
		runtime := startAutoTitleRuntime(t, sessions, generator, withAutoTitleEventWriter(events))

		for attempt := 1; attempt <= autoTitleMaxAttempts; attempt++ {
			turnID := fmt.Sprintf("turn-%d", attempt)
			deliverAutoTitlePayload(t, runtime, autoTitleHookPayload("sess-retry", turnID, "Working on it."))
			summary := events.next(t)
			var content autoTitleFailureEventPayload
			if err := json.Unmarshal(summary.Content, &content); err != nil {
				t.Fatalf("failure event content = %s, decode error = %v", summary.Content, err)
			}
			wantRetry := attempt < autoTitleMaxAttempts
			if summary.Type != eventspkg.RoleAutoTitleFailed || summary.SessionID != "sess-retry" ||
				content.Attempt != attempt || content.TurnID != turnID ||
				content.RetryOnLaterTurn != wantRetry || content.Operation != "generate title" ||
				!strings.Contains(content.Error, "deadline exceeded") {
				t.Fatalf("attempt %d failure event = %#v content = %#v", attempt, summary, content)
			}
			// More persisted messages of the failed turn must not retry it.
			deliverAutoTitlePayload(t, runtime, autoTitleHookPayload("sess-retry", turnID, "Still working."))
			if runtime.isClaimed("sess-retry") {
				t.Fatalf("attempt %d: the failed turn re-enqueued its own title", attempt)
			}
		}
		deliverAutoTitlePayload(t, runtime, autoTitleHookPayload("sess-retry", "turn-late", "Done."))
		if runtime.isClaimed("sess-retry") {
			t.Fatal("a session past the attempt budget enqueued another title")
		}
		if got := generator.callCount(); got != autoTitleMaxAttempts {
			t.Fatalf("Generate() calls = %d, want %d", got, autoTitleMaxAttempts)
		}
	})

	t.Run("Should apply the title when a later turn recovers from a failed attempt", func(t *testing.T) {
		t.Parallel()

		generator := &autoTitleGeneratorStub{
			generate: func(_ context.Context, request autoTitleRequest) (string, error) {
				if request.AssistantReply == "First reply." {
					return "", errors.New("spawn automatic title session: context deadline exceeded")
				}
				if request.UserMessage != "Retry the checkout fix" {
					return "", fmt.Errorf("retry user message = %q", request.UserMessage)
				}
				return "Checkout retry fix", nil
			},
		}
		sessions := &autoTitleSessionsStub{events: map[string][]store.SessionEvent{
			"sess-recover": {
				autoTitleUserEvent(t, "sess-recover", "turn-1", "Fix checkout retries"),
				autoTitleUserEvent(t, "sess-recover", "turn-2", "Retry the checkout fix"),
			},
		}}
		events := newAutoTitleEventRecorder()
		runtime := startAutoTitleRuntime(t, sessions, generator, withAutoTitleEventWriter(events))

		deliverAutoTitlePayload(t, runtime, autoTitleHookPayload("sess-recover", "turn-1", "First reply."))
		events.next(t)
		deliverAutoTitlePayload(t, runtime, autoTitleHookPayload("sess-recover", "turn-2", "Second reply."))
		shutdownCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()
		if err := runtime.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		applied := sessions.appliedSnapshot()
		if len(applied) != 1 || applied[0].title != "Checkout retry fix" {
			t.Fatalf("applied titles = %#v, want the recovered title", applied)
		}
	})
}

func TestForkedAutoTitleGenerator(t *testing.T) {
	t.Parallel()

	t.Run("Should skip spawning when the role is disabled", func(t *testing.T) {
		t.Parallel()

		sessions := &autoTitleSpawnSessionsStub{}
		generator := newForkedAutoTitleGenerator(
			sessions,
			resolvedRoleResolver(&ResolvedRole{Enabled: false}),
			time.Second,
			slog.Default(),
		)
		_, err := generator.Generate(testutil.Context(t), autoTitleRequest{
			SessionID:   "sess-parent",
			AgentName:   "coder",
			WorkspaceID: "ws-title",
		})
		if !errors.Is(err, errAutoTitleRoleDisabled) {
			t.Fatalf("Generate() error = %v, want %v", err, errAutoTitleRoleDisabled)
		}
		if sessions.spawnCalls != 0 {
			t.Fatalf("Spawn() calls = %d, want 0", sessions.spawnCalls)
		}
	})

	t.Run("Should use one bounded hidden spawn and stop it after collecting the title", func(t *testing.T) {
		t.Parallel()

		sessions := &autoTitleSpawnSessionsStub{}
		generator := newForkedAutoTitleGenerator(
			sessions,
			resolvedRoleResolver(&ResolvedRole{Enabled: true, Inherit: true, Model: "title-model"}),
			time.Second,
			slog.Default(),
		)
		title, err := generator.Generate(testutil.Context(t), autoTitleRequest{
			SessionID:      "sess-parent",
			AgentName:      "coder",
			WorkspaceID:    "ws-title",
			UserMessage:    "Investigate flaky checkout retries",
			AssistantReply: "I found the retry race.",
		})
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if title != "Checkout retry race" {
			t.Fatalf("Generate() title = %q, want %q", title, "Checkout retry race")
		}
		if sessions.spawn.ParentSessionID != "sess-parent" ||
			sessions.spawn.AgentName != "coder" ||
			sessions.spawn.Model != "title-model" ||
			sessions.spawn.SpawnRole != session.SpawnRoleAutoTitle ||
			!sessions.spawn.AutoStopOnParent || sessions.spawn.TTL <= time.Second {
			t.Fatalf("Spawn() opts = %#v", sessions.spawn)
		}
		if sessions.prompt.Metadata.Reason != "auto_title" || sessions.prompt.Message == "" {
			t.Fatalf("PromptSynthetic() opts = %#v", sessions.prompt)
		}
		if len(sessions.stops) != 1 || sessions.stops[0].cause != session.CauseCompleted {
			t.Fatalf("StopWithCause() calls = %#v", sessions.stops)
		}
	})

	t.Run("Should store only the title when the provider appends a notice", func(t *testing.T) {
		t.Parallel()

		messageChunk := func(id string, text string) acp.AgentEvent {
			raw := fmt.Sprintf(`{"content":{"type":"text","text":%q},"messageId":%q}`, text, id)
			return acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: text, Raw: json.RawMessage(raw)}
		}
		text := func(value string) acp.AgentEvent {
			return acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: value}
		}
		tests := []struct {
			name   string
			events []acp.AgentEvent
		}{
			{
				name:   "glued in one message",
				events: []acp.AgentEvent{text("Checkout retry race**Notice:** Stop saying that")},
			},
			{name: "streamed in chunks", events: []acp.AgentEvent{text("Checkout retry"), text(" race")}},
			{name: "in a second message", events: []acp.AgentEvent{
				messageChunk(
					"m1",
					"Checkout ",
				),
				messageChunk("m1", "retry race"),
				messageChunk("m2", "Stop saying that"),
			}},
			{name: "after a tool call", events: []acp.AgentEvent{
				text("Checkout retry race"), {Type: acp.EventTypeToolCall}, text("Stop saying that"),
			}},
		}
		for _, test := range tests {
			t.Run("Should clean output "+test.name, func(t *testing.T) {
				t.Parallel()

				sessions := &autoTitleSpawnSessionsStub{events: test.events}
				generator := newForkedAutoTitleGenerator(
					sessions,
					resolvedRoleResolver(&ResolvedRole{Enabled: true, Inherit: true}),
					time.Second,
					slog.Default(),
				)
				title, err := generator.Generate(
					testutil.Context(t),
					autoTitleRequest{SessionID: "sess-parent", AgentName: "coder"},
				)
				if err != nil {
					t.Fatalf("Generate() error = %v", err)
				}
				if title != "Checkout retry race" {
					t.Fatalf("Generate() title = %q, want %q", title, "Checkout retry race")
				}
			})
		}
	})

	t.Run("Should fail the attempt when the output has no usable title", func(t *testing.T) {
		t.Parallel()

		for _, output := range []string{"**Notice:** Stop saying that", `"Notice: be brief"`} {
			t.Run("Should reject "+output, func(t *testing.T) {
				t.Parallel()

				sessions := &autoTitleSpawnSessionsStub{events: []acp.AgentEvent{
					{Type: acp.EventTypeAgentMessage, Text: output},
				}}
				generator := newForkedAutoTitleGenerator(
					sessions,
					resolvedRoleResolver(&ResolvedRole{Enabled: true, Inherit: true}),
					time.Second,
					slog.Default(),
				)
				title, err := generator.Generate(
					testutil.Context(t),
					autoTitleRequest{SessionID: "sess-parent", AgentName: "coder"},
				)
				if err == nil || title != "" || !strings.Contains(err.Error(), "no usable title") {
					t.Fatalf("Generate() = %q, %v, want an unusable-title error", title, err)
				}
				if len(sessions.stops) != 1 || sessions.stops[0].cause != session.CauseFailed {
					t.Fatalf("StopWithCause() calls = %#v", sessions.stops)
				}
			})
		}
	})

	t.Run("Should cancel the hidden spawn promptly and still stop the child", func(t *testing.T) {
		t.Parallel()

		sessions := newCancelAwareAutoTitleSpawnSessionsStub()
		generator := newForkedAutoTitleGenerator(
			sessions,
			resolvedRoleResolver(&ResolvedRole{Enabled: true, Inherit: true}),
			5*time.Second,
			slog.Default(),
		)
		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan error, 1)
		go func() {
			_, err := generator.Generate(ctx, autoTitleRequest{
				SessionID: "sess-parent", AgentName: "coder",
				UserMessage: "Investigate checkout retries", AssistantReply: "I found the race.",
			})
			result <- err
		}()

		select {
		case <-sessions.promptStarted:
		case <-time.After(time.Second):
			t.Fatal("automatic title prompt did not start")
		}
		cancel()
		select {
		case err := <-result:
			if err == nil {
				t.Fatal("Generate() error = nil, want cancellation")
			}
		case <-time.After(time.Second):
			t.Fatal("Generate() did not honor cancellation")
		}
		if len(sessions.stops) != 1 || sessions.stops[0].cause != session.CauseFailed {
			t.Fatalf("StopWithCause() calls = %#v, want one failed stop", sessions.stops)
		}
	})
}

func autoTitleUserEvent(t *testing.T, sessionID string, turnID string, text string) store.SessionEvent {
	t.Helper()
	payload, err := transcript.MarshalPromptInputEvent(acp.AgentEvent{
		Type:      acp.EventTypeUserMessage,
		SessionID: sessionID,
		TurnID:    turnID,
		Timestamp: time.Now().UTC(),
		Text:      text,
	}, text)
	if err != nil {
		t.Fatalf("MarshalPromptInputEvent() error = %v", err)
	}
	return store.SessionEvent{
		ID: "event-user", SessionID: sessionID, Sequence: 1, TurnID: turnID,
		Type: acp.EventTypeUserMessage, AgentName: "coder", Content: payload, Timestamp: time.Now().UTC(),
	}
}

func autoTitleHookPayload(sessionID string, turnID string, text string) hookspkg.SessionMessagePersistedPayload {
	return hookspkg.SessionMessagePersistedPayload{
		ProfileID: store.DefaultProfileID, SessionID: sessionID,
		SessionType: string(session.SessionTypeUser), AgentName: "coder",
		TurnID: turnID,
		Role:   "assistant",
		Text:   text,
	}
}

type autoTitleGeneratorStub struct {
	mu       sync.Mutex
	calls    int
	generate func(context.Context, autoTitleRequest) (string, error)
}

func (s *autoTitleGeneratorStub) Generate(ctx context.Context, request autoTitleRequest) (string, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	return s.generate(ctx, request)
}

func (s *autoTitleGeneratorStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type appliedAutoTitle struct {
	sessionID string
	title     string
}

type autoTitleSessionsStub struct {
	mu      sync.Mutex
	events  map[string][]store.SessionEvent
	queries []store.EventQuery
	applied []appliedAutoTitle
}

func (s *autoTitleSessionsStub) Events(
	_ context.Context,
	id string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, query)
	var events []store.SessionEvent
	for _, event := range s.events[id] {
		if query.TurnID == "" || event.TurnID == query.TurnID {
			events = append(events, event)
		}
	}
	return events, nil
}

func (s *autoTitleSessionsStub) ApplyAutomaticSessionTitle(
	_ context.Context,
	id string,
	title string,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applied = append(s.applied, appliedAutoTitle{sessionID: id, title: title})
	return true, nil
}

func (s *autoTitleSessionsStub) appliedSnapshot() []appliedAutoTitle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]appliedAutoTitle(nil), s.applied...)
}

func (s *autoTitleSessionsStub) querySnapshot() []store.EventQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.EventQuery(nil), s.queries...)
}

type autoTitleStopCall struct {
	id    string
	cause session.StopCause
}

type autoTitleSpawnSessionsStub struct {
	spawnCalls int
	spawn      session.SpawnOpts
	prompt     session.SyntheticPromptOpts
	stops      []autoTitleStopCall
	// refuseProvider makes the provider refuse the turn on that route, the
	// post-acceptance refusal a fallback advance has to survive.
	refuseProvider string
	// refuseAfterOutput streams agent text before the refusal, which must keep
	// the turn on its accepted route.
	refuseAfterOutput bool
	providers         []string
	// events replaces the default title output when set.
	events []acp.AgentEvent
}

type cancelAwareAutoTitleSpawnSessionsStub struct {
	promptStarted chan struct{}
	stops         []autoTitleStopCall
}

func newCancelAwareAutoTitleSpawnSessionsStub() *cancelAwareAutoTitleSpawnSessionsStub {
	return &cancelAwareAutoTitleSpawnSessionsStub{promptStarted: make(chan struct{})}
}

func (s *cancelAwareAutoTitleSpawnSessionsStub) Spawn(
	_ context.Context,
	_ session.SpawnOpts,
) (*session.Session, error) {
	return &session.Session{ID: "sess-title-child"}, nil
}

func (s *cancelAwareAutoTitleSpawnSessionsStub) PromptSynthetic(
	ctx context.Context,
	_ string,
	_ session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	events := make(chan acp.AgentEvent)
	close(s.promptStarted)
	go func() {
		<-ctx.Done()
		close(events)
	}()
	return events, nil
}

func (s *cancelAwareAutoTitleSpawnSessionsStub) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	_ string,
) error {
	s.stops = append(s.stops, autoTitleStopCall{id: id, cause: cause})
	return nil
}

func (s *autoTitleSpawnSessionsStub) Spawn(
	_ context.Context,
	opts session.SpawnOpts,
) (*session.Session, error) {
	s.spawnCalls++
	s.spawn = opts
	s.providers = append(s.providers, opts.Provider)
	return &session.Session{ID: fmt.Sprintf("sess-title-child-%d", s.spawnCalls)}, nil
}

func (s *autoTitleSpawnSessionsStub) PromptSynthetic(
	_ context.Context,
	_ string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	s.prompt = opts
	events := make(chan acp.AgentEvent, len(s.events)+2)
	defer close(events)
	if s.refuseProvider != "" && s.providers[len(s.providers)-1] == s.refuseProvider {
		if s.refuseAfterOutput {
			events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "Partial"}
		}
		events <- acp.AgentEvent{
			Type:          acp.EventTypeError,
			Error:         "You've hit your session limit",
			ProviderError: &acp.ProviderErrorDiagnostic{Code: acp.ProviderErrorRateLimited},
		}
		return events, nil
	}
	if len(s.events) == 0 {
		events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "Checkout retry race"}
	}
	for _, event := range s.events {
		events <- event
	}
	events <- acp.AgentEvent{Type: acp.EventTypeDone}
	return events, nil
}

func (s *autoTitleSpawnSessionsStub) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	_ string,
) error {
	s.stops = append(s.stops, autoTitleStopCall{id: id, cause: cause})
	return nil
}

func startAutoTitleRuntime(
	t *testing.T,
	sessions autoTitleSessions,
	generator autoTitleGenerator,
	opts ...autoTitleRuntimeOption,
) *autoTitleRuntime {
	t.Helper()
	runtime := newAutoTitleRuntime(sessions, generator, time.Second, slog.Default(), opts...)
	if err := runtime.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := runtime.Shutdown(shutdownCtx); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})
	return runtime
}

func deliverAutoTitlePayload(
	t *testing.T,
	runtime *autoTitleRuntime,
	payload hookspkg.SessionMessagePersistedPayload,
) {
	t.Helper()
	if err := runtime.HandleSessionMessagePersisted(testutil.Context(t), payload); err != nil {
		t.Fatalf("HandleSessionMessagePersisted(%s) error = %v", payload.TurnID, err)
	}
}

func (r *autoTitleRuntime) isClaimed(sessionID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, claimed := r.claimed[sessionID]
	return claimed
}

type autoTitleEventRecorder struct {
	summaries chan store.EventSummary
}

func newAutoTitleEventRecorder() *autoTitleEventRecorder {
	return &autoTitleEventRecorder{summaries: make(chan store.EventSummary, autoTitleMaxAttempts+1)}
}

func (r *autoTitleEventRecorder) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	r.summaries <- summary
	return nil
}

func (r *autoTitleEventRecorder) next(t *testing.T) store.EventSummary {
	t.Helper()
	select {
	case summary := <-r.summaries:
		return summary
	case <-time.After(2 * time.Second):
		t.Fatal("automatic title failure event was not recorded")
		return store.EventSummary{}
	}
}
