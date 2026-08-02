package session

import (
	"context"
	"errors"
	"strings"
)

func (m *Manager) beginSessionResume(sessionID string) (*sessionResumeRun, bool, error) {
	target := strings.TrimSpace(sessionID)
	m.resumeLifecycle.mu.Lock()
	defer m.resumeLifecycle.mu.Unlock()
	if m.resumeLifecycle.closing {
		return nil, false, errors.New("session: manager is shutting down")
	}
	if run := m.resumeLifecycle.runs[target]; run != nil {
		return run, false, nil
	}
	run := &sessionResumeRun{done: make(chan struct{})}
	m.resumeLifecycle.runs[target] = run
	return run, true, nil
}

func (m *Manager) finishSessionResume(
	sessionID string,
	run *sessionResumeRun,
	session *Session,
	err error,
) {
	if run == nil {
		return
	}
	m.resumeLifecycle.mu.Lock()
	run.session = session
	run.err = err
	if m.resumeLifecycle.runs[strings.TrimSpace(sessionID)] == run {
		delete(m.resumeLifecycle.runs, strings.TrimSpace(sessionID))
	}
	close(run.done)
	m.resumeLifecycle.mu.Unlock()
}

func waitForSessionResume(ctx context.Context, run *sessionResumeRun) (*Session, error) {
	if run == nil {
		return nil, errors.New("session: resume run is required")
	}
	select {
	case <-run.done:
		return run.session, run.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Manager) closeSessionResumes() {
	m.resumeLifecycle.mu.Lock()
	m.resumeLifecycle.closing = true
	m.resumeLifecycle.mu.Unlock()
}

func (m *Manager) waitForSessionResumes(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session: wait for resumes context is required")
	}
	m.resumeLifecycle.mu.Lock()
	runs := make([]*sessionResumeRun, 0, len(m.resumeLifecycle.runs))
	for _, run := range m.resumeLifecycle.runs {
		runs = append(runs, run)
	}
	m.resumeLifecycle.mu.Unlock()
	for _, run := range runs {
		select {
		case <-run.done:
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
