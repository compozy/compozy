package contract

// TerminalErrorCode is the closed public error vocabulary for terminal transports.
type TerminalErrorCode string

const (
	TerminalErrorNotFound                 TerminalErrorCode = "terminal_not_found"
	TerminalErrorProfileSelectionConflict TerminalErrorCode = "profile_selection_conflict"
	TerminalErrorProfileSessionConflict   TerminalErrorCode = "profile_session_conflict"
	TerminalErrorRequiresWorkspace        TerminalErrorCode = "terminal_requires_workspace"
	TerminalErrorProfileArchived          TerminalErrorCode = "profile_archived"
	TerminalErrorProfileUnavailable       TerminalErrorCode = "profile_unavailable"
	TerminalErrorLimitReached             TerminalErrorCode = "terminal_limit_reached"
	TerminalErrorSubscriberLimitReached   TerminalErrorCode = "subscriber_limit_reached"
	TerminalErrorExited                   TerminalErrorCode = "terminal_exited"
	TerminalErrorExpired                  TerminalErrorCode = "terminal_expired"
	TerminalErrorInteractiveUnavailable   TerminalErrorCode = "terminal_interactive_unavailable"
	TerminalErrorNotInteractive           TerminalErrorCode = "terminal_not_interactive"
	TerminalErrorInvalidCwd               TerminalErrorCode = "invalid_cwd"
	TerminalErrorTimeoutOutOfRange        TerminalErrorCode = "timeout_out_of_range"
	TerminalErrorWriteOwnerHeld           TerminalErrorCode = "write_owner_held"
	TerminalErrorLeaseRevoked             TerminalErrorCode = "lease_revoked"
	TerminalErrorGenerationFenced         TerminalErrorCode = "generation_fenced"
	TerminalErrorTypingGrantRejected      TerminalErrorCode = "typing_grant_rejected"
	TerminalErrorApprovalRejected         TerminalErrorCode = "approval_rejected"
	TerminalErrorTicketInvalid            TerminalErrorCode = "ticket_invalid"
	TerminalErrorTicketExpired            TerminalErrorCode = "ticket_expired"
	TerminalErrorInputRequestNotFound     TerminalErrorCode = "input_request_not_found"
	TerminalErrorInputRequestAnswered     TerminalErrorCode = "input_request_already_answered"
	TerminalErrorInputRequestSuperseded   TerminalErrorCode = "input_request_superseded"
	TerminalErrorInputRequestLimitReached TerminalErrorCode = "input_request_limit_reached"
	TerminalErrorInputAnswerRequiresWrite TerminalErrorCode = "input_answer_requires_write"
	TerminalErrorRecordingAlreadyStarted  TerminalErrorCode = "recording_already_started"
	TerminalErrorRecordingNotActive       TerminalErrorCode = "recording_not_active"
	TerminalErrorRecordingUnavailable     TerminalErrorCode = "recording_unavailable"
	TerminalErrorSlowConsumer             TerminalErrorCode = "slow_consumer"
	TerminalErrorJournalUnavailable       TerminalErrorCode = "journal_unavailable"
)

var terminalErrorCodeValues = [...]TerminalErrorCode{
	TerminalErrorNotFound,
	TerminalErrorProfileSelectionConflict,
	TerminalErrorProfileSessionConflict,
	TerminalErrorRequiresWorkspace,
	TerminalErrorProfileArchived,
	TerminalErrorProfileUnavailable,
	TerminalErrorLimitReached,
	TerminalErrorSubscriberLimitReached,
	TerminalErrorExited,
	TerminalErrorExpired,
	TerminalErrorInteractiveUnavailable,
	TerminalErrorNotInteractive,
	TerminalErrorInvalidCwd,
	TerminalErrorTimeoutOutOfRange,
	TerminalErrorWriteOwnerHeld,
	TerminalErrorLeaseRevoked,
	TerminalErrorGenerationFenced,
	TerminalErrorTypingGrantRejected,
	TerminalErrorApprovalRejected,
	TerminalErrorTicketInvalid,
	TerminalErrorTicketExpired,
	TerminalErrorInputRequestNotFound,
	TerminalErrorInputRequestAnswered,
	TerminalErrorInputRequestSuperseded,
	TerminalErrorInputRequestLimitReached,
	TerminalErrorInputAnswerRequiresWrite,
	TerminalErrorRecordingAlreadyStarted,
	TerminalErrorRecordingNotActive,
	TerminalErrorRecordingUnavailable,
	TerminalErrorSlowConsumer,
	TerminalErrorJournalUnavailable,
}

// TerminalErrorCodeValues returns the frozen terminal error vocabulary in contract order.
func TerminalErrorCodeValues() []string {
	values := make([]string, len(terminalErrorCodeValues))
	for index, code := range terminalErrorCodeValues {
		values[index] = string(code)
	}
	return values
}

// IsTerminalErrorCode reports whether code belongs to the frozen public vocabulary.
func IsTerminalErrorCode(code TerminalErrorCode) bool {
	for _, candidate := range terminalErrorCodeValues {
		if code == candidate {
			return true
		}
	}
	return false
}

// TerminalErrorDetail is the canonical terminal failure carried by HTTP, UDS,
// CLI structured output, and terminal WebSocket ERROR frames.
type TerminalErrorDetail struct {
	Code    TerminalErrorCode `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// TerminalErrorResponse wraps every public terminal failure in one envelope.
type TerminalErrorResponse struct {
	Error TerminalErrorDetail `json:"error"`
}
