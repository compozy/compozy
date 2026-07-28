package hooks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const (
	permissionBlockedKey = "blocked"
	permissionRejectKey  = "reject"
)

var (
	ErrHookPatchRejected           = errors.New("hooks: hook patch rejected")
	ErrPermissionEscalationBlocked = errors.New("hooks: permission escalation blocked")
)

const permissionDecisionDeny = "deny"

func newPermissionRequestGuard(
	logger *slog.Logger,
	metrics *hookMetrics,
) patchGuard[PermissionRequestPayload, PermissionRequestPatch] {
	if logger == nil {
		logger = slog.Default()
	}

	return func(
		ctx context.Context,
		hook RegisteredHook,
		payload PermissionRequestPayload,
		patch PermissionRequestPatch,
	) error {
		beforeDecision := normalizedPermissionDecision(payload.Decision)
		afterDecision := normalizedPermissionDecision(permissionDecisionAfterPatch(payload.Decision, patch))
		if permissionDecisionDenied(beforeDecision) && !permissionDecisionDenied(afterDecision) {
			metrics.observePermissionEscalationBlock()
			logger.WarnContext(
				ctx,
				"hook.dispatch.permission_escalation_blocked",
				"hook", hook.Name,
				"event", hook.Event.String(),
				"source", hook.Source.String(),
				"decision_before", beforeDecision,
				"decision_after", afterDecision,
			)

			return fmt.Errorf("%w: %w", ErrHookPatchRejected, ErrPermissionEscalationBlocked)
		}

		return nil
	}
}

func permissionDecisionAfterPatch(decision string, patch PermissionRequestPatch) string {
	switch {
	case patch.Deny:
		return permissionDecisionDeny
	case patch.Decision != nil:
		return *patch.Decision
	default:
		return decision
	}
}

func permissionPatchDenies(patch PermissionRequestPatch) bool {
	switch {
	case patch.Deny:
		return true
	case patch.Decision == nil:
		return false
	default:
		return permissionDecisionDenied(*patch.Decision)
	}
}

func permissionDecisionDenied(decision string) bool {
	clean := normalizedPermissionDecision(decision)
	switch {
	case clean == "":
		return false
	case clean == "block", clean == permissionBlockedKey:
		return true
	case clean == permissionDecisionDeny, clean == "denied", clean == "rejected":
		return true
	case strings.HasPrefix(clean, "block-"):
		return true
	case strings.HasPrefix(clean, permissionDecisionDeny+"-"):
		return true
	case clean == permissionRejectKey:
		return true
	case strings.HasPrefix(clean, "reject-"):
		return true
	default:
		return false
	}
}

func normalizedPermissionDecision(decision string) string {
	return strings.ToLower(strings.TrimSpace(decision))
}
