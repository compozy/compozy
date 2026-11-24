package sshsetup

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/account"
)

var errCanceled = errors.New("setup canceled by operator")

func (m *Manager) Status(ctx context.Context, id string, wait time.Duration) (Status, error) {
	identity, installation, err := m.identity()
	if err != nil {
		return Status{}, err
	}
	m.mu.Lock()
	job := m.runs[id]
	if job == nil || job.identity != identity {
		row, err := m.load(ctx, installation.AccountID, id)
		m.mu.Unlock()
		return storedStatus(row), cmp.Or(identity.Err(), err)
	}
	status, update := job.status, job.changed
	m.mu.Unlock()
	if wait > 0 && status.State == Running {
		timer := time.NewTimer(min(wait, MaxStatusWait))
		defer timer.Stop()
		select {
		case <-update:
			m.mu.Lock()
			status = job.status
			m.mu.Unlock()
		case <-timer.C:
		case <-ctx.Done():
			return Status{}, ctx.Err()
		case <-identity.Done():
			return Status{}, account.ErrSignInRequired
		}
	}
	return status, identity.Err()
}

func (m *Manager) Answer(ctx context.Context, id string, answer Answer, operator bool) error {
	if answer.Kind == Secret && !operator {
		return ErrSecretOperatorOnly
	}
	identity, installation, err := m.identity()
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := cmp.Or(ctx.Err(), identity.Err()); err != nil {
		return err
	}
	job := m.runs[id]
	if job == nil || job.identity != identity {
		_, err := m.load(ctx, installation.AccountID, id)
		return cmp.Or(err, error(ErrNoPrompt))
	}
	if job.answer == nil {
		return ErrNoPrompt
	}
	if err := job.answer(answer); err != nil {
		return err
	}
	job.answer = nil
	status := job.status
	status.Prompt, status.State, status.Answered = nil, Running, true
	m.publish(job, status)
	return nil
}

func (m *Manager) waitForAnswer(
	ctx context.Context, job *setupRun, state string, prompt Prompt, accept func(Answer) error,
) error {
	answered := make(chan struct{})
	m.mu.Lock()
	if err := ctx.Err(); err != nil {
		m.mu.Unlock()
		return err
	}
	if job.answer != nil {
		m.mu.Unlock()
		return ErrNoPrompt
	}
	job.answer = func(answer Answer) error {
		if err := accept(answer); err != nil {
			return err
		}
		close(answered)
		return nil
	}
	status := job.status
	status.Prompt, status.State, status.Answered = &prompt, state, false
	m.publish(job, status)
	m.mu.Unlock()
	select {
	case <-answered:
		return nil
	case <-ctx.Done():
		m.mu.Lock()
		select {
		case <-answered:
		default:
			job.answer = nil
		}
		m.mu.Unlock()
		return ctx.Err()
	}
}

func (m *Manager) trustPrompt(ctx context.Context, job *setupRun, t target, fingerprint string) error {
	trusted := false
	err := m.waitForAnswer(ctx, job, NeedsInput, Prompt{Kind: TrustHost, Host: t.address(), Fingerprint: fingerprint},
		func(answer Answer) error {
			if answer.Kind != TrustHost {
				return ErrNoPrompt
			}
			trusted = answer.Accept
			return nil
		})
	if err == nil && !trusted {
		return errDeclined
	}
	return err
}

func (m *Manager) ask(job *setupRun) askpassPrompt {
	return func(ctx context.Context, kind string, write func(string) error) error {
		return m.waitForAnswer(ctx, job, NeedsOperatorSecret, Prompt{Kind: kind}, func(answer Answer) error {
			if answer.Kind != Secret {
				return ErrNoPrompt
			}
			if len(answer.Value) == 0 || len(answer.Value) > maxAnswer ||
				strings.ContainsAny(answer.Value, "\x00\n\r") {
				return failure(http.StatusBadRequest, "invalid_computer_request", "Enter a password or code.")
			}
			return write(answer.Value)
		})
	}
}

func (m *Manager) forwardPrompt(ctx context.Context, address string) askpassPrompt {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, job := range m.runs {
		if job.row.HostKey == address && job.ctx.Err() == nil && job.status.Step == StepForward && ctx.Err() == nil {
			return func(ctx context.Context, kind string, write func(string) error) error {
				prompt, cancel := context.WithCancel(ctx)
				defer cancel()
				stop := context.AfterFunc(job.ctx, cancel)
				defer stop()
				return m.ask(job)(prompt, kind, write)
			}
		}
	}
	return nil
}
