package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
)

func TestBootAutoTitleRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should keep the runtime configured when automatic titles are disabled", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultWithHome(aghconfig.HomePaths{})
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
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			generator := &autoTitleGeneratorStub{
				generate: func(_ context.Context, request autoTitleRequest) (string, error) {
					close(started)
					<-release
					if request.SessionID != "sess-title" || request.AgentName != "coder" ||
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
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				shutdownErr <- runtime.Shutdown(shutdownCtx)
			}()
			select {
			case err := <-shutdownErr:
				t.Fatalf("Shutdown() returned before generator release: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			releaseOnce.Do(func() { close(release) })
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
		var releaseOnce sync.Once
		t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
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
			releaseOnce.Do(func() { close(release) })
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

func TestForkedAutoTitleGenerator(t *testing.T) {
	t.Parallel()

	t.Run("Should skip spawning when the role is disabled", func(t *testing.T) {
		t.Parallel()

		sessions := &autoTitleSpawnSessionsStub{}
		generator := newForkedAutoTitleGenerator(
			sessions,
			resolvedRoleResolver(ResolvedRole{Enabled: false}),
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
			resolvedRoleResolver(ResolvedRole{Enabled: true, Inherit: true, Model: "title-model"}),
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

	t.Run("Should cancel the hidden spawn promptly and still stop the child", func(t *testing.T) {
		t.Parallel()

		sessions := newCancelAwareAutoTitleSpawnSessionsStub()
		generator := newForkedAutoTitleGenerator(
			sessions,
			resolvedRoleResolver(ResolvedRole{Enabled: true, Inherit: true}),
			5*time.Second,
			slog.Default(),
		)
		ctx, cancel := context.WithCancel(context.Background())
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
		SessionContext: hookspkg.SessionContext{
			SessionID: sessionID, SessionType: string(session.SessionTypeUser), AgentName: "coder",
		},
		TurnContext: hookspkg.TurnContext{TurnID: turnID},
		Role:        "assistant",
		Text:        text,
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
	return append([]store.SessionEvent(nil), s.events[id]...), nil
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
	return &session.Session{ID: "sess-title-child"}, nil
}

func (s *autoTitleSpawnSessionsStub) PromptSynthetic(
	_ context.Context,
	_ string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	s.prompt = opts
	events := make(chan acp.AgentEvent, 2)
	events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, Text: "Checkout retry race"}
	events <- acp.AgentEvent{Type: acp.EventTypeDone}
	close(events)
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
