package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/events"
)

// TurnCancelOutcome describes verified turn quiescence and any escalation.
type TurnCancelOutcome struct {
	TurnID    string
	Quiesced  bool
	Escalated bool
	Phase     StopPhase
	Elapsed   time.Duration
}

type turnStopRun struct {
	session    *Session
	process    *AgentProcess
	turnID     string
	promptDone <-chan struct{}
	cancel     context.CancelFunc
	setup      bool
	done       chan struct{}
	outcome    TurnCancelOutcome
	err        error
}

// CancelTurn requests cancellation of the current turn and preserves the session.
func (m *Manager) CancelTurn(ctx context.Context, id, expectedTurn string, cause StopCause) error {
	_, err := m.requestTurnStop(ctx, id, expectedTurn, cause)
	return err
}

func (m *Manager) requestTurnStop(ctx context.Context, id, expectedTurn string, cause StopCause) (*turnStopRun, error) {
	if m == nil || ctx == nil {
		return nil, errors.New("session: turn cancellation requires manager and context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	session, ok := m.Get(strings.TrimSpace(id))
	if !ok {
		return nil, ErrSessionNotFound
	}
	run, owned, err := session.claimTurnStop(strings.TrimSpace(expectedTurn))
	if err != nil || !owned {
		return run, err
	}
	m.mu.Lock()
	if m.turnStopRuns == nil {
		m.turnStopRuns = make(map[string]*turnStopRun)
	}
	m.turnStopRuns[session.ID] = run
	m.mu.Unlock()
	if run.setup && run.cancel != nil {
		run.cancel()
	}
	m.emitPromptCancelMarker(ctx, session, run.turnID)
	m.startTrackedPromptTask(func() { m.executeTurnStop(context.WithoutCancel(ctx), run, cause) })
	return run, nil
}

// AwaitTurnQuiesced joins the shared cancellation independently of its request context.
func (m *Manager) AwaitTurnQuiesced(ctx context.Context, id, turnID string) (TurnCancelOutcome, error) {
	if m == nil || ctx == nil {
		return TurnCancelOutcome{}, errors.New("session: turn wait requires manager and context")
	}
	m.mu.RLock()
	run := m.turnStopRuns[strings.TrimSpace(id)]
	active := m.sessions[strings.TrimSpace(id)]
	m.mu.RUnlock()
	if active != nil {
		active.mu.RLock()
		run = active.turnStop
		active.mu.RUnlock()
	}
	if run == nil || run.turnID != strings.TrimSpace(turnID) {
		return TurnCancelOutcome{}, ErrPromptNotInProgress
	}
	select {
	case <-run.done:
		return run.outcome, run.err
	case <-ctx.Done():
		return TurnCancelOutcome{}, ctx.Err()
	}
}

func (s *Session) claimTurnStop(expected string) (*turnStopRun, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive {
		return nil, false, ErrSessionNotActive
	}
	if previous := s.turnStop; previous != nil && !signalClosed(previous.done) {
		if expected != "" && expected != previous.turnID {
			return nil, false, ErrActiveTurnMismatch
		}
		return previous, false, nil
	}
	if s.currentTurnID == "" {
		return nil, false, ErrPromptNotInProgress
	}
	if expected != "" && expected != s.currentTurnID {
		return nil, false, ErrActiveTurnMismatch
	}
	run := &turnStopRun{
		session: s, process: s.process, turnID: s.currentTurnID,
		promptDone: s.currentPromptDone, cancel: s.currentPromptCancel,
		setup: s.promptSetupCount > 0, done: make(chan struct{}),
	}
	s.turnStop, s.turnStopPending = run, true
	s.promptCancelRequested, s.currentPromptCancelTurn = true, run.turnID
	return run, true, nil
}

func (m *Manager) executeTurnStop(ctx context.Context, run *turnStopRun, cause StopCause) {
	result, err := m.runTerminationLadder(ctx, run.process, terminationTarget{
		promptDone: run.promptDone, turnOnly: true,
		beforeAction: func(ctx context.Context, phase StopPhase, elapsed time.Duration) error {
			return m.recordStopEvent(ctx, run.session, events.SessionStopEscalated, stopEventPayload{
				Scope: "turn", TurnID: run.turnID, Phase: phase, ElapsedMS: elapsed.Milliseconds(), Cause: cause,
			})
		},
		cooperative: func(ctx context.Context, proc *AgentProcess) error {
			cancelErr := m.cancelStopProcess(ctx, proc)
			if scoped, ok := m.driver.(ScopedInterrupter); ok {
				_, interruptErr := scoped.Interrupt(ctx, run.session.ID, run.turnID)
				if !errors.Is(interruptErr, ErrScopedInterruptNotFound) {
					cancelErr = errors.Join(cancelErr, interruptErr)
				}
			}
			return cancelErr
		},
	})
	run.outcome = TurnCancelOutcome{
		TurnID: run.turnID, Quiesced: result.Verified, Escalated: result.Escalated,
		Phase: result.Phase, Elapsed: result.Elapsed,
	}
	if result.Verified && result.processExited {
		err = errors.Join(err, m.rebindCanceledTurn(run))
	}
	if err != nil {
		run.session.setStopCause(cause)
		run.session.recordAutomaticRecoveryFailure(err, m.now())
		err = errors.Join(err, m.beginStoppingSession(ctx, run.session))
	}
	if !result.Verified {
		err = errors.Join(err, m.recordStopVerificationFailure(ctx, run.session, stopEventPayload{
			Scope:     "turn",
			TurnID:    run.turnID,
			Phase:     result.Phase,
			ElapsedMS: result.Elapsed.Milliseconds(),
			Cause:     cause,
		}))
	}
	run.err = errors.Join(err, result.observationErr)
	run.session.mu.Lock()
	if run.session.turnStop == run {
		run.session.turnStopPending = err != nil
	}
	close(run.done)
	run.session.mu.Unlock()
	if err == nil {
		m.startNextQueuedInputPrompt(run.session.ID)
		m.startNextQueuedSyntheticPrompt(run.session.ID)
	}
}

func (m *Manager) rebindCanceledTurn(run *turnStopRun) error {
	if run.cancel != nil {
		run.cancel()
	}
	cleanupCtx, cancel := m.lifecycleCleanupContext()
	defer cancel()
	if run.promptDone != nil {
		select {
		case <-run.promptDone:
		case <-cleanupCtx.Done():
			return fmt.Errorf("session: canceled turn drain: %w", cleanupCtx.Err())
		}
	}
	if run.process == nil || run.session.Info().State != StateActive || !run.session.isCurrentProcess(run.process) {
		return nil
	}
	_, _, err := m.recoverPromptRuntime(cleanupCtx, run.session)
	if errors.Is(err, ErrSessionNotActive) && run.session.Info().State != StateActive {
		return nil
	}
	return err
}

func (s *Session) pendingTurnStop(proc *AgentProcess) <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.turnStop != nil && s.turnStop.process == proc && !signalClosed(s.turnStop.done) {
		return s.turnStop.done
	}
	return nil
}

func (s *Session) ownsTurnStop(turnID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.turnStop != nil && s.turnStop.turnID == turnID && !signalClosed(s.turnStop.done)
}
