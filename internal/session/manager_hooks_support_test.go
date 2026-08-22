package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

type compactionHandlerStub struct {
	mu      sync.Mutex
	calls   []CompactionRequest
	compact func(context.Context, CompactionRequest) (CompactionResult, error)
}

func (s *compactionHandlerStub) CompactSessionContext(
	ctx context.Context,
	request CompactionRequest,
) (CompactionResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, request)
	compact := s.compact
	s.mu.Unlock()
	if compact == nil {
		return CompactionResult{}, nil
	}
	return compact(ctx, request)
}

func (s *compactionHandlerStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

func recordCompactionTurn(
	t *testing.T,
	manager *Manager,
	session *Session,
	turnID string,
	text string,
	complete bool,
) {
	t.Helper()
	for _, event := range []acp.AgentEvent{
		{Type: acp.EventTypeUserMessage, TurnID: turnID, Text: text, Timestamp: time.Now().UTC()},
		{Type: acp.EventTypeDone, TurnID: turnID, Timestamp: time.Now().UTC()},
	} {
		if !complete && event.Type == acp.EventTypeDone {
			continue
		}
		if err := manager.recordEvent(testutil.Context(t), session, event); err != nil {
			t.Fatalf("recordEvent(%s) error = %v", event.Type, err)
		}
	}
}

func newNativeHookDispatcher(
	t *testing.T,
	decls []hookspkg.HookDecl,
	executors map[string]hookspkg.Executor,
) *hookspkg.Hooks {
	t.Helper()

	hooks := hookspkg.NewHooks(
		hookspkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		hookspkg.WithAsyncWorkerCount(1),
		hookspkg.WithAsyncQueueCapacity(16),
		hookspkg.WithNativeDeclarations(decls),
		hookspkg.WithExecutorResolver(func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
			executor := executors[strings.TrimSpace(decl.Name)]
			if executor == nil {
				return nil, errors.New("missing native executor")
			}
			return executor, nil
		}),
	)
	if err := hooks.Rebuild(testutil.Context(t)); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	t.Cleanup(hooks.Close)
	return hooks
}

func fullHookSet(runtime interface {
	LifecycleHooks
	RuntimeRecoveryHooks
	PromptHooks
	EventHooks
	AgentHooks
	ConversationHooks
	CompactionHooks
}) HookSet {
	return HookSet{
		Session:         runtime,
		RuntimeRecovery: runtime,
		Prompt:          runtime,
		Events:          runtime,
		Agent:           runtime,
		Conversation:    runtime,
		Compaction:      runtime,
	}
}

type spyHookDispatcher struct {
	dispatchSessionPreCreateFn              func(context.Context, hookspkg.SessionPreCreatePayload) (hookspkg.SessionPreCreatePayload, error)
	dispatchSessionPostCreateFn             func(context.Context, hookspkg.SessionPostCreatePayload) (hookspkg.SessionPostCreatePayload, error)
	dispatchSessionPreResumeFn              func(context.Context, hookspkg.SessionPreResumePayload) (hookspkg.SessionPreResumePayload, error)
	dispatchSessionPostResumeFn             func(context.Context, hookspkg.SessionPostResumePayload) (hookspkg.SessionPostResumePayload, error)
	dispatchSessionPreStopFn                func(context.Context, hookspkg.SessionPreStopPayload) (hookspkg.SessionPreStopPayload, error)
	dispatchSessionPostStopFn               func(context.Context, hookspkg.SessionPostStopPayload) (hookspkg.SessionPostStopPayload, error)
	dispatchSessionRuntimeRecoveryStartedFn func(
		context.Context,
		hookspkg.SessionRuntimeRecoveryStartedPayload,
	) (hookspkg.SessionRuntimeRecoveryStartedPayload, error)
	dispatchSessionRuntimeRecoverySucceededFn func(
		context.Context,
		hookspkg.SessionRuntimeRecoverySucceededPayload,
	) (hookspkg.SessionRuntimeRecoverySucceededPayload, error)
	dispatchSessionRuntimeRecoveryExhaustedFn func(
		context.Context,
		hookspkg.SessionRuntimeRecoveryExhaustedPayload,
	) (hookspkg.SessionRuntimeRecoveryExhaustedPayload, error)
	dispatchInputPreSubmitFn          func(context.Context, hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error)
	dispatchPromptPostAssembleFn      func(context.Context, hookspkg.PromptPayload) (hookspkg.PromptPayload, error)
	dispatchEventPreRecordFn          func(context.Context, hookspkg.EventPreRecordPayload) (hookspkg.EventPreRecordPayload, error)
	dispatchEventPostRecordFn         func(context.Context, hookspkg.EventPostRecordPayload) (hookspkg.EventPostRecordPayload, error)
	dispatchAgentPreStartFn           func(context.Context, hookspkg.AgentPreStartPayload) (hookspkg.AgentPreStartPayload, error)
	dispatchAgentSpawnedFn            func(context.Context, hookspkg.AgentSpawnedPayload) (hookspkg.AgentSpawnedPayload, error)
	dispatchAgentCrashedFn            func(context.Context, hookspkg.AgentCrashedPayload) (hookspkg.AgentCrashedPayload, error)
	dispatchAgentStoppedFn            func(context.Context, hookspkg.AgentStoppedPayload) (hookspkg.AgentStoppedPayload, error)
	dispatchTurnStartFn               func(context.Context, hookspkg.TurnStartPayload) (hookspkg.TurnStartPayload, error)
	dispatchTurnEndFn                 func(context.Context, hookspkg.TurnEndPayload) (hookspkg.TurnEndPayload, error)
	dispatchMessageStartFn            func(context.Context, hookspkg.MessageStartPayload) (hookspkg.MessageStartPayload, error)
	dispatchMessageDeltaFn            func(context.Context, hookspkg.MessageDeltaPayload) (hookspkg.MessageDeltaPayload, error)
	dispatchMessageEndFn              func(context.Context, hookspkg.MessageEndPayload) (hookspkg.MessageEndPayload, error)
	dispatchSessionMessagePersistedFn func(
		context.Context,
		hookspkg.SessionMessagePersistedPayload,
	) (hookspkg.SessionMessagePersistedPayload, error)
	dispatchContextPreCompactFn  func(context.Context, hookspkg.ContextPreCompactPayload) (hookspkg.ContextPreCompactPayload, error)
	dispatchContextPostCompactFn func(context.Context, hookspkg.ContextPostCompactPayload) (hookspkg.ContextPostCompactPayload, error)
}

func (s *spyHookDispatcher) DispatchSessionPreCreate(
	ctx context.Context,
	payload hookspkg.SessionPreCreatePayload,
) (hookspkg.SessionPreCreatePayload, error) {
	if s.dispatchSessionPreCreateFn != nil {
		return s.dispatchSessionPreCreateFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPostCreate(
	ctx context.Context,
	payload hookspkg.SessionPostCreatePayload,
) (hookspkg.SessionPostCreatePayload, error) {
	if s.dispatchSessionPostCreateFn != nil {
		return s.dispatchSessionPostCreateFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPreResume(
	ctx context.Context,
	payload hookspkg.SessionPreResumePayload,
) (hookspkg.SessionPreResumePayload, error) {
	if s.dispatchSessionPreResumeFn != nil {
		return s.dispatchSessionPreResumeFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPostResume(
	ctx context.Context,
	payload hookspkg.SessionPostResumePayload,
) (hookspkg.SessionPostResumePayload, error) {
	if s.dispatchSessionPostResumeFn != nil {
		return s.dispatchSessionPostResumeFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPreStop(
	ctx context.Context,
	payload hookspkg.SessionPreStopPayload,
) (hookspkg.SessionPreStopPayload, error) {
	if s.dispatchSessionPreStopFn != nil {
		return s.dispatchSessionPreStopFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionPostStop(
	ctx context.Context,
	payload hookspkg.SessionPostStopPayload,
) (hookspkg.SessionPostStopPayload, error) {
	if s.dispatchSessionPostStopFn != nil {
		return s.dispatchSessionPostStopFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionRuntimeRecoveryStarted(
	ctx context.Context,
	payload hookspkg.SessionRuntimeRecoveryStartedPayload,
) (hookspkg.SessionRuntimeRecoveryStartedPayload, error) {
	if s.dispatchSessionRuntimeRecoveryStartedFn != nil {
		return s.dispatchSessionRuntimeRecoveryStartedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionRuntimeRecoverySucceeded(
	ctx context.Context,
	payload hookspkg.SessionRuntimeRecoverySucceededPayload,
) (hookspkg.SessionRuntimeRecoverySucceededPayload, error) {
	if s.dispatchSessionRuntimeRecoverySucceededFn != nil {
		return s.dispatchSessionRuntimeRecoverySucceededFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionRuntimeRecoveryExhausted(
	ctx context.Context,
	payload hookspkg.SessionRuntimeRecoveryExhaustedPayload,
) (hookspkg.SessionRuntimeRecoveryExhaustedPayload, error) {
	if s.dispatchSessionRuntimeRecoveryExhaustedFn != nil {
		return s.dispatchSessionRuntimeRecoveryExhaustedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchInputPreSubmit(
	ctx context.Context,
	payload hookspkg.InputPreSubmitPayload,
) (hookspkg.InputPreSubmitPayload, error) {
	if s.dispatchInputPreSubmitFn != nil {
		return s.dispatchInputPreSubmitFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchPromptPostAssemble(
	ctx context.Context,
	payload hookspkg.PromptPayload,
) (hookspkg.PromptPayload, error) {
	if s.dispatchPromptPostAssembleFn != nil {
		return s.dispatchPromptPostAssembleFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchEventPreRecord(
	ctx context.Context,
	payload hookspkg.EventPreRecordPayload,
) (hookspkg.EventPreRecordPayload, error) {
	if s.dispatchEventPreRecordFn != nil {
		return s.dispatchEventPreRecordFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchEventPostRecord(
	ctx context.Context,
	payload hookspkg.EventPostRecordPayload,
) (hookspkg.EventPostRecordPayload, error) {
	if s.dispatchEventPostRecordFn != nil {
		return s.dispatchEventPostRecordFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentPreStart(
	ctx context.Context,
	payload hookspkg.AgentPreStartPayload,
) (hookspkg.AgentPreStartPayload, error) {
	if s.dispatchAgentPreStartFn != nil {
		return s.dispatchAgentPreStartFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentSpawned(
	ctx context.Context,
	payload hookspkg.AgentSpawnedPayload,
) (hookspkg.AgentSpawnedPayload, error) {
	if s.dispatchAgentSpawnedFn != nil {
		return s.dispatchAgentSpawnedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentCrashed(
	ctx context.Context,
	payload hookspkg.AgentCrashedPayload,
) (hookspkg.AgentCrashedPayload, error) {
	if s.dispatchAgentCrashedFn != nil {
		return s.dispatchAgentCrashedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchAgentStopped(
	ctx context.Context,
	payload hookspkg.AgentStoppedPayload,
) (hookspkg.AgentStoppedPayload, error) {
	if s.dispatchAgentStoppedFn != nil {
		return s.dispatchAgentStoppedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchTurnStart(
	ctx context.Context,
	payload hookspkg.TurnStartPayload,
) (hookspkg.TurnStartPayload, error) {
	if s.dispatchTurnStartFn != nil {
		return s.dispatchTurnStartFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchTurnEnd(
	ctx context.Context,
	payload hookspkg.TurnEndPayload,
) (hookspkg.TurnEndPayload, error) {
	if s.dispatchTurnEndFn != nil {
		return s.dispatchTurnEndFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchMessageStart(
	ctx context.Context,
	payload hookspkg.MessageStartPayload,
) (hookspkg.MessageStartPayload, error) {
	if s.dispatchMessageStartFn != nil {
		return s.dispatchMessageStartFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchMessageDelta(
	ctx context.Context,
	payload hookspkg.MessageDeltaPayload,
) (hookspkg.MessageDeltaPayload, error) {
	if s.dispatchMessageDeltaFn != nil {
		return s.dispatchMessageDeltaFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchMessageEnd(
	ctx context.Context,
	payload hookspkg.MessageEndPayload,
) (hookspkg.MessageEndPayload, error) {
	if s.dispatchMessageEndFn != nil {
		return s.dispatchMessageEndFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchSessionMessagePersisted(
	ctx context.Context,
	payload hookspkg.SessionMessagePersistedPayload,
) (hookspkg.SessionMessagePersistedPayload, error) {
	if s.dispatchSessionMessagePersistedFn != nil {
		return s.dispatchSessionMessagePersistedFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchContextPreCompact(
	ctx context.Context,
	payload hookspkg.ContextPreCompactPayload,
) (hookspkg.ContextPreCompactPayload, error) {
	if s.dispatchContextPreCompactFn != nil {
		return s.dispatchContextPreCompactFn(ctx, payload)
	}
	return payload, nil
}

func (s *spyHookDispatcher) DispatchContextPostCompact(
	ctx context.Context,
	payload hookspkg.ContextPostCompactPayload,
) (hookspkg.ContextPostCompactPayload, error) {
	if s.dispatchContextPostCompactFn != nil {
		return s.dispatchContextPostCompactFn(ctx, payload)
	}
	return payload, nil
}

type orderedRecorder struct {
	onRecord func(store.SessionEvent)
	nextSeq  int64
	events   []store.SessionEvent
}

func (r *orderedRecorder) ListTokenUsage(context.Context) ([]store.TokenUsage, error) {
	return nil, nil
}

type recordingNetworkPeerLifecycle struct {
	joinErr  error
	leaveErr error
	joins    []networkJoinCall
	leaves   []string
}

type networkJoinCall struct {
	sessionID    string
	peerID       string
	channel      string
	capabilities []NetworkPeerCapability
}

func (r *recordingNetworkPeerLifecycle) JoinChannel(
	_ context.Context,
	join NetworkPeerJoin,
) error {
	r.joins = append(r.joins, networkJoinCall{
		sessionID:    join.SessionID,
		peerID:       join.PeerID,
		channel:      join.Channel,
		capabilities: cloneNetworkPeerCapabilities(join.Capabilities),
	})
	return r.joinErr
}

func (r *recordingNetworkPeerLifecycle) LeaveChannel(_ context.Context, sessionID string) error {
	r.leaves = append(r.leaves, sessionID)
	return r.leaveErr
}

func (r *recordingNetworkPeerLifecycle) joinCount() int {
	return len(r.joins)
}

func (r *recordingNetworkPeerLifecycle) leaveCount() int {
	return len(r.leaves)
}

func (r *orderedRecorder) Record(_ context.Context, event store.SessionEvent) error {
	_, err := r.RecordPersisted(context.Background(), event)
	return err
}

func (r *orderedRecorder) RecordPersisted(_ context.Context, event store.SessionEvent) (store.SessionEvent, error) {
	if event.ID == "" {
		generatedID, err := store.NewID("ev")
		if err != nil {
			return store.SessionEvent{}, err
		}
		event.ID = generatedID
	}
	if event.Sequence <= 0 {
		r.nextSeq++
		event.Sequence = r.nextSeq
	}
	r.events = append(r.events, event)
	if r.onRecord != nil {
		r.onRecord(event)
	}
	return event, nil
}

func (r *orderedRecorder) RecordTokenUsage(context.Context, store.TokenUsage) error {
	return nil
}

func (r *orderedRecorder) Query(context.Context, store.EventQuery) ([]store.SessionEvent, error) {
	return append([]store.SessionEvent(nil), r.events...), nil
}

func (r *orderedRecorder) History(context.Context, store.EventQuery) ([]store.TurnHistory, error) {
	return nil, nil
}

func (r *orderedRecorder) Close(context.Context) error {
	return nil
}
