package session

import (
	"context"

	"github.com/compozy/compozy/internal/admission"
)

// WithWorkAdmissionChecker injects the daemon-owned new-work gate.
func WithWorkAdmissionChecker(checker admission.Checker) Option {
	return func(manager *Manager) {
		manager.workAdmission = checker
	}
}

func WithLocalExecutionChecker(check func() error) Option {
	return func(manager *Manager) { manager.localExecution = check }
}

func (m *Manager) checkSessionCreationAdmission(ctx context.Context) error {
	if err := m.checkNewWorkAdmission(ctx); err != nil {
		return err
	}
	if m.localExecution != nil {
		return m.localExecution()
	}
	return nil
}

func (m *Manager) checkNewWorkAdmission(ctx context.Context) error {
	if m == nil || m.workAdmission == nil {
		return nil
	}
	return m.workAdmission.Check(ctx)
}
