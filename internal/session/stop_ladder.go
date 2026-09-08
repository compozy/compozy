package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

// StopPhase identifies the last attempted termination phase.
type StopPhase string

const (
	StopPhaseCooperative StopPhase = "cooperative"
	StopPhaseForced      StopPhase = "forced"
	StopPhaseKilled      StopPhase = "killed"
	stopForcedGrace                = 5 * time.Second
	stopKillGrace                  = 5 * time.Second
)

// StopOutcome records confirmed termination or exhausted verification.
type StopOutcome struct {
	FinalState State
	Verified   bool
	Cause      StopCause
	Escalated  bool
	Phase      StopPhase
	Elapsed    time.Duration
}

// StopContract exposes the two scopes of managed termination.
type StopContract interface {
	CancelTurn(context.Context, string, string, StopCause) error
	AwaitTurnQuiesced(context.Context, string, string) (TurnCancelOutcome, error)
	RequestStop(context.Context, string, StopCause) error
	AwaitStopped(context.Context, string) (StopOutcome, error)
}

var _ StopContract = (*Manager)(nil)

// AgentKiller is the process owner's final force-termination operation.
type AgentKiller interface {
	Kill(context.Context, *AgentProcess) error
}

// AgentCooperativeCanceler requests cancellation while preserving peer completion evidence.
type AgentCooperativeCanceler interface {
	CancelCooperatively(context.Context, *AgentProcess) error
}

func (m *Manager) cancelStopProcess(ctx context.Context, proc *AgentProcess) error {
	if canceler, ok := m.driver.(AgentCooperativeCanceler); ok {
		return canceler.CancelCooperatively(ctx, proc)
	}
	return m.driver.Cancel(ctx, proc)
}

func (m *Manager) processExitVerified(proc *AgentProcess) (bool, error) {
	if proc == nil {
		return true, nil
	}
	if verifier, ok := m.driver.(AgentExitVerifier); ok {
		return verifier.VerifyExit(proc)
	}
	return procutil.VerifyProcessExit(proc.PID, proc.StartedAt)
}

func (m *Manager) killStopProcess(ctx context.Context, proc *AgentProcess) error {
	if killer, ok := m.driver.(AgentKiller); ok {
		return killer.Kill(ctx, proc)
	}
	verified, err := procutil.VerifyProcessExit(proc.PID, proc.StartedAt)
	if err != nil || verified {
		return err
	}
	return procutil.KillProcessGroupIDAndWait(proc.PID, time.Second)
}

type terminationTarget struct {
	promptDone        <-chan struct{}
	turnOnly          bool
	cooperative       func(context.Context, *AgentProcess) error
	forced            func(context.Context, *AgentProcess) error
	kill              func(context.Context, *AgentProcess) error
	verifyExit        func(*AgentProcess) (bool, error)
	verifyExitContext func(context.Context, *AgentProcess) (bool, error)
	beforeAction      func(context.Context, StopPhase, time.Duration) error
}

type terminationResult struct {
	StopOutcome
	processExited  bool
	observationErr error
}

func (m *Manager) runTerminationLadder(
	ctx context.Context, proc *AgentProcess, target terminationTarget,
) (terminationResult, error) {
	started := m.now()
	outcome := terminationResult{StopOutcome: StopOutcome{FinalState: StateStopping, Phase: StopPhaseCooperative}}
	if proc == nil {
		outcome.Verified, outcome.processExited = true, true
		return outcome, nil
	}
	cooperative := target.cooperative
	if cooperative == nil {
		cooperative = m.cancelStopProcess
	}
	forced, kill := target.forced, target.kill
	if forced == nil {
		forced = m.driver.Stop
	}
	if kill == nil {
		kill = m.killStopProcess
	}
	phases := []struct {
		phase  StopPhase
		budget time.Duration
		action func(context.Context, *AgentProcess) error
	}{
		{StopPhaseCooperative, m.stopConfig.CooperativeGrace, cooperative},
		{StopPhaseForced, stopForcedGrace, forced},
		{StopPhaseKilled, stopKillGrace, kill},
	}
	var phaseErr error
	for _, phase := range phases {
		outcome.Phase = phase.phase
		outcome.Escalated = phase.phase != StopPhaseCooperative
		phaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), phase.budget)
		if isProcessDone(proc) || (target.turnOnly && signalClosed(target.promptDone)) {
			verified, exited, verifyErr := m.verifyTerminationTarget(phaseCtx, proc, target, phase.phase)
			if verified && verifyErr == nil {
				cancel()
				outcome.Verified, outcome.processExited = true, exited
				outcome.Elapsed = m.now().Sub(started)
				return outcome, nil
			}
		}
		if target.beforeAction != nil && phase.phase != StopPhaseCooperative {
			observeCtx, cancelObserve := context.WithTimeout(phaseCtx, time.Second)
			outcome.observationErr = errors.Join(outcome.observationErr, m.runStopAction(observeCtx, proc,
				func(ctx context.Context, _ *AgentProcess) error {
					return target.beforeAction(ctx, phase.phase, m.now().Sub(started))
				}))
			cancelObserve()
		}
		actionErr := m.runStopAction(phaseCtx, proc, phase.action)
		verified, exited, verifyErr := m.verifyTerminationTarget(phaseCtx, proc, target, phase.phase)
		if actionErr == nil && !verified && verifyErr == nil {
			waitStopPhase(phaseCtx, proc, target.promptDone, phase.phase)
			verified, exited, verifyErr = m.verifyTerminationTarget(phaseCtx, proc, target, phase.phase)
		}
		cancel()
		outcome.Elapsed = m.now().Sub(started)
		if verified && verifyErr == nil {
			outcome.Verified, outcome.processExited = true, exited
			return outcome, nil
		}
		phaseErr = errors.Join(phaseErr, actionErr, verifyErr)
	}
	return outcome, errors.Join(ErrStopVerificationFailed, phaseErr)
}

func (m *Manager) verifyTerminationTarget(
	ctx context.Context, proc *AgentProcess, target terminationTarget, phase StopPhase,
) (bool, bool, error) {
	verifyExit := target.verifyExit
	if target.verifyExitContext != nil {
		verifyExit = func(proc *AgentProcess) (bool, error) { return target.verifyExitContext(ctx, proc) }
	}
	if verifyExit == nil {
		verifyExit = m.processExitVerified
	}
	exited, err := m.verifyStopPhaseWith(ctx, proc, verifyExit)
	if err != nil || exited {
		return exited, exited, err
	}
	quiesced := target.turnOnly && phase == StopPhaseCooperative && signalClosed(target.promptDone)
	return quiesced, false, nil
}

func signalClosed(signal <-chan struct{}) bool {
	select {
	case <-signal:
		return true
	default:
		return false
	}
}

func (m *Manager) verifyStopPhase(ctx context.Context, proc *AgentProcess) (bool, error) {
	return m.verifyStopPhaseWith(ctx, proc, m.processExitVerified)
}

func (m *Manager) verifyStopPhaseWith(
	ctx context.Context, proc *AgentProcess, verifyExit func(*AgentProcess) (bool, error),
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	type proof struct {
		verified bool
		err      error
	}
	completed := make(chan proof, 1)
	m.startTrackedPromptTask(func() {
		verified, err := verifyExit(proc)
		completed <- proof{verified: verified, err: err}
	})
	select {
	case result := <-completed:
		return result.verified, result.err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (m *Manager) runStopAction(
	ctx context.Context,
	proc *AgentProcess,
	action func(context.Context, *AgentProcess) error,
) error {
	completed := make(chan error, 1)
	m.startTrackedPromptTask(func() { completed <- action(ctx, proc) })
	select {
	case err := <-completed:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func waitStopPhase(ctx context.Context, proc *AgentProcess, promptDone <-chan struct{}, phase StopPhase) {
	// With no active turn, cooperative cancellation has nothing left to drain.
	if phase == StopPhaseCooperative && promptDone == nil {
		return
	}
	if phase != StopPhaseCooperative {
		promptDone = nil
	}
	select {
	case <-proc.Done():
	case <-promptDone:
	case <-ctx.Done():
	}
}
