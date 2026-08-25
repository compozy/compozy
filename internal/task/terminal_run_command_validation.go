package task

import (
	"bytes"
	"fmt"
	"strings"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

func validateTerminalRunCommandCredentialFields(command TerminalRunCommand) error {
	for _, field := range []struct {
		path  string
		value string
	}{
		{path: "command_id", value: command.commandID},
		{path: "run_id", value: command.fence.RunID()},
		{path: "task_id", value: command.taskID},
		{path: "workspace_id", value: command.workspaceID},
		{path: "session_id", value: command.fence.SessionID()},
		{path: "claim_hash", value: command.fence.ClaimTokenHash()},
		{path: "actor_ref", value: command.actor.Actor.Ref},
		{path: "origin_ref", value: command.actor.Origin.Ref},
		{path: "scope_session_id", value: command.actor.Scope.SessionID},
		{path: "scope_workspace_id", value: command.actor.Scope.WorkspaceID},
	} {
		if redactpkg.ContainsRawClaimToken(field.value) {
			return fmt.Errorf(
				"%w: terminal run command %s contains raw lease credentials",
				ErrValidation,
				field.path,
			)
		}
	}
	return nil
}

func validateTerminalRunCommandPhase(phase TerminalRunCommandPhase) error {
	switch phase {
	case TerminalRunCommandPhaseAdmitted,
		TerminalRunCommandPhaseStopRequested,
		TerminalRunCommandPhaseStopConfirmed:
		return nil
	default:
		return fmt.Errorf("%w: invalid terminal run command phase %q", ErrValidation, phase)
	}
}

func validateTerminalRunCommandIntent(kind TerminalRunCommandKind, intent terminalRunCommandIntent) error {
	if intent.Version != terminalRunCommandIntentVersion {
		return fmt.Errorf("%w: unsupported terminal run command intent version %d", ErrValidation, intent.Version)
	}
	if err := validateTerminalRunCommandIntentShape(kind, intent); err != nil {
		return err
	}
	if err := validateTerminalRunCommandEventIDs(kind, intent); err != nil {
		return err
	}
	switch kind {
	case TerminalRunCommandCompleted:
		return validateCompletionTerminalRunCommandIntent(intent)
	case TerminalRunCommandFailed:
		return validateFailureTerminalRunCommandIntent(intent)
	case TerminalRunCommandCanceled:
		return validateCancellationTerminalRunCommandIntent(intent)
	case TerminalRunCommandNeedsAttention:
		return validateNeedsAttentionTerminalRunCommandIntent(intent)
	default:
		return fmt.Errorf("%w: invalid terminal run command kind %q", ErrValidation, kind)
	}
}

func validateTerminalRunCommandEventIDs(kind TerminalRunCommandKind, intent terminalRunCommandIntent) error {
	want := 1
	if kind == TerminalRunCommandFailed && intent.RecoveryAudit != nil {
		want++
	}
	if kind == TerminalRunCommandCanceled && intent.StopRequired {
		want++
	}
	if len(intent.EventIDs) != want {
		return fmt.Errorf(
			"%w: terminal run command %q requires %d event ids, got %d",
			ErrValidation,
			kind,
			want,
			len(intent.EventIDs),
		)
	}
	seen := make(map[string]struct{}, len(intent.EventIDs))
	for index, eventID := range intent.EventIDs {
		trimmed := strings.TrimSpace(eventID)
		if trimmed == "" {
			return fmt.Errorf("%w: terminal run command event_ids[%d] is required", ErrValidation, index)
		}
		if err := ValidateReferenceSize(trimmed, fmt.Sprintf("terminal_run_command.event_ids[%d]", index)); err != nil {
			return err
		}
		if redactpkg.ContainsRawClaimToken(trimmed) {
			return fmt.Errorf(
				"%w: terminal run command event_ids[%d] contains raw lease credentials",
				ErrValidation,
				index,
			)
		}
		if _, exists := seen[trimmed]; exists {
			return fmt.Errorf("%w: terminal run command event id %q is duplicated", ErrValidation, trimmed)
		}
		seen[trimmed] = struct{}{}
	}
	return nil
}

func validateTerminalRunCommandIntentAgainstFence(command TerminalRunCommand) error {
	status := command.fence.Status()
	hasSession := command.fence.SessionID() != ""
	expectedStop := false

	switch command.kind {
	case TerminalRunCommandCompleted:
		if status != TaskRunStatusRunning {
			return invalidTerminalRunCommandSourceStatus(command.kind, status)
		}
		expectedStop = hasSession
	case TerminalRunCommandFailed:
		if status != TaskRunStatusStarting && status != TaskRunStatusRunning {
			return invalidTerminalRunCommandSourceStatus(command.kind, status)
		}
		expectedStop = hasSession
		if audit := command.intent.RecoveryAudit; audit != nil {
			if audit.Recovery.Action != RunBootRecoveryFail ||
				audit.PreviousStatus != status ||
				audit.PreviousSessionID != command.fence.SessionID() {
				return fmt.Errorf(
					"%w: terminal failure recovery audit does not match its source fence",
					ErrValidation,
				)
			}
			if audit.Recovery.StopRequired && !hasSession {
				return fmt.Errorf(
					"%w: terminal failure recovery requires a fenced session to stop",
					ErrValidation,
				)
			}
			expectedStop = audit.Recovery.StopRequired && hasSession
		}
	case TerminalRunCommandCanceled:
		switch status {
		case TaskRunStatusQueued, TaskRunStatusClaimed:
		case TaskRunStatusStarting, TaskRunStatusRunning:
			expectedStop = hasSession
		default:
			return invalidTerminalRunCommandSourceStatus(command.kind, status)
		}
	case TerminalRunCommandNeedsAttention:
		switch status {
		case TaskRunStatusQueued,
			TaskRunStatusClaimed,
			TaskRunStatusStarting,
			TaskRunStatusRunning:
		default:
			return invalidTerminalRunCommandSourceStatus(command.kind, status)
		}
	default:
		return fmt.Errorf("%w: invalid terminal run command kind %q", ErrValidation, command.kind)
	}

	if command.intent.StopRequired != expectedStop {
		return fmt.Errorf(
			"%w: terminal run command stop requirement does not match its source fence",
			ErrValidation,
		)
	}
	return nil
}

func invalidTerminalRunCommandSourceStatus(kind TerminalRunCommandKind, status RunStatus) error {
	return fmt.Errorf(
		"%w: terminal run command %q cannot use source status %q",
		ErrValidation,
		kind,
		status,
	)
}

func validateTerminalRunCommandIntentShape(
	kind TerminalRunCommandKind,
	intent terminalRunCommandIntent,
) error {
	unexpected := false
	switch kind {
	case TerminalRunCommandCompleted:
		unexpected = intent.Failure != nil || intent.Cancellation != nil ||
			strings.TrimSpace(intent.Diagnostic) != "" || intent.RecoveryAudit != nil
	case TerminalRunCommandFailed:
		unexpected = len(intent.Result) != 0 || intent.Cancellation != nil ||
			strings.TrimSpace(intent.Diagnostic) != ""
	case TerminalRunCommandCanceled:
		unexpected = len(intent.Result) != 0 || intent.Failure != nil ||
			strings.TrimSpace(intent.Diagnostic) != "" || intent.RecoveryAudit != nil
	case TerminalRunCommandNeedsAttention:
		unexpected = len(intent.Result) != 0 || intent.Failure != nil ||
			intent.Cancellation != nil || intent.RecoveryAudit != nil
	}
	if unexpected {
		return fmt.Errorf("%w: terminal run command contains fields for another intent kind", ErrValidation)
	}
	return nil
}

func validateCompletionTerminalRunCommandIntent(intent terminalRunCommandIntent) error {
	if intent.StopReason != StopReasonCompleted {
		return fmt.Errorf("%w: completion command requires completed stop reason", ErrValidation)
	}
	if hasRawClaimTokenJSON(intent.Result) {
		return fmt.Errorf("%w: terminal run command result contains raw lease credentials", ErrValidation)
	}
	return nil
}

func validateFailureTerminalRunCommandIntent(intent terminalRunCommandIntent) error {
	if intent.StopReason != StopReasonFailed {
		return fmt.Errorf("%w: failure command requires failed stop reason", ErrValidation)
	}
	if intent.Failure == nil {
		return fmt.Errorf("%w: terminal failure intent is required", ErrValidation)
	}
	if err := intent.Failure.Validate("terminal_run_command.failure"); err != nil {
		return err
	}
	if redactpkg.ContainsRawClaimToken(intent.Failure.Error) ||
		hasRawClaimTokenJSON(intent.Failure.Metadata) {
		return fmt.Errorf("%w: terminal failure contains raw lease credentials", ErrValidation)
	}
	if intent.RecoveryAudit != nil {
		return validateTerminalRunRecoveryIntent(*intent.RecoveryAudit)
	}
	return nil
}

func validateTerminalRunRecoveryIntent(intent terminalRunRecoveryIntent) error {
	if err := intent.Recovery.Validate("terminal_run_command.recovery_audit"); err != nil {
		return err
	}
	if err := intent.PreviousStatus.Validate("terminal_run_command.recovery_audit.previous_status"); err != nil {
		return err
	}
	if err := ValidateReferenceSize(
		intent.PreviousSessionID,
		"terminal_run_command.recovery_audit.previous_session_id",
	); err != nil {
		return err
	}
	if redactpkg.ContainsRawClaimToken(intent.Recovery.Reason) ||
		redactpkg.ContainsRawClaimToken(intent.Recovery.SessionState) ||
		redactpkg.ContainsRawClaimToken(intent.Recovery.Classification) ||
		redactpkg.ContainsRawClaimToken(intent.Recovery.Detail) {
		return fmt.Errorf("%w: terminal recovery audit contains raw lease credentials", ErrValidation)
	}
	return nil
}

func validateCancellationTerminalRunCommandIntent(intent terminalRunCommandIntent) error {
	if intent.StopReason != StopReasonCancellation {
		return fmt.Errorf("%w: cancellation command requires cancellation stop reason", ErrValidation)
	}
	if intent.Cancellation == nil {
		return fmt.Errorf("%w: terminal cancellation intent is required", ErrValidation)
	}
	if err := intent.Cancellation.Request.Validate("terminal_run_command.cancellation"); err != nil {
		return err
	}
	if err := ValidateReferenceSize(
		intent.Cancellation.PropagatedFromTaskID,
		"terminal_run_command.cancellation.propagated_from_task_id",
	); err != nil {
		return err
	}
	if redactpkg.ContainsRawClaimToken(intent.Cancellation.Request.Reason) ||
		hasRawClaimTokenJSON(intent.Cancellation.Request.Metadata) {
		return fmt.Errorf("%w: terminal cancellation contains raw lease credentials", ErrValidation)
	}
	return nil
}

func hasRawClaimTokenJSON(raw []byte) bool {
	return len(raw) > 0 && !bytes.Equal(raw, redactpkg.ClaimTokensJSON(raw))
}

func validateNeedsAttentionTerminalRunCommandIntent(intent terminalRunCommandIntent) error {
	if intent.StopRequired || intent.StopReason != "" {
		return fmt.Errorf("%w: needs-attention command cannot stop a session", ErrValidation)
	}
	if err := ValidateDiagnosticSize(intent.Diagnostic, "terminal_run_command.diagnostic"); err != nil {
		return err
	}
	if redactpkg.ContainsRawClaimToken(intent.Diagnostic) {
		return fmt.Errorf("%w: terminal diagnostic contains a raw claim token", ErrValidation)
	}
	return nil
}
