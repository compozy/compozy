package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// runtimeRelease owns a detached process after idle release or failed configuration rollback.
// The session stays active with an unbound runtime and keeps owning the released process
// until its exit is verified: done closes when the release stop returns, and err keeps
// a failed stop so neither a rebind nor a session stop treats the process as gone.
type runtimeRelease struct {
	acpSessionID string
	process      *AgentProcess
	done         chan struct{}
	mu           sync.Mutex
	err          error
}

func (r *runtimeRelease) finish(err error) {
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()
	close(r.done)
}

// unverifiedProcess waits for the release stop, which is bounded by the driver's
// stop budget, and returns the released process while its exit is unverified.
func (r *runtimeRelease) unverifiedProcess() *AgentProcess {
	if r == nil {
		return nil
	}
	<-r.done
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err == nil {
		return nil
	}
	return r.process
}

// idleRuntimeReleaseDueLocked advances the idle episode of a session that has no turn in
// flight and reports whether its runtime has been idle long enough to release. Any work
// signal, unknown source, pending operator interaction, or ineligible runtime resets the
// episode. The caller holds target.mu.
func idleRuntimeReleaseDueLocked(
	target *Session, state *SupervisionState, after time.Duration, now time.Time,
) bool {
	if after <= 0 || len(state.WorkSignals) > 0 || supervisionSourceUnknown(state) ||
		!target.idleRuntimeReleasableLocked() {
		target.supervisionIdleSince = time.Time{}
		return false
	}
	if target.supervisionIdleSince.IsZero() {
		target.supervisionIdleSince = now
		return false
	}
	return !now.Before(target.supervisionIdleSince.Add(after))
}

// idleRuntimeReleasableLocked reports a live runtime that is idle and can be restored
// natively: nothing runs or waits on the operator, no lifecycle operation holds the
// session, and the agent supports session/load so the next prompt keeps its context.
func (s *Session) idleRuntimeReleasableLocked() bool {
	return s.State == StateActive && s.process != nil && !isProcessDone(s.process) &&
		s.runtimeRelease == nil && s.stopCause == CauseNone &&
		s.RuntimeStatus == RuntimeStatusReady && !s.isPromptingLocked() &&
		len(
			s.PendingInteractions,
		) == 0 && !s.conversationRewindReserved && !s.worktreeForkReserved && !s.worktreeBindingReserved &&
		strings.TrimSpace(s.ACPSessionID) != "" && s.ACPCaps.SupportsLoadSession
}

// detachIdleRuntime revalidates the release under the session lock and detaches the
// process before it is stopped, so its exit is not mistaken for a session stop. The
// liveness keeps the process identity: until its exit is verified, the persisted
// release is in progress and boot recovery still verifies the recorded process.
func (s *Session) detachIdleRuntime(progress time.Time, now time.Time) *runtimeRelease {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.supervisionProgressAt != progress || !s.idleRuntimeReleasableLocked() {
		return nil
	}
	s.captureProcessCapabilitiesLocked()
	release := &runtimeRelease{
		acpSessionID: strings.TrimSpace(s.ACPSessionID), process: s.process, done: make(chan struct{}),
	}
	s.process = nil
	s.runtimeRelease = release
	s.RuntimeStatus = RuntimeStatusUnbound
	s.RuntimeTransition = RuntimeTransitionNone
	s.RuntimeFailure = ""
	s.RuntimeRecovery = nil
	s.supervisionIdleSince = time.Time{}
	if s.Liveness != nil {
		s.Liveness.StallState = ""
		s.Liveness.StallReason = ""
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
	return release
}

// reattachIdleRuntime undoes a detach whose persistence failed, unless a prompt or a
// stop already claimed the unbound session.
func (s *Session) reattachIdleRuntime(release *runtimeRelease) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive || s.runtimeRelease != release || s.process != nil || s.isPromptingLocked() {
		return false
	}
	s.process = release.process
	s.runtimeRelease = nil
	s.RuntimeStatus = RuntimeStatusReady
	return true
}

// clearReleasedRuntimeIdentity drops the released process identity once its exit is
// verified, completing the release, unless the session moved on meanwhile.
func (s *Session) clearReleasedRuntimeIdentity(release *runtimeRelease) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.State != StateActive || s.runtimeRelease != release || s.process != nil || s.Liveness == nil ||
		s.Liveness.SubprocessPID != release.process.PID {
		return false
	}
	s.Liveness.SubprocessPID = 0
	s.Liveness.SubprocessStartedAt = nil
	return true
}

// releaseIdleRuntime stops the agent process of an idle session without stopping the
// session. Nothing user-facing changes: the session stays active, its transcript and
// ACP session id are kept, and the next prompt rebinds the runtime via session/load.
func (m *Manager) releaseIdleRuntime(ctx context.Context, session *Session, progress time.Time) error {
	release := session.detachIdleRuntime(progress, m.now())
	if release == nil {
		return nil
	}
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		if session.reattachIdleRuntime(release) {
			cleanupCtx, cancel := m.lifecycleCleanupContext()
			err = errors.Join(err, m.persistSessionLifecycleState(cleanupCtx, session, false))
			cancel()
			release.finish(nil)
		} else {
			err = errors.Join(err, m.stopReleasedRuntime(session, release))
		}
		return fmt.Errorf("session: persist idle runtime release for %q: %w", session.ID, err)
	}
	if err := m.stopReleasedRuntime(session, release); err != nil {
		return fmt.Errorf("session: stop idle runtime for %q: %w", session.ID, err)
	}
	if session.clearReleasedRuntimeIdentity(release) {
		if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
			return fmt.Errorf("session: persist completed idle runtime release for %q: %w", session.ID, err)
		}
	}
	m.sessionLogger(session).Info(
		"session.runtime.idle_released",
		"idle_release_after", m.supervision.IdleReleaseAfter,
	)
	return nil
}

// AgentStopBudgeter is the optional driver surface reporting how long one Stop may run.
type AgentStopBudgeter interface {
	StopBudget() time.Duration
}

// runtimeStopContext grants a replaced or released process at least the driver's stop budget.
func (m *Manager) runtimeStopContext() (context.Context, context.CancelFunc) {
	budget := defaultLifecycleTimeout
	if budgeter, ok := m.driver.(AgentStopBudgeter); ok {
		budget = max(budget, budgeter.StopBudget())
	}
	return context.WithTimeout(context.WithoutCancel(m.lifecycleCtx), budget)
}

// stopReleasedRuntime stops the released process and settles the release outcome; a
// failed stop stays recorded so the process is not treated as exited.
func (m *Manager) stopReleasedRuntime(session *Session, release *runtimeRelease) error {
	err := m.stopReplacedRuntime(session, release.process, true)
	release.finish(err)
	return err
}

// waitRuntimeRelease blocks a rebind until the released process exit is verified. A
// release whose stop failed is retried here; while the old process may still run, the
// rebind fails instead of loading its ACP session into a second process.
func (m *Manager) waitRuntimeRelease(ctx context.Context, session *Session, release *runtimeRelease) error {
	if release == nil {
		return nil
	}
	select {
	case <-release.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	release.mu.Lock()
	defer release.mu.Unlock()
	if release.err == nil {
		return nil
	}
	if err := m.stopReplacedRuntime(session, release.process, true); err != nil {
		return fmt.Errorf("session: released runtime for %q has not exited: %w", session.ID, err)
	}
	release.err = nil
	return nil
}

// releasedProcessForStop joins an idle runtime release of the session, so a session
// stop never verifies an exit, finalizes, or runs stop hooks while the released process
// may still run. It returns that process when the release could not verify its exit.
func (s *Session) releasedProcessForStop() *AgentProcess {
	s.mu.RLock()
	release := s.runtimeRelease
	s.mu.RUnlock()
	return release.unverifiedProcess()
}

// rebindReleasedRuntime binds the next prompt of a session whose idle runtime was
// released. The same selection keeps the accepted seat and reloads the ACP session; a
// different selection binds the new route and carries the transcript, like a live switch.
func (m *Manager) rebindReleasedRuntime(
	ctx context.Context,
	session *Session,
	snapshot *runtimeBindingSnapshot,
	plan *promptRuntimePlan,
) (*AgentProcess, error) {
	if err := m.waitRuntimeRelease(ctx, session, snapshot.runtimeRelease); err != nil {
		return nil, err
	}
	if !runtimeSelectionsEqual(snapshot.selection, plan.selection) {
		plan.replayHistory = true
		return m.replacePromptRuntime(ctx, session, snapshot, plan)
	}
	routePlan, err := m.preparePromptRuntimePlanForRoute(
		ctx, session, plan.selection, snapshot.acceptedCommand, snapshot.acceptedSessionEnv,
	)
	if err != nil {
		return nil, err
	}
	if err := m.validateExplicitStartModel(ctx, &routePlan.runtime, &routePlan.spec); err != nil {
		return nil, err
	}
	if snapshot.acceptedRoute != nil {
		routePlan.attempt = snapshot.acceptedRoute.Attempt
	}
	routePlan.spec.fallbackAttempt = routePlan.attempt
	routePlan.reloadACPSessionID = snapshot.runtimeRelease.acpSessionID
	return m.replacePromptRuntime(ctx, session, snapshot, routePlan)
}
