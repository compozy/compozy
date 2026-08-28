package daemon

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const (
	taskClaimHandoffReason = "task_run_handoff"
	taskClaimHandoffPrompt = "Task run %s is active in this session's bound execution target. " +
		"Begin the assigned work now. Do not claim this run again."
)

type taskClaimHandoffCoordinator interface {
	HasPending(sessionID string) bool
	ScheduleBootstrapStop(bootstrapSessionID string, runID string, targetSessionID string) error
	Activate(ctx context.Context, bootstrapSessionID string, run taskpkg.Run) error
}

type taskClaimHandoffSessions interface {
	PromptSynthetic(ctx context.Context, id string, opts session.SyntheticPromptOpts) (<-chan acp.AgentEvent, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type pendingTaskClaimHandoff struct {
	runID           string
	targetSessionID string
	stopping        bool
}

type taskClaimHandoffRuntime struct {
	drainCtx context.Context
	sessions taskClaimHandoffSessions
	logger   *slog.Logger
	workers  *ownedWorkerGroup
	mu       sync.Mutex
	pending  map[string]pendingTaskClaimHandoff
}

func newTaskClaimHandoffRuntime(
	ctx context.Context,
	sessions taskClaimHandoffSessions,
	logger *slog.Logger,
) (*taskClaimHandoffRuntime, error) {
	if ctx == nil {
		return nil, errors.New("daemon: task claim handoff context is required")
	}
	if sessions == nil {
		return nil, errors.New("daemon: task claim handoff requires sessions")
	}
	if logger == nil {
		logger = slog.Default()
	}
	drainCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	return &taskClaimHandoffRuntime{
		drainCtx: drainCtx,
		sessions: sessions,
		logger:   logger,
		workers:  newOwnedWorkerGroup(cancel),
		pending:  make(map[string]pendingTaskClaimHandoff),
	}, nil
}

func taskClaimHandoffForState(state *bootState) taskClaimHandoffCoordinator {
	if state == nil || state.tasks == nil {
		return nil
	}
	return state.tasks.claimHandoff
}

func registerTaskClaimHandoffTurnEnd(
	sessions SessionManager,
	runtime *taskClaimHandoffRuntime,
) {
	registrar, ok := sessions.(turnEndNotifierRegistrar)
	if !ok || runtime == nil {
		return
	}
	registrar.AddTurnEndNotifier(func(ctx context.Context, identity session.PromptRunIdentity) {
		runtime.onTurnEnd(ctx, identity.SessionID)
	})
}

func (r *taskClaimHandoffRuntime) HasPending(sessionID string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.pending[strings.TrimSpace(sessionID)]
	return ok
}

func (r *taskClaimHandoffRuntime) Activate(
	ctx context.Context,
	bootstrapSessionID string,
	run taskpkg.Run,
) error {
	if r == nil || r.sessions == nil {
		return errors.New("daemon: task claim handoff is not initialized")
	}
	bootstrapID := strings.TrimSpace(bootstrapSessionID)
	targetID := strings.TrimSpace(run.SessionID)
	if bootstrapID == "" || targetID == "" {
		return errors.New("daemon: task claim handoff requires bootstrap and target sessions")
	}
	if bootstrapID == targetID {
		return nil
	}

	complete, admitted := r.workers.Begin()
	if !admitted {
		return errors.New("daemon: task claim handoff is stopping")
	}
	if err := r.ScheduleBootstrapStop(bootstrapID, run.ID, targetID); err != nil {
		complete()
		return err
	}
	events, err := r.sessions.PromptSynthetic(ctx, targetID, session.SyntheticPromptOpts{
		Message: fmt.Sprintf(taskClaimHandoffPrompt, strings.TrimSpace(run.ID)),
		Metadata: acp.PromptSyntheticMeta{
			TaskID:    strings.TrimSpace(run.TaskID),
			TaskRunID: strings.TrimSpace(run.ID),
			Reason:    taskClaimHandoffReason,
			Summary:   "Task run ready for execution",
		},
		TurnID:                  taskClaimHandoffTurnID(targetID, run.ID),
		SkipIfBusy:              false,
		InterruptIfAgentWaiting: false,
	})
	if err != nil {
		r.cancelBootstrapStop(bootstrapID, run.ID, targetID)
		complete()
		return fmt.Errorf("daemon: prompt task claim target session %q: %w", targetID, err)
	}
	r.drainEvents(run, targetID, events, complete)
	return nil
}

func (r *taskClaimHandoffRuntime) cancelBootstrapStop(bootstrapSessionID, runID, targetSessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	bootstrapID := strings.TrimSpace(bootstrapSessionID)
	pending, ok := r.pending[bootstrapID]
	if ok && !pending.stopping && pending.runID == strings.TrimSpace(runID) &&
		pending.targetSessionID == strings.TrimSpace(targetSessionID) {
		delete(r.pending, bootstrapID)
	}
}

func taskClaimHandoffTurnID(sessionID string, runID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sessionID) + "\x00" + strings.TrimSpace(runID)))
	return fmt.Sprintf("task-claim-handoff-%x", digest[:16])
}

func (r *taskClaimHandoffRuntime) ScheduleBootstrapStop(
	bootstrapSessionID string,
	runID string,
	targetSessionID string,
) error {
	if r == nil {
		return errors.New("daemon: task claim handoff is not initialized")
	}
	bootstrapID := strings.TrimSpace(bootstrapSessionID)
	if bootstrapID == "" {
		return errors.New("daemon: task claim handoff requires a bootstrap session")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.pending[bootstrapID]; exists {
		return errors.New("daemon: task claim handoff already pending")
	}
	r.pending[bootstrapID] = pendingTaskClaimHandoff{
		runID:           strings.TrimSpace(runID),
		targetSessionID: strings.TrimSpace(targetSessionID),
	}
	return nil
}

func (r *taskClaimHandoffRuntime) drainEvents(
	run taskpkg.Run,
	targetSessionID string,
	events <-chan acp.AgentEvent,
	complete func(),
) {
	if events == nil {
		complete()
		return
	}
	go func() {
		defer complete()
		for {
			select {
			case <-r.drainCtx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if event.Type == acp.EventTypeError {
					r.logger.Warn(
						"task.claim_handoff.agent_error",
						"session_id", targetSessionID,
						"task_id", strings.TrimSpace(run.TaskID),
						"run_id", strings.TrimSpace(run.ID),
					)
				}
			}
		}
	}()
}

func (r *taskClaimHandoffRuntime) onTurnEnd(ctx context.Context, sessionID string) {
	if r == nil {
		return
	}
	bootstrapID := strings.TrimSpace(sessionID)
	r.mu.Lock()
	pending, ok := r.pending[bootstrapID]
	if !ok || pending.stopping {
		r.mu.Unlock()
		return
	}
	pending.stopping = true
	r.pending[bootstrapID] = pending
	r.mu.Unlock()

	detail := fmt.Sprintf("task run %q ended before bootstrap session execution", pending.runID)
	if pending.targetSessionID != "" {
		detail = fmt.Sprintf(
			"task run %q handed off to session %q",
			pending.runID,
			pending.targetSessionID,
		)
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	err := r.sessions.StopWithCause(
		stopCtx,
		bootstrapID,
		session.CauseCompleted,
		detail,
	)
	cancel()

	r.mu.Lock()
	if err == nil || errors.Is(err, session.ErrSessionNotFound) || errors.Is(err, session.ErrSessionNotActive) {
		delete(r.pending, bootstrapID)
	} else {
		pending.stopping = false
		r.pending[bootstrapID] = pending
	}
	r.mu.Unlock()
	if err != nil && !errors.Is(err, session.ErrSessionNotFound) && !errors.Is(err, session.ErrSessionNotActive) {
		r.logger.Warn(
			"task.claim_handoff.bootstrap_stop_failed",
			"session_id", bootstrapID,
			"target_session_id", pending.targetSessionID,
			"run_id", pending.runID,
			"error", err,
		)
	}
}

func (r *taskClaimHandoffRuntime) shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: task claim handoff shutdown context is required")
	}
	done := r.workers.Stop()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("daemon: shutdown task claim handoff: %w", ctx.Err())
	}
}
