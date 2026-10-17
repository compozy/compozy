package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// runtimeRelease marks an agent process the supervisor released from an idle session.
// The session stays active with an unbound runtime; done closes once the released
// process exit is verified, so a rebind never races the old process for its ACP session.
type runtimeRelease struct {
	acpSessionID string
	done         chan struct{}
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
		len(s.PendingInteractions) == 0 && !s.conversationRewindReserved && !s.worktreeForkReserved &&
		strings.TrimSpace(s.ACPSessionID) != "" && s.ACPCaps.SupportsLoadSession
}

// detachIdleRuntime revalidates the release under the session lock and detaches the
// process before it is stopped, so its exit is not mistaken for a session stop.
func (s *Session) detachIdleRuntime(
	progress time.Time, now time.Time,
) (*AgentProcess, *runtimeRelease, *store.SessionLivenessMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.supervisionProgressAt != progress || !s.idleRuntimeReleasableLocked() {
		return nil, nil, nil
	}
	proc := s.process
	liveness := store.CloneSessionLivenessMeta(s.Liveness)
	release := &runtimeRelease{acpSessionID: strings.TrimSpace(s.ACPSessionID), done: make(chan struct{})}
	s.process = nil
	s.runtimeRelease = release
	s.RuntimeStatus = RuntimeStatusUnbound
	s.RuntimeTransition = RuntimeTransitionNone
	s.RuntimeFailure = ""
	s.RuntimeRecovery = nil
	s.supervisionIdleSince = time.Time{}
	if s.Liveness != nil {
		s.Liveness.SubprocessPID = 0
		s.Liveness.SubprocessStartedAt = nil
		s.Liveness.StallState = ""
		s.Liveness.StallReason = ""
	}
	if !now.IsZero() {
		s.UpdatedAt = now.UTC()
	}
	return proc, release, liveness
}

// reattachIdleRuntime undoes a detach whose persistence failed, unless a prompt already
// claimed the unbound session.
func (s *Session) reattachIdleRuntime(
	proc *AgentProcess, release *runtimeRelease, liveness *store.SessionLivenessMeta,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtimeRelease != release || s.process != nil || s.isPromptingLocked() {
		return false
	}
	s.process = proc
	s.runtimeRelease = nil
	s.RuntimeStatus = RuntimeStatusReady
	s.Liveness = liveness
	return true
}

// releaseIdleRuntime stops the agent process of an idle session without stopping the
// session. Nothing user-facing changes: the session stays active, its transcript and
// ACP session id are kept, and the next prompt rebinds the runtime via session/load.
func (m *Manager) releaseIdleRuntime(ctx context.Context, session *Session, progress time.Time) error {
	proc, release, liveness := session.detachIdleRuntime(progress, m.now())
	if proc == nil {
		return nil
	}
	defer close(release.done)
	if err := m.persistSessionLifecycleState(ctx, session, false); err != nil {
		if !session.reattachIdleRuntime(proc, release, liveness) {
			err = errors.Join(err, m.stopReplacedRuntime(session, proc, true))
		}
		return fmt.Errorf("session: persist idle runtime release for %q: %w", session.ID, err)
	}
	if err := m.stopReplacedRuntime(session, proc, true); err != nil {
		return fmt.Errorf("session: stop idle runtime for %q: %w", session.ID, err)
	}
	m.sessionLogger(session).Info(
		"session.runtime.idle_released",
		"idle_release_after", m.supervision.IdleReleaseAfter,
	)
	return nil
}

// waitRuntimeRelease blocks a rebind until the released process exit is verified.
func waitRuntimeRelease(ctx context.Context, release *runtimeRelease) error {
	if release == nil {
		return nil
	}
	select {
	case <-release.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	if err := waitRuntimeRelease(ctx, snapshot.runtimeRelease); err != nil {
		return nil, err
	}
	if !runtimeSelectionsEqual(snapshot.selection, plan.selection) {
		plan.replayHistory = true
		return m.replacePromptRuntime(ctx, session, snapshot, plan)
	}
	routePlan, err := m.preparePromptRuntimePlanForRoute(ctx, session, plan.selection, snapshot.acceptedCommand)
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
