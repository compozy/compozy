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

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/google/uuid"
)

const clarifyObservabilityTimeout = 2 * time.Second

type clarifyEventPublisher interface {
	PublishClarifyEvent(context.Context, toolspkg.ClarifyEvent) error
}

type clarifyBridge struct {
	mu        sync.Mutex
	pending   map[string]*clarifyHandle
	timeout   time.Duration
	publisher clarifyEventPublisher
	summaries clarifySummaryWriter
	logger    *slog.Logger
	now       func() time.Time
	newID     func() string
	closed    bool
	waiters   sync.WaitGroup
}

type clarifyHandle struct {
	completionMu sync.Mutex
	pending      toolspkg.ClarifyPending
	result       chan clarifyResult
	ready        atomic.Bool
	terminal     bool
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
		result: make(chan clarifyResult, 1),
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
		handle.completionMu.Unlock()
		return toolspkg.ClarifyAnswer{}, fmt.Errorf("daemon: persist pending clarification: %w", err)
	}
	handle.ready.Store(true)
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
	if handle == nil || !handle.ready.Load() || !clarifyScopeMatches(normalized, handle.pending) {
		return []toolspkg.ClarifyPending{}, nil
	}
	return []toolspkg.ClarifyPending{handle.pending.Clone()}, nil
}

func (b *clarifyBridge) Answer(
	ctx context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswer, error) {
	if ctx == nil {
		return toolspkg.ClarifyAnswer{}, errors.New("daemon: clarification context is required")
	}
	normalized, err := normalizeClarifyLookupScope(scope)
	if err != nil {
		return toolspkg.ClarifyAnswer{}, err
	}
	target := strings.TrimSpace(requestID)
	if target == "" {
		return toolspkg.ClarifyAnswer{}, errors.New("daemon: clarification request ID is required")
	}
	b.mu.Lock()
	handle := b.pending[normalized.SessionID]
	b.mu.Unlock()
	if handle == nil ||
		!handle.ready.Load() ||
		handle.pending.RequestID != target ||
		!clarifyScopeMatches(normalized, handle.pending) {
		return toolspkg.ClarifyAnswer{}, fmt.Errorf("%w: %s", toolspkg.ErrClarifyNotFound, target)
	}
	answer, err := request.Normalize(toolspkg.ClarifyQuestion{
		Question: handle.pending.Question,
		Choices:  handle.pending.Choices,
	})
	if err != nil {
		return toolspkg.ClarifyAnswer{}, err
	}
	if err := b.completeExplicit(ctx, handle, answer); err != nil {
		return toolspkg.ClarifyAnswer{}, err
	}
	return answer, nil
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

func clarifyScopeMatches(scope toolspkg.Scope, pending toolspkg.ClarifyPending) bool {
	if scope.WorkspaceID != "" && scope.WorkspaceID != pending.WorkspaceID {
		return false
	}
	return scope.AgentName == "" || scope.AgentName == pending.AgentName
}
