package task

import (
	"fmt"
	"strings"
)

const (
	taskRunStatusInvalid RunStatus = ^RunStatus(0)
	runKindInvalid       RunKind   = ^RunKind(0)
)

// Normalize returns the normalized representation of the task-run status.
func (s RunStatus) Normalize() RunStatus {
	switch s {
	case TaskRunStatusUnknown,
		TaskRunStatusQueued,
		TaskRunStatusClaimed,
		TaskRunStatusStarting,
		TaskRunStatusRunning,
		TaskRunStatusCompleted,
		TaskRunStatusFailed,
		TaskRunStatusCanceled,
		TaskRunStatusNeedsAttention:
		return s
	default:
		return taskRunStatusInvalid
	}
}

// Validate reports whether the task-run status is one of the supported lifecycle states.
func (s RunStatus) Validate(path string) error {
	switch s.Normalize() {
	case TaskRunStatusQueued,
		TaskRunStatusClaimed,
		TaskRunStatusStarting,
		TaskRunStatusRunning,
		TaskRunStatusCompleted,
		TaskRunStatusFailed,
		TaskRunStatusCanceled,
		TaskRunStatusNeedsAttention:
		return nil
	case TaskRunStatusUnknown:
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be one of %q, %q, %q, %q, %q, %q, %q, or %q: %q",
			ErrValidation,
			path,
			TaskRunStatusQueued.String(),
			TaskRunStatusClaimed.String(),
			TaskRunStatusStarting.String(),
			TaskRunStatusRunning.String(),
			TaskRunStatusCompleted.String(),
			TaskRunStatusFailed.String(),
			TaskRunStatusCanceled.String(),
			TaskRunStatusNeedsAttention.String(),
			enumValidationValue(s),
		)
	}
}

// ParseRunStatus returns the canonical task-run status for a durable string value.
func ParseRunStatus(value string) RunStatus {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return TaskRunStatusUnknown
	case TaskRunStatusQueued.String():
		return TaskRunStatusQueued
	case TaskRunStatusClaimed.String():
		return TaskRunStatusClaimed
	case TaskRunStatusStarting.String():
		return TaskRunStatusStarting
	case TaskRunStatusRunning.String():
		return TaskRunStatusRunning
	case TaskRunStatusCompleted.String():
		return TaskRunStatusCompleted
	case TaskRunStatusFailed.String():
		return TaskRunStatusFailed
	case TaskRunStatusCanceled.String():
		return TaskRunStatusCanceled
	case TaskRunStatusNeedsAttention.String():
		return TaskRunStatusNeedsAttention
	default:
		return taskRunStatusInvalid
	}
}

// Normalize returns the normalized representation of the task-run kind.
func (k RunKind) Normalize() RunKind {
	switch k {
	case RunKindUnknown, RunKindWorker, RunKindCoordinator, RunKindNetworkWake:
		return k
	default:
		return runKindInvalid
	}
}

// Validate reports whether the task-run kind is one of the supported executor kinds.
func (k RunKind) Validate(path string) error {
	switch k.Normalize() {
	case RunKindWorker, RunKindCoordinator, RunKindNetworkWake:
		return nil
	case RunKindUnknown:
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be %q, %q, or %q: %q",
			ErrValidation,
			path,
			RunKindWorker.String(),
			RunKindCoordinator.String(),
			RunKindNetworkWake.String(),
			enumValidationValue(k),
		)
	}
}

// ParseRunKind returns the canonical task-run kind for a durable string value.
func ParseRunKind(value string) RunKind {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return RunKindUnknown
	case RunKindWorker.String():
		return RunKindWorker
	case RunKindCoordinator.String():
		return RunKindCoordinator
	case RunKindNetworkWake.String():
		return RunKindNetworkWake
	default:
		return runKindInvalid
	}
}

func enumValidationValue(value fmt.Stringer) string {
	if display := value.String(); display != "" {
		return display
	}
	return fmt.Sprintf("%d", value)
}
