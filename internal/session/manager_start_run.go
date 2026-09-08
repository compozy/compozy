package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type sessionStartRun struct {
	ctx         context.Context
	cancel      context.CancelCauseFunc
	done        chan struct{}
	err         error
	stopOutcome *StopOutcome

	recorderReady     chan struct{}
	recorderReadyOnce sync.Once
	// launchCommit serializes immutable launch metadata with terminal state transitions.
	launchCommit chan struct{}
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

func (m *Manager) sessionStartRun(sessionID string) *sessionStartRun {
	m.startLifecycle.mu.Lock()
	defer m.startLifecycle.mu.Unlock()
	return m.startLifecycle.runs[strings.TrimSpace(sessionID)]
}

// sessionInfoForRead returns the externally visible session snapshot.
// Active is projected as Starting while its start run remains tracked.
func (m *Manager) sessionInfoForRead(session *Session) *Info {
	if session == nil {
		return nil
	}
	info := session.Info()
	if info != nil && info.State == StateActive && m.sessionStartRun(info.ID) != nil {
		info.State = StateStarting
	}
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
