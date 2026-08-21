package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/google/uuid"
)

const clarifyObservabilityTimeout = 2 * time.Second

type clarifyEventPublisher interface {
	PublishClarifyEvent(context.Context, toolspkg.ClarifyEvent) error
}

type clarifyOrphanResolver interface {
	ResolveOrphanedClarification(
		context.Context,
		toolspkg.Scope,
		string,
		toolspkg.ClarifyAnswerRequest,
	) (toolspkg.ClarifyAnswerResult, error)
}

type clarifyBridge struct {
	mu                sync.Mutex
	pending           map[string]*clarifyHandle
	timeout           time.Duration
	publisher         clarifyEventPublisher
	summaries         clarifySummaryWriter
	logger            *slog.Logger
	profileForSession func(context.Context, string) (string, error)
	now               func() time.Time
	newID             func() string
	closed            bool
	waiters           sync.WaitGroup
}

type clarifyHandle struct {
	completionMu sync.Mutex
	pending      toolspkg.ClarifyPending
	result       chan clarifyResult
	published    chan struct{}
	ready        atomic.Bool
	terminal     bool
	profileID    string
}

type clarifyResult struct {
	answer toolspkg.ClarifyAnswer
	err    error
}

type clarifyBridgeOption func(*clarifyBridge)

func withClarifyClock(now func() time.Time) clarifyBridgeOption {
	return func(bridge *clarifyBridge) {
		bridge.now = now
	}
}

func withClarifyIDGenerator(newID func() string) clarifyBridgeOption {
	return func(bridge *clarifyBridge) {
		bridge.newID = newID
	}
}

func withClarifySessionProfileResolver(
	resolve func(context.Context, string) (string, error),
) clarifyBridgeOption {
	return func(bridge *clarifyBridge) {
		bridge.profileForSession = resolve
	}
}

func clarifySessionProfileResolver(
	manager interface {
		Status(context.Context, string) (*session.Info, error)
	},
) func(context.Context, string) (string, error) {
	if manager == nil {
		return nil
	}
	return func(ctx context.Context, sessionID string) (string, error) {
		info, err := manager.Status(ctx, sessionID)
		if err != nil {
			return "", err
		}
		if info == nil || strings.TrimSpace(info.ProfileID) == "" {
			return "", errors.New("daemon: clarification session has no bound profile")
		}
		return strings.TrimSpace(info.ProfileID), nil
	}
}

func newClarifyBridge(
	timeout time.Duration,
	publisher clarifyEventPublisher,
	summaries clarifySummaryWriter,
	logger *slog.Logger,
	options ...clarifyBridgeOption,
) (*clarifyBridge, error) {
	if timeout <= 0 {
		return nil, errors.New("daemon: clarification timeout must be positive")
	}
	if publisher == nil {
		return nil, errors.New("daemon: clarification event publisher is required")
	}
	if logger == nil {
		return nil, errors.New("daemon: clarification logger is required")
	}
	bridge := &clarifyBridge{
		pending:   make(map[string]*clarifyHandle),
		timeout:   timeout,
		publisher: publisher,
		summaries: summaries,
		logger:    logger,
		now:       time.Now,
		newID:     uuid.NewString,
	}
	for _, option := range options {
		if option != nil {
			option(bridge)
		}
	}
	if bridge.now == nil || bridge.newID == nil {
		return nil, errors.New("daemon: clarification clock and ID generator are required")
	}
	return bridge, nil
}

var _ toolspkg.ClarifyBroker = (*clarifyBridge)(nil)

func (b *clarifyBridge) Ask(
	ctx context.Context,
	scope toolspkg.Scope,
	question toolspkg.ClarifyQuestion,
) (toolspkg.ClarifyAnswer, error) {
	if ctx == nil {
		return toolspkg.ClarifyAnswer{}, errors.New("daemon: clarification context is required")
	}
	normalizedScope, err := normalizeClarifyScope(scope)
	if err != nil {
		return toolspkg.ClarifyAnswer{}, err
	}
	normalizedQuestion, err := question.Normalize()
	if err != nil {
		return toolspkg.ClarifyAnswer{}, err
	}
	now := b.now().UTC()
	handle := &clarifyHandle{
		profileID: strings.TrimSpace(normalizedScope.ProfileID),
		pending: toolspkg.ClarifyPending{
			RequestID:   strings.TrimSpace(b.newID()),
			WorkspaceID: normalizedScope.WorkspaceID,
			SessionID:   normalizedScope.SessionID,
			AgentName:   normalizedScope.AgentName,
			Question:    normalizedQuestion.Question,
			Choices:     append([]string(nil), normalizedQuestion.Choices...),
			AskedAt:     now,
			Deadline:    now.Add(b.timeout),
		},
		result:    make(chan clarifyResult, 1),
		published: make(chan struct{}),
	}
	if handle.pending.RequestID == "" {
		return toolspkg.ClarifyAnswer{}, errors.New("daemon: clarification request ID is required")
	}
	handle.completionMu.Lock()
	if err := b.register(handle); err != nil {
		handle.completionMu.Unlock()
		return toolspkg.ClarifyAnswer{}, err
	}
	defer b.waiters.Done()

	if err := b.publish(ctx, handle, toolspkg.ClarifyStatusPending, nil, true); err != nil {
		handle.terminal = true
		b.rollback(handle)
		close(handle.published)
		handle.completionMu.Unlock()
		return toolspkg.ClarifyAnswer{}, fmt.Errorf("daemon: persist pending clarification: %w", err)
	}
	handle.ready.Store(true)
	close(handle.published)
	handle.completionMu.Unlock()

	timer := time.NewTimer(time.Until(handle.pending.Deadline))
	defer timer.Stop()
	for {
		select {
		case result := <-handle.result:
			return result.answer, result.err
		case <-timer.C:
			fallback := toolspkg.ClarifyAnswer{Fallback: true}
			b.completeAutomatic(handle, toolspkg.ClarifyStatusTimedOut, fallback, nil)
		case <-ctx.Done():
			b.completeAutomatic(
				handle,
				toolspkg.ClarifyStatusCanceled,
				toolspkg.ClarifyAnswer{},
				toolspkg.ErrClarifyCanceled,
			)
		}
	}
}

func (b *clarifyBridge) Pending(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.ClarifyPending, error) {
	if ctx == nil {
		return nil, errors.New("daemon: clarification context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	normalized, err := normalizeClarifyLookupScope(scope)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	handle := b.pending[normalized.SessionID]
	b.mu.Unlock()
	if handle == nil {
		return []toolspkg.ClarifyPending{}, nil
	}
	if err := waitForClarifyPublication(ctx, handle); err != nil {
		return nil, err
	}
	handle.completionMu.Lock()
	defer handle.completionMu.Unlock()
	if handle.terminal || !handle.ready.Load() ||
		!clarifyScopeMatches(normalized, handle.pending, handle.profileID) {
		return []toolspkg.ClarifyPending{}, nil
	}
	return []toolspkg.ClarifyPending{handle.pending.Clone()}, nil
}

func (b *clarifyBridge) Answer(
	ctx context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswerResult, error) {
	if ctx == nil {
		return toolspkg.ClarifyAnswerResult{}, errors.New("daemon: clarification context is required")
	}
	normalized, err := normalizeClarifyLookupScope(scope)
	if err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	target := strings.TrimSpace(requestID)
	if target == "" {
		return toolspkg.ClarifyAnswerResult{}, errors.New("daemon: clarification request ID is required")
	}
	b.mu.Lock()
	handle := b.pending[normalized.SessionID]
	b.mu.Unlock()
	if handle == nil {
		return b.resolveOrphaned(ctx, normalized, target, request)
	}
	if err := waitForClarifyPublication(ctx, handle); err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	handle.completionMu.Lock()
	if handle.terminal ||
		!handle.ready.Load() ||
		handle.pending.RequestID != target ||
		!clarifyScopeMatches(normalized, handle.pending, handle.profileID) {
		handle.completionMu.Unlock()
		return b.resolveOrphaned(ctx, normalized, target, request)
	}
	question := toolspkg.ClarifyQuestion{
		Question: handle.pending.Question,
		Choices:  append([]string(nil), handle.pending.Choices...),
	}
	handle.completionMu.Unlock()
	answer, err := request.Normalize(question)
	if err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	if err := b.completeExplicit(ctx, handle, answer); err != nil {
		return toolspkg.ClarifyAnswerResult{}, err
	}
	return toolspkg.ClarifyAnswerResult{
		ClarifyAnswer:  answer,
		Outcome:        store.PendingInteractionOutcomeAnswered,
		RequestID:      target,
		ResolvedAnswer: answer.Resolution(question),
	}, nil
}

func (b *clarifyBridge) resolveOrphaned(
	ctx context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswerResult, error) {
	resolver, ok := b.publisher.(clarifyOrphanResolver)
	if !ok {
		return toolspkg.ClarifyAnswerResult{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, requestID)
	}
	return resolver.ResolveOrphanedClarification(ctx, scope, requestID, request)
}

func waitForClarifyPublication(ctx context.Context, handle *clarifyHandle) error {
	select {
	case <-handle.published:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *clarifyBridge) register(handle *clarifyHandle) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return toolspkg.ErrClarifyClosed
	}
	if _, exists := b.pending[handle.pending.SessionID]; exists {
		return fmt.Errorf("%w: %s", toolspkg.ErrClarifyPending, handle.pending.SessionID)
	}
	b.pending[handle.pending.SessionID] = handle
	b.waiters.Add(1)
	return nil
}

func (b *clarifyBridge) rollback(handle *clarifyHandle) {
	b.mu.Lock()
	if b.pending[handle.pending.SessionID] == handle {
		delete(b.pending, handle.pending.SessionID)
	}
	b.mu.Unlock()
}

func normalizeClarifyScope(scope toolspkg.Scope) (toolspkg.Scope, error) {
	normalized, err := normalizeClarifyLookupScope(scope)
	if err != nil {
		return toolspkg.Scope{}, err
	}
	if normalized.WorkspaceID == "" {
		return toolspkg.Scope{}, errors.New("daemon: clarification workspace ID is required")
	}
	if normalized.AgentName == "" {
		return toolspkg.Scope{}, errors.New("daemon: clarification agent name is required")
	}
	return normalized, nil
}

func normalizeClarifyLookupScope(scope toolspkg.Scope) (toolspkg.Scope, error) {
	normalized := toolspkg.Scope{
		ProfileID:   strings.TrimSpace(scope.ProfileID),
		WorkspaceID: strings.TrimSpace(scope.WorkspaceID),
		SessionID:   strings.TrimSpace(scope.SessionID),
		AgentName:   strings.TrimSpace(scope.AgentName),
		ActorKind:   strings.TrimSpace(scope.ActorKind),
		Operator:    scope.Operator,
	}
	if normalized.SessionID == "" {
		return toolspkg.Scope{}, errors.New("daemon: clarification session ID is required")
	}
	return normalized, nil
}

func clarifyScopeMatches(scope toolspkg.Scope, pending toolspkg.ClarifyPending, profileID string) bool {
	if scope.ProfileID != "" && strings.TrimSpace(scope.ProfileID) != strings.TrimSpace(profileID) {
		return false
	}
	if scope.WorkspaceID != "" && scope.WorkspaceID != pending.WorkspaceID {
		return false
	}
	return scope.AgentName == "" || scope.AgentName == pending.AgentName
}
