package automation

import (
	"errors"
	"strings"
)

// ErrTargetIdentityImmutable reports an attempted automation target mutation.
var ErrTargetIdentityImmutable = errors.New("automation: target identity is immutable")

// ValidateImmutableJobTarget rejects changes to the execution target selected at creation.
func ValidateImmutableJobTarget(current, next Job) error {
	return validateImmutableTargetIdentity(
		current.TargetKind,
		current.AgentName,
		current.WorkspaceID,
		current.LoopTarget,
		next.TargetKind,
		next.AgentName,
		next.WorkspaceID,
		next.LoopTarget,
	)
}

// ValidateImmutableTriggerTarget rejects changes to the execution target selected at creation.
func ValidateImmutableTriggerTarget(current, next Trigger) error {
	return validateImmutableTargetIdentity(
		current.TargetKind,
		current.AgentName,
		current.WorkspaceID,
		current.LoopTarget,
		next.TargetKind,
		next.AgentName,
		next.WorkspaceID,
		next.LoopTarget,
	)
}

func validateImmutableTargetIdentity(
	currentKind TargetKind,
	currentAgentName string,
	currentWorkspaceID string,
	currentLoopTarget *LoopTarget,
	nextKind TargetKind,
	nextAgentName string,
	nextWorkspaceID string,
	nextLoopTarget *LoopTarget,
) error {
	currentKind = targetKindOrDefault(currentKind, currentLoopTarget)
	nextKind = targetKindOrDefault(nextKind, nextLoopTarget)
	if currentKind != nextKind ||
		strings.TrimSpace(currentWorkspaceID) != strings.TrimSpace(nextWorkspaceID) {
		return ErrTargetIdentityImmutable
	}
	switch currentKind {
	case TargetKindAgent:
		if strings.TrimSpace(currentAgentName) != strings.TrimSpace(nextAgentName) {
			return ErrTargetIdentityImmutable
		}
	case TargetKindLoop:
		if currentLoopTarget == nil || nextLoopTarget == nil ||
			strings.TrimSpace(currentLoopTarget.WorkspaceID) != strings.TrimSpace(nextLoopTarget.WorkspaceID) ||
			strings.TrimSpace(currentLoopTarget.LoopName) != strings.TrimSpace(nextLoopTarget.LoopName) {
			return ErrTargetIdentityImmutable
		}
	default:
		return ErrTargetIdentityImmutable
	}
	return nil
}
