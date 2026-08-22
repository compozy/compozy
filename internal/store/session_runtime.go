package store

import (
	"fmt"
	"strings"
)

// SessionRuntimeStatus is the persisted lifecycle of a logical session's ACP binding.
type SessionRuntimeStatus string

const (
	SessionRuntimeUnbound       SessionRuntimeStatus = "unbound"
	SessionRuntimeBinding       SessionRuntimeStatus = "binding"
	SessionRuntimeReady         SessionRuntimeStatus = "ready"
	SessionRuntimeReconfiguring SessionRuntimeStatus = "reconfiguring"
	SessionRuntimeRecovering    SessionRuntimeStatus = "recovering"
	SessionRuntimeFailed        SessionRuntimeStatus = "failed"
)

// SessionRuntimeTransition identifies the most recent successful binding strategy.
type SessionRuntimeTransition string

const (
	SessionRuntimeTransitionNone               SessionRuntimeTransition = ""
	SessionRuntimeTransitionInitialBind        SessionRuntimeTransition = "initial_bind"
	SessionRuntimeTransitionLiveConfiguration  SessionRuntimeTransition = "live_configuration"
	SessionRuntimeTransitionProcessReplacement SessionRuntimeTransition = "process_replacement"
	SessionRuntimeTransitionAutomaticRecovery  SessionRuntimeTransition = "automatic_recovery"
)

func validateSessionRuntime(status SessionRuntimeStatus, transition SessionRuntimeTransition, failure string) error {
	switch status {
	case SessionRuntimeUnbound,
		SessionRuntimeBinding,
		SessionRuntimeReady,
		SessionRuntimeReconfiguring,
		SessionRuntimeRecovering,
		SessionRuntimeFailed:
	default:
		return fmt.Errorf("store: invalid session runtime status %q", status)
	}
	switch transition {
	case SessionRuntimeTransitionNone,
		SessionRuntimeTransitionInitialBind,
		SessionRuntimeTransitionLiveConfiguration,
		SessionRuntimeTransitionProcessReplacement,
		SessionRuntimeTransitionAutomaticRecovery:
	default:
		return fmt.Errorf("store: invalid session runtime transition %q", transition)
	}
	if status == SessionRuntimeFailed && strings.TrimSpace(string(transition)) == "" {
		return fmt.Errorf("store: failed session runtime requires a transition strategy")
	}
	if status == SessionRuntimeFailed && strings.TrimSpace(failure) == "" {
		return fmt.Errorf("store: failed session runtime requires a failure")
	}
	return nil
}

// ValidateSessionRuntime validates one durable runtime binding projection.
func ValidateSessionRuntime(status SessionRuntimeStatus, transition SessionRuntimeTransition, failure string) error {
	return validateSessionRuntime(status, transition, failure)
}

// SessionRuntimeFailurePointer normalizes an optional persisted runtime diagnostic.
func SessionRuntimeFailurePointer(value string) *string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil
	}
	return &normalized
}

// SessionRuntimeFailureValue returns the normalized runtime diagnostic value.
func SessionRuntimeFailureValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
