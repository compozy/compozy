package scheduler

import (
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

const (
	schedulerActiveKey   = "active"
	schedulerStartingKey = "starting"

	capacityReasonOccupied          = "occupied"
	capacityReasonCapabilityUnknown = "capability_unknown"
	capacityReasonPrompting         = "prompting"
	capacityReasonStarting          = "starting"
)

func classifyRunCapacity(
	work *RunSnapshot,
	sessions []SessionSnapshot,
	occupied map[string]struct{},
) CapacityDisposition {
	if work == nil {
		return CapacityDisposition{Kind: CapacityUnmatched}
	}

	available := make([]SessionSnapshot, 0, len(sessions))
	waitingReason := ""
	indeterminate := false
	for _, candidate := range sessions {
		if !sessionStructurallyMatches(work, candidate) {
			continue
		}
		if len(work.Run.RequiredCapabilities) > 0 &&
			normalizeCapabilityState(candidate.CapabilityState) == CapabilityStateUnknown {
			indeterminate = true
			continue
		}
		if !capabilitiesCover(candidate.Capabilities, work.Run.RequiredCapabilities) {
			continue
		}

		sessionID := strings.TrimSpace(candidate.ID)
		switch {
		case strings.TrimSpace(candidate.State) == schedulerStartingKey:
			waitingReason = firstCapacityReason(waitingReason, capacityReasonStarting)
		case candidate.Prompting:
			waitingReason = firstCapacityReason(waitingReason, capacityReasonPrompting)
		case isOccupied(occupied, sessionID):
			waitingReason = firstCapacityReason(waitingReason, capacityReasonOccupied)
		default:
			available = append(available, candidate)
		}
	}

	if len(available) > 0 {
		return CapacityDisposition{Kind: CapacityAvailable, Available: available}
	}
	if waitingReason != "" {
		return CapacityDisposition{Kind: CapacityWaiting, Reason: waitingReason}
	}
	if indeterminate {
		return CapacityDisposition{
			Kind:   CapacityIndeterminate,
			Reason: capacityReasonCapabilityUnknown,
		}
	}
	return CapacityDisposition{Kind: CapacityUnmatched}
}

func sessionStructurallyMatches(work *RunSnapshot, candidate SessionSnapshot) bool {
	if work == nil || strings.TrimSpace(candidate.ID) == "" {
		return false
	}
	switch strings.TrimSpace(candidate.State) {
	case schedulerActiveKey, schedulerStartingKey:
	default:
		return false
	}
	return scopeMatches(work.Task, candidate) &&
		ownerMatches(work.Task, candidate)
}

func ownerMatches(task taskpkg.Task, candidate SessionSnapshot) bool {
	if task.Owner == nil || task.Owner.IsZero() {
		return true
	}
	owner := *task.Owner
	kind := owner.Kind.Normalize()
	ref := strings.TrimSpace(owner.Ref)
	if ref == "" {
		return false
	}
	switch kind {
	case taskpkg.OwnerKindPool:
		return strings.TrimSpace(candidate.AgentName) == ref
	case taskpkg.OwnerKindAgentSession:
		return strings.TrimSpace(candidate.ID) == ref
	default:
		return false
	}
}

func scopeMatches(task taskpkg.Task, candidate SessionSnapshot) bool {
	workspaceID := strings.TrimSpace(task.WorkspaceID)
	sessionWorkspaceID := strings.TrimSpace(candidate.WorkspaceID)
	switch task.Scope.Normalize() {
	case taskpkg.ScopeWorkspace:
		return workspaceID != "" && workspaceID == sessionWorkspaceID
	case taskpkg.ScopeGlobal:
		return sessionWorkspaceID == ""
	default:
		return false
	}
}

func capabilitiesCover(available []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	caps := make(map[string]struct{}, len(available))
	for _, value := range available {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			caps[trimmed] = struct{}{}
		}
	}
	for _, value := range required {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := caps[trimmed]; !ok {
			return false
		}
	}
	return true
}

func normalizeCapabilityState(state CapabilityState) CapabilityState {
	if state == CapabilityStateUnknown {
		return CapabilityStateUnknown
	}
	return CapabilityStateKnown
}

func isOccupied(occupied map[string]struct{}, sessionID string) bool {
	if strings.TrimSpace(sessionID) == "" {
		return false
	}
	_, ok := occupied[sessionID]
	return ok
}

func firstCapacityReason(current string, candidate string) string {
	if current != "" {
		return current
	}
	return candidate
}
