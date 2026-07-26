package task

import (
	"fmt"
	"strings"
)

// WithCoordinatorTerminalStatusValidator injects the owner-defined loop-run status vocabulary.
func WithCoordinatorTerminalStatusValidator(validator func(string) bool) Option {
	return func(opts *managerOptions) {
		opts.coordinatorStatusOK = validator
	}
}

// WithCoordinatorTerminalHookStatusValidator injects the subset that should emit loop terminal hooks.
func WithCoordinatorTerminalHookStatusValidator(validator func(string) bool) Option {
	return func(opts *managerOptions) {
		opts.coordinatorHookOK = validator
	}
}

func (m *Service) validateCoordinatorPlan(plan CoordinatorCompletionPlan, path string) error {
	if err := plan.Validate(path); err != nil {
		return err
	}
	if plan.Terminal == nil {
		return nil
	}
	status := strings.TrimSpace(plan.Terminal.Status)
	if m.coordinatorStatusOK == nil || !m.coordinatorStatusOK(status) {
		return fmt.Errorf(
			"%w: %s must be accepted by the coordinator owner: %q",
			ErrValidation,
			nestedPath(nestedPath(path, "terminal"), "status"),
			plan.Terminal.Status,
		)
	}
	return nil
}
