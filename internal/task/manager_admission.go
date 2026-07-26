package task

import (
	"context"

	"github.com/compozy/agh/internal/admission"
)

// WithWorkAdmissionChecker injects the daemon-owned new-work gate.
func WithWorkAdmissionChecker(checker admission.Checker) Option {
	return func(options *managerOptions) {
		options.workAdmission = checker
	}
}

func (m *Service) checkNewWorkAdmission(ctx context.Context) error {
	if m == nil || m.workAdmission == nil {
		return nil
	}
	return m.workAdmission.Check(ctx)
}
