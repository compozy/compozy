package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	commandpkg "github.com/compozy/compozy/internal/command"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

func TestPromptUsesPatchedInputMessage(t *testing.T) {
	t.Parallel()

	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-input",
			Event:        hookspkg.HookInputPreSubmit,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-input": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPatch, error) {
					message := "patched message"
					return hookspkg.InputPreSubmitPatch{Message: &message}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "original message")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	if got := h.driver.promptCalls[0].Message; got != "patched message" {
		t.Fatalf("prompt message = %q, want %q", got, "patched message")
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	userMessage := storedEventByType(t, stored, acp.EventTypeUserMessage)
	if !strings.Contains(userMessage.Content, `"text":"patched message"`) {
		t.Fatalf("stored user message content = %q, want patched text", userMessage.Content)
	}
	if !strings.Contains(userMessage.Content, `"authored_text":"original message"`) {
		t.Fatalf("stored user message content = %q, want exact authored text", userMessage.Content)
	}
}

func TestPromptInputHookReconcilesSkillInvocations(t *testing.T) {
	t.Parallel()
	t.Run("Should retain only admitted skill invocations that survive the hook", func(t *testing.T) {
		t.Parallel()

		catalog, err := commandpkg.BuildCatalog(
			commandpkg.DefaultBuiltins(),
			nil,
			[]commandpkg.SkillSpec{
				{
					Name: "review", Description: "Review carefully", Available: true,
					Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
				},
				{
					Name: "docs", Description: "Read the docs", Available: true,
					Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
				},
				{
					Name: "new", Description: "New task", Available: true,
					Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
				},
			},
		)
		if err != nil {
			t.Fatalf("BuildCatalog() error = %v", err)
		}
		authored := "Use /review and /docs"
		wantAdmitted, err := commandpkg.ParseSkillInvocations(authored, catalog)
		if err != nil {
			t.Fatalf("ParseSkillInvocations() error = %v", err)
		}
		if len(wantAdmitted) != 2 {
			t.Fatalf("ParseSkillInvocations() = %#v, want review and docs", wantAdmitted)
		}

		dispatcher := &spyHookDispatcher{
			dispatchInputPreSubmitFn: func(
				_ context.Context,
				payload hookspkg.InputPreSubmitPayload,
			) (hookspkg.InputPreSubmitPayload, error) {
				payload.Message = "Use /docs and add /new"
				return payload, nil
			},
		}
		service := &busyInputCommandService{
			catalog:  catalog,
			expanded: make(chan []commandpkg.Invocation, 1),
		}
		h := newHarness(t, WithCommandService(service), WithHookSet(fullHookSet(dispatcher)))
		sess := createSession(t, h)
		t.Cleanup(func() {
			reportSessionStop(t, h, sess.ID)
		})

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message:       authored,
			AllowCommands: true,
			Caller:        PromptCaller{Kind: "human", ID: "operator", Source: "http"},
		})
		if err != nil {
			t.Fatalf("SendPrompt() error = %v", err)
		}
		_ = collectEvents(t, result.Events)

		select {
		case got := <-service.expanded:
			wantSurvivor := wantAdmitted[1:]
			if !slices.Equal(got, wantSurvivor) {
				t.Fatalf("Expand() invocations = %#v, want only authored docs invocation %#v", got, wantSurvivor)
			}
		case <-time.After(time.Second):
			t.Fatal("hook-reconciled skill invocation did not reach expansion")
		}

		if got, want := h.driver.promptCalls[0].Message, "EXPANDED\n\nUse /docs and add /new"; got != want {
			t.Fatalf("driver prompt message = %q, want %q", got, want)
		}
		stored := storedEventByType(t, readStoredEvents(t, sess), acp.EventTypeUserMessage)
		decoded, err := transcript.UnmarshalAgentEvent(stored.Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent() error = %v", err)
		}
		storedInvocations := decoded.SkillInvocations()
		if len(storedInvocations) != 1 {
			t.Fatalf("stored skill invocations = %#v, want only surviving docs invocation", storedInvocations)
		}
		got := storedInvocations[0]
		want := wantAdmitted[1]
		if got.Ref.CommandID != want.Ref.CommandID || got.Ref.Name != want.Ref.Name ||
			got.Ref.Source != want.Ref.Source || got.Start != want.Start || got.End != want.End {
			t.Fatalf("stored surviving invocation = %#v, want authored docs identity and offsets %#v", got, want)
		}
	})
}

func TestRecordEventDispatchesAroundPersistence(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 3)
	dispatcher := &spyHookDispatcher{
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			order = append(order, "pre:"+payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			order = append(order, "post:"+payload.RecordType)
			return payload, nil
		},
	}
	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	recorder := &orderedRecorder{
		onRecord: func(event store.SessionEvent) {
			order = append(order, "record:"+event.Type)
		},
	}
	now := h.manager.now()
	session := &Session{
		ID:          "sess-event",
		AgentName:   "coder",
		WorkspaceID: h.workspaceID,
		Workspace:   h.workspace,
		Type:        SessionTypeUser,
		State:       StateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		recorder:    recorder,
	}

	err := h.manager.recordEvent(testutil.Context(t), session, acp.AgentEvent{
		Type:      acp.EventTypeDone,
		TurnID:    "turn-1",
		Timestamp: now,
		Text:      "done",
	})
	if err != nil {
		t.Fatalf("recordEvent() error = %v", err)
	}

	want := []string{"pre:done", "record:done", "post:done"}
	if !testutil.EqualStringSlices(order, want) {
		t.Fatalf("dispatch order = %#v, want %#v", order, want)
	}
}

func TestRecordEventDispatchesSessionMessagePersistedAfterDurableAgentMessage(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 4)
	var persistedPayload hookspkg.SessionMessagePersistedPayload
	dispatcher := &spyHookDispatcher{
		dispatchEventPreRecordFn: func(_ context.Context, payload hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error) {
			order = append(order, "pre:"+payload.RecordType)
			return payload, nil
		},
		dispatchEventPostRecordFn: func(_ context.Context, payload hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error) {
			order = append(order, "post:"+payload.RecordType)
			if payload.Sequence != 1 {
				t.Fatalf("event.post_record sequence = %d, want 1", payload.Sequence)
			}
			return payload, nil
		},
		dispatchSessionMessagePersistedFn: func(_ context.Context, payload hookspkg.SessionMessagePersistedPayload) (hookspkg.SessionMessagePersistedPayload, error) {
			order = append(order, "persisted:"+payload.Role)
			persistedPayload = payload
			return payload, nil
		},
	}
	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))

	recorder := &orderedRecorder{
		onRecord: func(event store.SessionEvent) {
			order = append(order, "record:"+event.Type)
		},
	}
	now := h.manager.now()
	session := &Session{
		ID:          "sess-event",
		AgentName:   "coder",
		WorkspaceID: h.workspaceID,
		Workspace:   h.workspace,
		Type:        SessionTypeUser,
		State:       StateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		recorder:    recorder,
	}

	err := h.manager.recordEvent(testutil.Context(t), session, acp.AgentEvent{
		Type:      acp.EventTypeAgentMessage,
		TurnID:    "turn-1",
		Timestamp: now,
		Text:      "durable reply",
		Raw:       []byte(`{"type":"agent_message","text":"durable reply"}`),
	})
	if err != nil {
		t.Fatalf("recordEvent() error = %v", err)
	}

	want := []string{"pre:agent_message", "record:agent_message", "post:agent_message", "persisted:assistant"}
	if !testutil.EqualStringSlices(order, want) {
		t.Fatalf("dispatch order = %#v, want %#v", order, want)
	}
	if persistedPayload.MessageSeq != 1 {
		t.Fatalf("message seq = %d, want 1", persistedPayload.MessageSeq)
	}
	if persistedPayload.Text != "durable reply" {
		t.Fatalf("payload text = %q, want durable reply", persistedPayload.Text)
	}
	if persistedPayload.RootSessionID != session.ID {
		t.Fatalf("root session id = %q, want %q", persistedPayload.RootSessionID, session.ID)
	}
	if persistedPayload.ParentSessionID != "" {
		t.Fatalf("parent session id = %q, want empty root session", persistedPayload.ParentSessionID)
	}
	if persistedPayload.ActorKind != sessionActorKindRoot {
		t.Fatalf("actor kind = %q, want agent_root", persistedPayload.ActorKind)
	}
}

func TestMessagePersistedLineageUsesSessionTypeForActorKind(t *testing.T) {
	t.Parallel()

	lineage := &store.SessionLineage{
		ParentSessionID: "sess-orchestrator", RootSessionID: "sess-orchestrator", SpawnDepth: 1,
	}
	goal := &Session{ID: "sess-goal", Type: SessionTypeSystem, Lineage: lineage}
	rootID, parentID, actorKind, sessionID := messagePersistedLineage(goal)
	if rootID != "sess-orchestrator" || parentID != "sess-orchestrator" ||
		actorKind != sessionActorKindRoot || sessionID != "sess-goal" {
		t.Fatalf(
			"messagePersistedLineage(system provenance) = %q/%q/%q/%q, want correlated agent_root",
			rootID, parentID, actorKind, sessionID,
		)
	}

	spawned := &Session{ID: "sess-worker", Type: SessionTypeSpawned, Lineage: lineage}
	spawnedRootID, spawnedParentID, actorKind, spawnedSessionID := messagePersistedLineage(spawned)
	if actorKind != sessionActorKindSubagent {
		t.Fatalf("messagePersistedLineage(spawned) actor kind = %q, want agent_subagent", actorKind)
	}
	if spawnedRootID == "" || spawnedParentID == "" || spawnedSessionID != spawned.ID {
		t.Fatalf(
			"messagePersistedLineage(spawned) = %q/%q/%q, want complete lineage",
			spawnedRootID, spawnedParentID, spawnedSessionID,
		)
	}
}

func TestPromptDispatchesTurnAndMessageHooksAtACPBoundaries(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 6)
	var (
		turnStartPayload        hookspkg.TurnStartPayload
		messageStartPayload     hookspkg.MessageStartPayload
		messageDeltaPayload     hookspkg.MessageDeltaPayload
		messagePersistedPayload hookspkg.SessionMessagePersistedPayload
		messageEndPayload       hookspkg.MessageEndPayload
		turnEndPayload          hookspkg.TurnEndPayload
	)

	dispatcher := &spyHookDispatcher{
		dispatchTurnStartFn: func(_ context.Context, payload hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error) {
			order = append(order, "turn.start")
			turnStartPayload = payload
			return payload, nil
		},
		dispatchMessageStartFn: func(_ context.Context, payload hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error) {
			order = append(order, "message.start")
			messageStartPayload = payload
			return payload, nil
		},
		dispatchMessageDeltaFn: func(_ context.Context, payload hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error) {
			order = append(order, "message.delta")
			messageDeltaPayload = payload
			return payload, nil
		},
		dispatchSessionMessagePersistedFn: func(_ context.Context, payload hookspkg.SessionMessagePersistedPayload) (hookspkg.SessionMessagePersistedPayload, error) {
			order = append(order, "session.message_persisted")
			messagePersistedPayload = payload
			return payload, nil
		},
		dispatchMessageEndFn: func(_ context.Context, payload hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error) {
			order = append(order, "message.end")
			messageEndPayload = payload
			return payload, nil
		},
		dispatchTurnEndFn: func(_ context.Context, payload hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error) {
			order = append(order, "turn.end")
			turnEndPayload = payload
			return payload, nil
		},
	}

	h := newHarness(t, WithHookSet(fullHookSet(dispatcher)))
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
		t.Fatalf("len(events) = %d, want 2", len(events))
	}

	wantOrder := []string{
		"turn.start",
		"message.start",
		"message.delta",
		"session.message_persisted",
		"message.end",
		"turn.end",
	}
	if !testutil.EqualStringSlices(order, wantOrder) {
		t.Fatalf("hook order = %#v, want %#v", order, wantOrder)
	}

	if turnStartPayload.UserMessage != "hello" {
		t.Fatalf("turn.start user message = %q, want %q", turnStartPayload.UserMessage, "hello")
	}
	if turnStartPayload.TurnID == "" {
		t.Fatal("turn.start turn id = empty, want populated turn id")
	}
	if turnStartPayload.InputClass != hookInputClassUserMessage {
		t.Fatalf("turn.start input class = %q, want %q", turnStartPayload.InputClass, hookInputClassUserMessage)
	}
	if messageStartPayload.MessageID == "" {
		t.Fatal("message.start message id = empty, want populated message id")
	}
	if messageStartPayload.Role != hookMessageRoleAssistant {
		t.Fatalf("message.start role = %q, want %q", messageStartPayload.Role, hookMessageRoleAssistant)
	}
	if messageStartPayload.DeltaType != hookMessageDeltaTypeFull {
		t.Fatalf("message.start delta type = %q, want %q", messageStartPayload.DeltaType, hookMessageDeltaTypeFull)
	}
	if messageStartPayload.Text != "reply" {
		t.Fatalf("message.start text = %q, want %q", messageStartPayload.Text, "reply")
	}
	if messageDeltaPayload.MessageID != messageStartPayload.MessageID {
		t.Fatalf("message.delta message id = %q, want %q", messageDeltaPayload.MessageID, messageStartPayload.MessageID)
	}
	if messageDeltaPayload.DeltaType != hookMessageDeltaTypeText {
		t.Fatalf("message.delta delta type = %q, want %q", messageDeltaPayload.DeltaType, hookMessageDeltaTypeText)
	}
	if messagePersistedPayload.MessageSeq <= 0 {
		t.Fatalf(
			"session.message_persisted sequence = %d, want durable positive sequence",
			messagePersistedPayload.MessageSeq,
		)
	}
	if messagePersistedPayload.Text != "reply" {
		t.Fatalf("session.message_persisted text = %q, want %q", messagePersistedPayload.Text, "reply")
	}
	if messagePersistedPayload.Role != hookMessageRoleAssistant {
		t.Fatalf(
			"session.message_persisted role = %q, want %q",
			messagePersistedPayload.Role,
			hookMessageRoleAssistant,
		)
	}
	if messageEndPayload.MessageID != messageStartPayload.MessageID {
		t.Fatalf("message.end message id = %q, want %q", messageEndPayload.MessageID, messageStartPayload.MessageID)
	}
	if messageEndPayload.Text != "reply" {
		t.Fatalf("message.end text = %q, want %q", messageEndPayload.Text, "reply")
	}
	if turnEndPayload.TurnID != turnStartPayload.TurnID {
		t.Fatalf("turn.end turn id = %q, want %q", turnEndPayload.TurnID, turnStartPayload.TurnID)
	}
}

func TestMessageStartPatchUpdatesFirstAssistantChunk(t *testing.T) {
	t.Parallel()

	patched := "patched reply"
	hooks := newNativeHookDispatcher(t,
		[]hookspkg.HookDecl{{
			Name:         "patch-message-start",
			Event:        hookspkg.HookMessageStart,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}},
		map[string]hookspkg.Executor{
			"patch-message-start": hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.MessageStartPayload) (hookspkg.MessageStartPatch, error) {
					return hookspkg.MessageStartPatch{Text: &patched}, nil
				},
			),
		},
	)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
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
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Text != patched {
		t.Fatalf("first event text = %q, want %q", events[0].Text, patched)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	assistantMessage := storedEventByType(t, stored, acp.EventTypeAgentMessage)
	if !strings.Contains(assistantMessage.Content, patched) {
		t.Fatalf("stored assistant content = %q, want patched reply", assistantMessage.Content)
	}
}

func TestMessageDeltaAsyncHooksDoNotBlockPromptStreaming(t *testing.T) {
	t.Parallel()

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseHook := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseHook)

	hooks := hookspkg.NewHooks(
		hookspkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		hookspkg.WithAsyncWorkerCount(1),
		hookspkg.WithAsyncQueueCapacity(1),
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
			Name:         "observe-message-delta",
			Event:        hookspkg.HookMessageDelta,
			Mode:         hookspkg.HookModeAsync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}}),
		hookspkg.WithExecutorResolver(func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
			if strings.TrimSpace(decl.Name) != "observe-message-delta" {
				return nil, errors.New("unexpected hook name")
			}
			return hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPatch, error) {
					select {
					case started <- struct{}{}:
					default:
					}
					<-release
					return hookspkg.MessageDeltaPatch{}, nil
				},
			), nil
		}),
	)
	if err := hooks.Rebuild(testutil.Context(t)); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	t.Cleanup(hooks.Close)

	h := newHarness(t, WithHookSet(fullHookSet(hooks)))
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "hello")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	select {
	case event, ok := <-eventsCh:
		if !ok {
			t.Fatal("first prompt event channel read closed early, want agent message")
		}
		if event.Type != acp.EventTypeAgentMessage {
			t.Fatalf("first prompt event type = %q, want %q", event.Type, acp.EventTypeAgentMessage)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first prompt event; message.delta hook blocked streaming")
	}

	select {
	case event, ok := <-eventsCh:
		if !ok {
			t.Fatal("second prompt event channel read closed early, want done event")
		}
		if event.Type != acp.EventTypeDone {
			t.Fatalf("second prompt event type = %q, want %q", event.Type, acp.EventTypeDone)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for done event; message.delta hook blocked prompt completion")
	}

	select {
	case _, ok := <-eventsCh:
		if ok {
			t.Fatal("prompt event channel still open after done event")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prompt event channel close")
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async message.delta hook to start")
	}
	releaseHook()
}
