package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/acp"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/gate"
	"github.com/compozy/agh/internal/session"
)

type loopJudgeExecution struct {
	cancel     context.CancelFunc
	done       chan struct{}
	cleanupErr error
	sessionID  string
	revoked    bool
}

type loopJudgeExecutionRegistry struct {
	mu      sync.Mutex
	active  map[string]*loopJudgeExecution
	pending map[string]struct{}
	revoked map[string]struct{}
}

func newLoopJudgeExecutionRegistry() *loopJudgeExecutionRegistry {
	return &loopJudgeExecutionRegistry{
		active:  make(map[string]*loopJudgeExecution),
		pending: make(map[string]struct{}),
		revoked: make(map[string]struct{}),
	}
}

func (r *loopJudgeExecutionRegistry) reserve(correlationID string) (func(), error) {
	id := strings.TrimSpace(correlationID)
	if id == "" {
		return nil, errors.New("daemon: loop judge reservation id is required")
	}
	r.mu.Lock()
	if _, exists := r.pending[id]; exists {
		r.mu.Unlock()
		return nil, fmt.Errorf("daemon: loop judge execution %q is already reserved", id)
	}
	r.pending[id] = struct{}{}
	r.mu.Unlock()

	return func() {
		r.mu.Lock()
		delete(r.pending, id)
		delete(r.revoked, id)
		r.mu.Unlock()
	}, nil
}

func (r *loopGateJudgeRunner) beginExecution(
	ctx context.Context,
	correlationID string,
) (context.Context, func(error), error) {
	id := strings.TrimSpace(correlationID)
	if id == "" {
		return ctx, func(error) {}, nil
	}
	if r.executions == nil {
		return nil, nil, errors.New("daemon: loop judge execution registry is unavailable")
	}
	return r.executions.begin(ctx, id)
}

func (r *loopJudgeExecutionRegistry) begin(
	ctx context.Context,
	id string,
) (context.Context, func(error), error) {
	judgeCtx, cancel := context.WithCancel(ctx)
	execution := &loopJudgeExecution{cancel: cancel, done: make(chan struct{})}

	r.mu.Lock()
	if _, revoked := r.revoked[id]; revoked {
		delete(r.revoked, id)
		r.mu.Unlock()
		cancel()
		return nil, nil, context.Canceled
	}
	if _, exists := r.active[id]; exists {
		r.mu.Unlock()
		cancel()
		return nil, nil, fmt.Errorf("daemon: loop judge execution %q is already active", id)
	}
	r.active[id] = execution
	r.mu.Unlock()

	finish := func(err error) {
		cancel()
		r.mu.Lock()
		if r.active[id] == execution {
			delete(r.active, id)
		}
		execution.cleanupErr = err
		close(execution.done)
		r.mu.Unlock()
	}
	return judgeCtx, finish, nil
}

func (r *loopJudgeExecutionRegistry) bind(correlationID string, sessionID string) error {
	id := strings.TrimSpace(correlationID)
	if id == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	execution := r.active[id]
	if execution == nil {
		return fmt.Errorf("daemon: loop judge execution %q is not active", id)
	}
	execution.sessionID = strings.TrimSpace(sessionID)
	if execution.revoked {
		execution.cancel()
		return context.Canceled
	}
	return nil
}

func (r *loopGateJudgeRunner) bindExecution(correlationID string, sessionID string) error {
	if strings.TrimSpace(correlationID) == "" {
		return nil
	}
	if r == nil || r.executions == nil {
		return errors.New("daemon: loop judge execution registry is unavailable")
	}
	return r.executions.bind(correlationID, sessionID)
}

func (r *loopGateJudgeRunner) revokeExecution(ctx context.Context, correlationID string) error {
	if r == nil || r.executions == nil {
		return errors.New("daemon: loop judge execution registry is unavailable")
	}
	if ctx == nil {
		return errors.New("daemon: loop judge revoke context is required")
	}
	id := strings.TrimSpace(correlationID)
	if id == "" {
		return nil
	}
	r.executions.mu.Lock()
	execution := r.executions.active[id]
	if _, pending := r.executions.pending[id]; execution == nil && pending {
		r.executions.revoked[id] = struct{}{}
	}
	if execution != nil {
		execution.revoked = true
		// Creation may already be externally visible before Judge receives the
		// session ID. Let bind finish so its deferred stop preserves the audit row.
		if strings.TrimSpace(execution.sessionID) != "" {
			execution.cancel()
		}
	}
	sessionID := ""
	if execution != nil {
		sessionID = execution.sessionID
	}
	r.executions.mu.Unlock()
	if execution == nil {
		return nil
	}
	var cancelErr error
	if sessionID != "" {
		if err := r.sessions.CancelPrompt(ctx, sessionID); err != nil {
			cancelErr = fmt.Errorf("daemon: cancel loop judge prompt %q: %w", sessionID, err)
		}
	}
	select {
	case <-execution.done:
		return errors.Join(cancelErr, execution.cleanupErr)
	case <-ctx.Done():
		execution.cancel()
		return errors.Join(cancelErr, ctx.Err())
	}
}

func collectLoopJudgeResult(
	ctx context.Context,
	sessions loopJudgeSessionManager,
	sessionID string,
	req gate.JudgeRequest,
) (looppkg.ActionPromptResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return looppkg.ActionPromptResult{}, errors.New("daemon: loop judge session id is required")
	}
	judgeMeta := acp.PromptJudgeMeta{
		Role:          acp.PromptJudgeRoleAgent,
		Attempt:       req.Attempt,
		CorrelationID: req.CorrelationID,
		GateID:        req.GateID,
		CriterionID:   req.CriterionID,
	}
	events, err := sessions.PromptWithOpts(ctx, sessionID, session.PromptOpts{
		Message:    req.Rubric,
		TurnSource: session.TurnSourceUser,
		PromptMeta: acp.PromptMeta{Judge: &judgeMeta},
	})
	if err != nil {
		return looppkg.ActionPromptResult{}, err
	}
	var text strings.Builder
	var tokensUsed int64
	tokensReported := false
	for event := range events {
		if event.Type == acp.EventTypeToolCall || event.Type == acp.EventTypeToolResult {
			activityErr := fmt.Errorf(
				"daemon: verdict-only judge attempted tool activity %q",
				strings.TrimSpace(event.ToolCallID),
			)
			if cancelErr := sessions.CancelPrompt(ctx, sessionID); cancelErr != nil {
				return looppkg.ActionPromptResult{}, errors.Join(activityErr, cancelErr)
			}
			return looppkg.ActionPromptResult{}, activityErr
		}
		if event.Type == acp.EventTypeAgentMessage && strings.TrimSpace(event.Text) != "" {
			text.WriteString(event.Text)
		}
		if tokens, ok := loopPromptTokensUsed(event.Usage); ok {
			tokensUsed = tokens
			tokensReported = true
		}
	}
	return looppkg.ActionPromptResult{
		Text: text.String(), TokensUsed: tokensUsed, TokensReported: tokensReported,
	}, nil
}
