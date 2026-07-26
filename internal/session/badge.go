package session

import (
	"strings"
	"time"

	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
)

// Badge is the canonical user-facing session status token.
type Badge string

const (
	BadgeRunning        Badge = "running"
	BadgeIdle           Badge = "idle"
	BadgeUnhealthy      Badge = "unhealthy"
	BadgeHung           Badge = "hung"
	BadgeWaitingForAuth Badge = "waiting-for-auth"
	BadgeStopped        Badge = "stopped"
	BadgeFailed         Badge = "failed"
	BadgeUnknown        Badge = "unknown"
)

// BadgeInputs are the runtime-truth fields used to compute a session badge.
type BadgeInputs struct {
	State               State
	HealthState         heartbeat.SessionHealthState
	Health              heartbeat.SessionHealthStatus
	Failure             *store.SessionFailure
	PendingAuth         bool
	ActivePrompt        bool
	Stalled             bool
	IneligibilityReason string
}

// CanonicalBadge collapses runtime state, health, and failure classification into
// the stable eight-token badge vocabulary used by API, CLI, and web clients.
func CanonicalBadge(input BadgeInputs) Badge {
	failure := store.CloneSessionFailure(input.Failure)
	terminal := input.State == StateStopped || input.HealthState == heartbeat.SessionHealthStateStopped
	if terminal && terminalFailureKind(failure) {
		return BadgeFailed
	}
	if terminal {
		return BadgeStopped
	}
	if input.PendingAuth || failureKindIsAuth(failure) {
		return BadgeWaitingForAuth
	}
	if input.Stalled {
		return BadgeHung
	}
	if input.IneligibilityReason == string(heartbeat.SessionHealthReasonHung) ||
		input.Health == heartbeat.SessionHealthDead ||
		input.Health == heartbeat.SessionHealthStale {
		return BadgeHung
	}
	if input.IneligibilityReason == string(heartbeat.SessionHealthReasonUnhealthy) ||
		input.Health == heartbeat.SessionHealthDegraded {
		return BadgeUnhealthy
	}
	if input.State == StateStarting || input.State == StateStopping || input.ActivePrompt ||
		input.HealthState == heartbeat.SessionHealthStatePrompting {
		return BadgeRunning
	}
	if input.State == StateActive || input.HealthState == heartbeat.SessionHealthStateIdle ||
		input.HealthState == heartbeat.SessionHealthStateDetached {
		return BadgeIdle
	}
	return BadgeUnknown
}

func terminalFailureKind(failure *store.SessionFailure) bool {
	if failure == nil {
		return false
	}
	kind := failure.Normalize().Kind
	return kind != "" && kind != store.FailureCanceled
}

// BadgeForInfo computes a badge from the session manager/catalog snapshot.
func BadgeForInfo(info *Info) Badge {
	if info == nil {
		return BadgeUnknown
	}
	return CanonicalBadge(BadgeInputs{
		State:       info.State,
		Failure:     info.Failure,
		PendingAuth: infoFailureNeedsAuth(info.Failure),
		Stalled:     infoHasDetectedStall(info),
		ActivePrompt: info.Liveness != nil &&
			info.Liveness.Activity != nil &&
			strings.TrimSpace(info.Liveness.Activity.TurnID) != "",
	})
}

// AttachableForInfo computes whether a session snapshot is eligible for an
// explicit attach without consulting UI state or spawning a new runtime.
func AttachableForInfo(info *Info, now time.Time) bool {
	if info == nil || info.State != StateActive {
		return false
	}
	switch BadgeForInfo(info) {
	case BadgeHung, BadgeFailed, BadgeStopped, BadgeUnknown:
		return false
	}
	if info.Failure != nil && strings.TrimSpace(string(info.Failure.Kind)) != "" {
		return false
	}
	if infoHasDetectedStall(info) {
		return false
	}
	if strings.TrimSpace(info.AttachedTo) == "" || info.AttachExpiresAt == nil {
		return true
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !info.AttachExpiresAt.After(now.UTC())
}

func infoHasDetectedStall(info *Info) bool {
	return info != nil &&
		info.Liveness != nil &&
		strings.TrimSpace(info.Liveness.StallState) == store.SessionStallStateDetected
}

// BadgeForHealth computes a badge from an explicit health row, preserving
// session/failure precedence from the base session info when supplied.
func BadgeForHealth(info *Info, health heartbeat.SessionHealth) Badge {
	health = health.Normalize()
	state := State("")
	failure := (*store.SessionFailure)(nil)
	if info != nil {
		state = info.State
		failure = info.Failure
	}
	return CanonicalBadge(BadgeInputs{
		State:               state,
		HealthState:         health.State,
		Health:              health.Health,
		Failure:             failure,
		PendingAuth:         infoFailureNeedsAuth(failure),
		ActivePrompt:        health.ActivePrompt,
		IneligibilityReason: health.IneligibilityReason,
	})
}

func failureKindIsAuth(failure *store.SessionFailure) bool {
	if failure == nil {
		return false
	}
	return failure.Normalize().Kind == store.FailureProviderAuth ||
		failure.Normalize().Kind == store.FailurePermission
}

func infoFailureNeedsAuth(failure *store.SessionFailure) bool {
	return failureKindIsAuth(failure)
}
