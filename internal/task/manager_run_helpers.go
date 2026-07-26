package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func nextRunAttempt(runs []Run) int {
	maxAttempt := 0
	for _, run := range runs {
		if int(run.Attempt) > maxAttempt {
			maxAttempt = int(run.Attempt)
		}
	}
	return maxAttempt + 1
}

func errorsJoin(errs ...error) error {
	return errors.Join(errs...)
}

func runBootRecoveryError(run Run, recovery RunBootRecovery) string {
	sessionID := strings.TrimSpace(run.SessionID)
	switch {
	case sessionID != "" && recovery.Classification != "" && recovery.Detail != "":
		return fmt.Sprintf(
			"orphaned on boot: session %q classified as %s (%s)",
			sessionID,
			recovery.Classification,
			recovery.Detail,
		)
	case sessionID != "" && recovery.Classification != "":
		return fmt.Sprintf(
			"orphaned on boot: session %q classified as %s",
			sessionID,
			recovery.Classification,
		)
	case sessionID != "" && recovery.SessionState != "":
		return fmt.Sprintf("orphaned on boot: session %q is %s", sessionID, recovery.SessionState)
	case sessionID != "":
		return fmt.Sprintf("orphaned on boot: session %q is not live", sessionID)
	default:
		return "orphaned on boot: run has no live session"
	}
}

func runBootRecoveryMetadata(run Run, recovery RunBootRecovery) json.RawMessage {
	payload, err := marshalTaskEventPayload(map[string]string{
		runBootRecoveryReasonKey: normalizedBootRecoveryReason(recovery.Reason),
		"previous_status":        run.Status.Normalize().String(),
		"session_id":             strings.TrimSpace(run.SessionID),
		"session_state":          strings.TrimSpace(recovery.SessionState),
		"classification":         strings.TrimSpace(recovery.Classification),
		"detail":                 strings.TrimSpace(recovery.Detail),
	})
	if err != nil {
		return nil
	}
	return payload
}

func normalizedBootRecoveryReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return managerOrphanedOnBootKey
	}
	return trimmed
}
