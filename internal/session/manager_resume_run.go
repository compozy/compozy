package session

import (
	"context"
	"errors"
	"strings"
)

func (m *Manager) beginSessionResume(sessionID string) (*sessionResumeRun, bool, error) {
	target := strings.TrimSpace(sessionID)
	m.resumeMu.Lock()
	defer m.resumeMu.Unlock()
	if m.resumeClosing {
		return nil, false, errors.New("session: manager is shutting down")
	}
	if run := m.resumeRuns[target]; run != nil {
		return run, false, nil
	}
	run := &sessionResumeRun{done: make(chan struct{})}
	m.resumeRuns[target] = run
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
	m.resumeMu.Lock()
	run.session = session
	run.err = err
	if m.resumeRuns[strings.TrimSpace(sessionID)] == run {
		delete(m.resumeRuns, strings.TrimSpace(sessionID))
	}
	close(run.done)
	m.resumeMu.Unlock()
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
	m.resumeMu.Lock()
	m.resumeClosing = true
	m.resumeMu.Unlock()
}

func (m *Manager) waitForSessionResumes(ctx context.Context) error {
	if ctx == nil {
		return errors.New("session: wait for resumes context is required")
	}
	m.resumeMu.Lock()
	runs := make([]*sessionResumeRun, 0, len(m.resumeRuns))
	for _, run := range m.resumeRuns {
		runs = append(runs, run)
	}
	m.resumeMu.Unlock()
	for _, run := range runs {
		if _, err := waitForSessionResume(ctx, run); err != nil {
			return err
		}
	}
	return nil
}
