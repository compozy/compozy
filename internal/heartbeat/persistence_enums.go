package heartbeat

import "strings"

// ValidRevisionOperation reports whether operation is a supported revision enum member.
func ValidRevisionOperation(operation RevisionOperation) bool {
	switch operation {
	case RevisionOperationWrite, RevisionOperationDelete, RevisionOperationRollback:
		return true
	default:
		return false
	}
}

// ValidActorKind reports whether kind is a supported authoring actor enum member.
func ValidActorKind(kind ActorKind) bool {
	switch kind {
	case ActorKindUser, ActorKindAgent, ActorKindExtension, ActorKindSystem:
		return true
	default:
		return false
	}
}

// ValidSessionHealthState reports whether state is a supported session health state.
func ValidSessionHealthState(state SessionHealthState) bool {
	switch state {
	case SessionHealthStateIdle,
		SessionHealthStatePrompting,
		SessionHealthStateStopped,
		SessionHealthStateDetached:
		return true
	default:
		return false
	}
}

// ValidSessionHealthStatus reports whether health is a supported session health enum member.
func ValidSessionHealthStatus(health SessionHealthStatus) bool {
	switch health {
	case SessionHealthHealthy,
		SessionHealthDegraded,
		SessionHealthStale,
		SessionHealthDead,
		SessionHealthUnknown:
		return true
	default:
		return false
	}
}

// ValidSessionHealthIneligibilityReason reports whether reason is a supported session-health reason.
func ValidSessionHealthIneligibilityReason(reason string) bool {
	switch SessionHealthIneligibilityReason(strings.TrimSpace(reason)) {
	case SessionHealthReasonPromptActive,
		SessionHealthReasonNotAttachable,
		SessionHealthReasonUnhealthy,
		SessionHealthReasonStale,
		SessionHealthReasonHung,
		SessionHealthReasonDead,
		SessionHealthReasonUnknown:
		return true
	default:
		return false
	}
}

// ValidWakeSource reports whether source is a supported wake source enum member.
func ValidWakeSource(source WakeSource) bool {
	switch source {
	case WakeSourceScheduler, WakeSourceManual, WakeSourceHarnessReentry:
		return true
	default:
		return false
	}
}

// ValidWakeResult reports whether result is a supported wake result enum member.
func ValidWakeResult(result WakeResult) bool {
	switch result {
	case WakeResultSent,
		WakeResultSkipped,
		WakeResultCoalesced,
		WakeResultRateLimited,
		WakeResultFailed:
		return true
	default:
		return false
	}
}

// ValidWakeReason reports whether reason is a supported wake reason enum member.
func ValidWakeReason(reason WakeReason) bool {
	switch reason {
	case WakeReasonSent,
		WakeReasonHeartbeatDisabled,
		WakeReasonHeartbeatInvalid,
		WakeReasonHeartbeatNoPolicy,
		WakeReasonHeartbeatRateLimited,
		WakeReasonHeartbeatNoEligible,
		WakeReasonCooldownActive,
		WakeReasonQuietWindow,
		WakeReasonSessionNotFound,
		WakeReasonSessionUnhealthy,
		WakeReasonSessionNotAttachable,
		WakeReasonSessionPromptActive,
		WakeReasonSessionPromptRace,
		WakeReasonSyntheticPromptFailed,
		WakeReasonCoalesced:
		return true
	default:
		return false
	}
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
