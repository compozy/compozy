package task

import (
	"fmt"

	"strings"
)

const maxCapabilityIDLength = 128

// Normalize returns the normalized representation of the scope.
func (s Scope) Normalize() Scope {
	return Scope(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the scope is one of the supported task scope values.
func (s Scope) Validate(path string) error {
	switch s.Normalize() {
	case ScopeGlobal, ScopeWorkspace:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be %q or %q: %q",
			ErrValidation,
			path,
			ScopeGlobal,
			ScopeWorkspace,
			s,
		)
	}
}

// Normalize returns the normalized representation of the task status.
func (s Status) Normalize() Status {
	return Status(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the task status is one of the supported lifecycle states.
func (s Status) Validate(path string) error {
	switch s.Normalize() {
	case TaskStatusDraft,
		TaskStatusPending,
		TaskStatusBlocked,
		TaskStatusNeedsAttention,
		TaskStatusReady,
		TaskStatusInProgress,
		TaskStatusCompleted,
		TaskStatusFailed,
		TaskStatusCanceled:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be one of %q, %q, %q, %q, %q, %q, %q, %q, or %q: %q",
			ErrValidation,
			path,
			TaskStatusDraft,
			TaskStatusPending,
			TaskStatusBlocked,
			TaskStatusNeedsAttention,
			TaskStatusReady,
			TaskStatusInProgress,
			TaskStatusCompleted,
			TaskStatusFailed,
			TaskStatusCanceled,
			s,
		)
	}
}

// Normalize returns the normalized representation of the task priority.
func (p Priority) Normalize() Priority {
	return Priority(strings.ToLower(strings.TrimSpace(string(p))))
}

// Validate reports whether the task priority is one of the supported values.
func (p Priority) Validate(path string) error {
	switch p.Normalize() {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be one of %q, %q, %q, or %q: %q",
			ErrValidation,
			path,
			PriorityLow,
			PriorityMedium,
			PriorityHigh,
			PriorityUrgent,
			p,
		)
	}
}

// Normalize returns the normalized representation of the approval policy.
func (p ApprovalPolicy) Normalize() ApprovalPolicy {
	return ApprovalPolicy(strings.ToLower(strings.TrimSpace(string(p))))
}

// Validate reports whether the approval policy is one of the supported values.
func (p ApprovalPolicy) Validate(path string) error {
	switch p.Normalize() {
	case ApprovalPolicyNone, ApprovalPolicyManual:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be %q or %q: %q",
			ErrValidation,
			path,
			ApprovalPolicyNone,
			ApprovalPolicyManual,
			p,
		)
	}
}

// Normalize returns the normalized representation of the approval state.
func (s ApprovalState) Normalize() ApprovalState {
	return ApprovalState(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the approval state is one of the supported values.
func (s ApprovalState) Validate(path string) error {
	switch s.Normalize() {
	case ApprovalStateNotRequired,
		ApprovalStatePending,
		ApprovalStateApproved,
		ApprovalStateRejected:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be one of %q, %q, %q, or %q: %q",
			ErrValidation,
			path,
			ApprovalStateNotRequired,
			ApprovalStatePending,
			ApprovalStateApproved,
			ApprovalStateRejected,
			s,
		)
	}
}

// Normalize returns the normalized representation of the actor kind.
func (k ActorKind) Normalize() ActorKind {
	return ActorKind(strings.ToLower(strings.TrimSpace(string(k))))
}

// Validate reports whether the actor kind is supported.
func (k ActorKind) Validate(path string) error {
	switch k.Normalize() {
	case ActorKindHuman,
		ActorKindAgentSession,
		ActorKindAutomation,
		ActorKindExtension,
		ActorKindNetworkPeer,
		ActorKindDaemon:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, k)
	}
}

// Normalize returns the normalized representation of the owner kind.
func (k OwnerKind) Normalize() OwnerKind {
	return OwnerKind(strings.ToLower(strings.TrimSpace(string(k))))
}

// Validate reports whether the owner kind is supported.
func (k OwnerKind) Validate(path string) error {
	switch k.Normalize() {
	case OwnerKindHuman,
		OwnerKindAgentSession,
		OwnerKindAutomation,
		OwnerKindExtension,
		OwnerKindNetworkPeer,
		OwnerKindPool:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, k)
	}
}

// Normalize returns the normalized representation of the origin kind.
func (k OriginKind) Normalize() OriginKind {
	return OriginKind(strings.ToLower(strings.TrimSpace(string(k))))
}

// Validate reports whether the origin kind is supported.
func (k OriginKind) Validate(path string) error {
	switch k.Normalize() {
	case OriginKindCLI,
		OriginKindWeb,
		OriginKindUDS,
		OriginKindHTTP,
		OriginKindAutomation,
		OriginKindExtension,
		OriginKindNetwork,
		OriginKindAgentSession,
		OriginKindDaemon:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, k)
	}
}

// Normalize returns the normalized representation of the dependency kind.
func (k DependencyKind) Normalize() DependencyKind {
	return DependencyKind(strings.ToLower(strings.TrimSpace(string(k))))
}

// Validate reports whether the dependency kind is supported.
func (k DependencyKind) Validate(path string) error {
	switch k.Normalize() {
	case DependencyKindBlocks:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, k)
	}
}

// Normalize returns the normalized representation of the task-block kind.
func (k BlockKind) Normalize() BlockKind {
	return BlockKind(strings.ToLower(strings.TrimSpace(string(k))))
}

// Validate reports whether the task-block kind is supported.
func (k BlockKind) Validate(path string) error {
	switch k.Normalize() {
	case BlockKindNeedsInput, BlockKindCapability, BlockKindTransient:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf(
			"%w: %s must be one of %q, %q, or %q: %q",
			ErrValidation,
			path,
			BlockKindNeedsInput,
			BlockKindCapability,
			BlockKindTransient,
			k,
		)
	}
}

// Normalize returns the normalized representation of the blocked-reason source.
func (s BlockedSource) Normalize() BlockedSource {
	return BlockedSource(strings.ToLower(strings.TrimSpace(string(s))))
}

// Validate reports whether the blocked-reason source is supported.
func (s BlockedSource) Validate(path string) error {
	switch s.Normalize() {
	case BlockedSourceDependency, BlockedSourceApproval, BlockedSourcePaused, BlockedSourceBlock:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, s)
	}
}

// Normalize returns the normalized representation of the session stop reason.
func (r StopReason) Normalize() StopReason {
	return StopReason(strings.ToLower(strings.TrimSpace(string(r))))
}

// Validate reports whether the stop reason is supported.
func (r StopReason) Validate(path string) error {
	switch r.Normalize() {
	case StopReasonCompleted,
		StopReasonFailed,
		StopReasonCancellation,
		StopReasonShutdown,
		StopReasonOrphanedRun:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, r)
	}
}

// Normalize returns the normalized representation of the boot-recovery action.
func (a RunBootRecoveryAction) Normalize() RunBootRecoveryAction {
	return RunBootRecoveryAction(strings.ToLower(strings.TrimSpace(string(a))))
}

// Validate reports whether the boot-recovery action is supported.
func (a RunBootRecoveryAction) Validate(path string) error {
	switch a.Normalize() {
	case RunBootRecoveryRequeue, RunBootRecoveryMarkRunning, RunBootRecoveryFail:
		return nil
	case "":
		return fmt.Errorf("%w: %s is required", ErrValidation, path)
	default:
		return fmt.Errorf("%w: %s has unsupported value %q", ErrValidation, path, a)
	}
}
