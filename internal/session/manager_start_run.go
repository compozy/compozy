package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

type sessionStartRun struct {
	ctx                      context.Context
	cancel                   context.CancelCauseFunc
	done                     chan struct{}
	err                      error
	stopOutcome              *StopOutcome
	stopForcedRecorded       atomic.Bool
	stopKilledRecorded       atomic.Bool
	stopVerificationRecorded atomic.Bool

	recorderReady     chan struct{}
	recorderReadyOnce sync.Once
	// launchCommit serializes immutable launch metadata with terminal state transitions.
	launchCommit chan struct{}
	// After durable activation, normal stop finalization owns termination.
	activationCommitted atomic.Bool
	// stopJoined records a Stop waiting for retained cleanup without changing its failure outcome.
	stopJoined        atomic.Bool
	stopRequested     chan struct{}
	stopRequestedOnce sync.Once
}

func (m *Manager) newSessionStartRun(base context.Context, sessionID string) (*sessionStartRun, error) {
	if base == nil {
		return nil, errors.New("session: start run context is required")
	}
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil, errors.New("session: start run id is required")
	}
	ctx, cancel := context.WithCancelCause(base)
	run := &sessionStartRun{
		ctx:           ctx,
		cancel:        cancel,
		done:          make(chan struct{}),
		stopRequested: make(chan struct{}),
		recorderReady: make(chan struct{}),
		launchCommit:  make(chan struct{}, 1),
	}
	run.launchCommit <- struct{}{}

	m.startLifecycle.mu.Lock()
	defer m.startLifecycle.mu.Unlock()
	if m.startLifecycle.closing {
		cancel(errors.New("session: manager is shutting down"))
		return nil, errors.New("session: manager is shutting down")
	}
	if _, exists := m.startLifecycle.runs[target]; exists {
		cancel(fmt.Errorf("session: start run %q already exists", target))
		return nil, fmt.Errorf("session: start run %q already exists", target)
	}
	m.startLifecycle.wg.Add(1)
	m.startLifecycle.runs[target] = run
	return run, nil
}

func (m *Manager) finishSessionStartRun(sessionID string, run *sessionStartRun, err error) {
	if run == nil {
		return
	}
	run.err = err
	run.signalRecorderReady()
	m.startLifecycle.mu.Lock()
	if m.startLifecycle.runs[strings.TrimSpace(sessionID)] == run {
		delete(m.startLifecycle.runs, strings.TrimSpace(sessionID))
	}
	close(run.done)
	m.startLifecycle.mu.Unlock()
	m.startLifecycle.wg.Done()
}

// markStopRequested wakes in-flight cleanup only after a Stop intent has been accepted.
func (r *sessionStartRun) markStopRequested() {
	r.stopJoined.Store(true)
	r.stopRequestedOnce.Do(func() { close(r.stopRequested) })
}

func (r *sessionStartRun) signalRecorderReady() {
	if r == nil {
		return
	}
	r.recorderReadyOnce.Do(func() { close(r.recorderReady) })
}

func waitForSessionStartRecorder(ctx context.Context, run *sessionStartRun) error {
	if run == nil {
		return nil
	}
	select {
	case <-run.recorderReady:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func acquireSessionStartLaunchCommit(
	ctx context.Context,
	run *sessionStartRun,
) (func(), error) {
	if run == nil {
		return func() {}, nil
	}
	select {
	case <-run.launchCommit:
		return sync.OnceFunc(func() { run.launchCommit <- struct{}{} }), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// commitSessionStartActivation transfers stop ownership before post-activation hooks.
// The launch gate orders this transfer against a stop still canceling startup.
func (m *Manager) commitSessionStartActivation(ctx context.Context, session *Session) error {
	run := m.sessionStartRun(session.ID)
	if run == nil {
		return nil
	}
	release, err := acquireSessionStartLaunchCommit(ctx, run)
	if err != nil {
		return err
	}
	defer release()
	if session.stopWasRequested() {
		return errStartupCanceledByStop
	}
	run.activationCommitted.Store(true)
	return nil
}

func (m *Manager) sessionStartRun(sessionID string) *sessionStartRun {
	m.startLifecycle.mu.Lock()
	defer m.startLifecycle.mu.Unlock()
	return m.startLifecycle.runs[strings.TrimSpace(sessionID)]
}

// sessionInfoForRead returns the externally visible session snapshot.
// Activation and retained failure settlement project Starting until their owner finishes.
func (m *Manager) sessionInfoForRead(session *Session) *Info {
	if session == nil {
		return nil
	}
	info := session.Info()
	m.projectPendingSessionStart(info)
	return info
}

func waitForSessionStartRun(ctx context.Context, run *sessionStartRun) error {
	if run == nil {
		return nil
	}
	select {
	case <-run.done:
		return run.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// projectPendingSessionStart keeps status and catalog filtering on the same owner snapshot.
func (m *Manager) projectPendingSessionStart(info *Info) {
	if info == nil {
		return
	}
	if run := m.sessionStartRun(info.ID); run != nil &&
		(info.State == StateActive || (info.State == StateStopped && !run.activationCommitted.Load())) {
		info.State = StateStarting
	}
}

// notifyActivatedSession orders creation against terminal notification, including
// a Stop that finishes while post-activation hooks are still running.
func (m *Manager) notifyActivatedSession(ctx context.Context, session *Session) {
	session.lifecycleNotifyMu.Lock()
	defer session.lifecycleNotifyMu.Unlock()
	if m.notifier != nil && session.Info().State == StateActive {
		m.notifier.OnSessionCreated(ctx, session)
	}
}

func (m *Manager) notifyStoppedSession(ctx context.Context, session *Session) {
	session.lifecycleNotifyMu.Lock()
	defer session.lifecycleNotifyMu.Unlock()
	if m.notifier != nil {
		m.notifier.OnSessionStopped(ctx, session)
	}
}
